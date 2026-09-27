package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TestMetricsEndpoint verifies the Prometheus telemetry endpoint requires admin auth.
func TestMetricsEndpoint(t *testing.T) {
	pool, cfg, router := getTestEnv(t)
	defer pool.Close()

	// Unauthenticated request must be rejected
	wUnauthed := httptest.NewRecorder()
	reqUnauthed, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(wUnauthed, reqUnauthed)
	if wUnauthed.Code != http.StatusUnauthorized && wUnauthed.Code != http.StatusForbidden {
		t.Errorf("Expected /metrics to reject unauthenticated request, got %d", wUnauthed.Code)
	}

	// Authenticated request with admin token must succeed
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected /metrics status 200 with admin token, got %d. Body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, expected := range []string{"go_goroutines", "go_memstats_alloc_bytes", "bayesmarket_db_connections_acquired", "bayesmarket_up 1"} {
		if !bytes.Contains([]byte(body), []byte(expected)) {
			t.Errorf("Expected /metrics output to contain %q, but was missing", expected)
		}
	}
}

// TestAdminResolveUnauthorized verifies rejection of unauthorized calls.
func TestAdminResolveUnauthorized(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	randomMarketID := uuid.New().String()
	url := fmt.Sprintf("/api/v1/admin/markets/%s/resolve", randomMarketID)

	// 1. Without Authorization header -> 401
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{"winning_outcome":"YES","oracle_proof":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.New().String())
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized without auth header, got %d", w.Code)
	}

	// 2. With invalid bearer token -> 403
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{"winning_outcome":"YES","oracle_proof":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-admin-token-xyz")
	req.Header.Set("Idempotency-Key", uuid.New().String())
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden with invalid token, got %d", w.Code)
	}

	// 3. With regular user bearer token -> 403 Forbidden
	wGuest := httptest.NewRecorder()
	reqGuest, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(wGuest, reqGuest)
	var authResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(wGuest.Body.Bytes(), &authResp)
	if authResp.Token != "" {
		wUser := httptest.NewRecorder()
		reqUser, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{"winning_outcome":"YES","oracle_proof":"test"}`))
		reqUser.Header.Set("Content-Type", "application/json")
		reqUser.Header.Set("Authorization", "Bearer "+authResp.Token)
		reqUser.Header.Set("Idempotency-Key", uuid.New().String())
		router.ServeHTTP(wUser, reqUser)

		if wUser.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden with user JWT on admin endpoint, got %d", wUser.Code)
		}
	}
}

// TestAdminResolveMissingIdempotency verifies requirement of Idempotency-Key.
func TestAdminResolveMissingIdempotency(t *testing.T) {
	pool, cfg, router := getTestEnv(t)
	defer pool.Close()

	randomMarketID := uuid.New().String()
	url := fmt.Sprintf("/api/v1/admin/markets/%s/resolve", randomMarketID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{"winning_outcome":"YES","oracle_proof":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request when missing Idempotency-Key, got %d", w.Code)
	}
}

// TestAdminResolveLiveWorkflow verifies atomic market resolution, winner payouts, and idempotency replay.
func TestAdminResolveLiveWorkflow(t *testing.T) {
	pool, cfg, router := getTestEnv(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. Create a dedicated test market
	testMarketID := uuid.New()
	testSlug := fmt.Sprintf("test-resolve-market-%d", time.Now().UnixNano())
	insertMarket := `
		INSERT INTO markets (id, slug, title, description, category, resolution_source, resolution_date, status)
		VALUES ($1, $2, 'Test Resolution Market', 'Resolves YES or NO for settlement test', 'macro', 'Oracle Official', NOW() + INTERVAL '1 day', 'active');
	`
	if _, err := pool.Exec(ctx, insertMarket, testMarketID, testSlug); err != nil {
		t.Fatalf("Failed to create test market: %v", err)
	}

	// 2. Initialize liquidity pool with 20,000 USDC collateral
	insertPool := `
		INSERT INTO liquidity_pools (market_id, reserve_yes, reserve_no, collateral_reserve, k_invariant, total_volume_usdc)
		VALUES ($1, 10000.00000000, 10000.00000000, 20000.00000000, 100000000.00000000, 0.00000000);
	`
	if _, err := pool.Exec(ctx, insertPool, testMarketID); err != nil {
		t.Fatalf("Failed to create test liquidity pool: %v", err)
	}

	// 3. Create two test users (one holding winning YES shares, one holding losing NO shares)
	userWinnerID := uuid.New()
	userLoserID := uuid.New()

	for _, uid := range []uuid.UUID{userWinnerID, userLoserID} {
		insertUser := `
			INSERT INTO users (id, is_guest, cash_balance)
			VALUES ($1, true, 100.00000000);
		`
		if _, err := pool.Exec(ctx, insertUser, uid); err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Insert positions: Winner holds 50 YES shares, Loser holds 50 NO shares
	insertWinnerPos := `
		INSERT INTO user_positions (user_id, market_id, outcome, shares_owned, avg_buy_price, total_invested_usdc)
		VALUES ($1, $2, 'YES', 50.00000000, 0.50000000, 25.00000000);
	`
	if _, err := pool.Exec(ctx, insertWinnerPos, userWinnerID, testMarketID); err != nil {
		t.Fatalf("Failed to insert winner position: %v", err)
	}

	insertLoserPos := `
		INSERT INTO user_positions (user_id, market_id, outcome, shares_owned, avg_buy_price, total_invested_usdc)
		VALUES ($1, $2, 'NO', 50.00000000, 0.50000000, 25.00000000);
	`
	if _, err := pool.Exec(ctx, insertLoserPos, userLoserID, testMarketID); err != nil {
		t.Fatalf("Failed to insert loser position: %v", err)
	}

	// 4. Resolve market to YES via admin API
	idempKey := fmt.Sprintf("resolve-%d", time.Now().UnixNano())
	resolvePayload := `{"winning_outcome":"YES","oracle_proof":"Official BLS Labor Release #2026-09"}`
	url := fmt.Sprintf("/api/v1/admin/markets/%s/resolve", testMarketID.String())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(resolvePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	req.Header.Set("Idempotency-Key", idempKey)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected resolution status 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status          string `json:"status"`
		WinningOutcome  string `json:"winning_outcome"`
		TotalPayoutUSDC string `json:"total_payout_usdc"`
		WinnersCredited int    `json:"winners_credited"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode resolve response: %v", err)
	}

	if resp.Status != "resolved" || resp.WinningOutcome != "YES" || resp.WinnersCredited != 1 {
		t.Errorf("Unexpected resolve response: %+v", resp)
	}

	// 5. Verify database state after resolution:
	// Winner balance should be 100 (initial) + 50 (payout) = 150 USDC
	var winnerBal decimal.Decimal
	_ = pool.QueryRow(ctx, "SELECT cash_balance FROM users WHERE id = $1", userWinnerID).Scan(&winnerBal)
	expectedWinnerBal := decimal.NewFromInt(150)
	if !winnerBal.Equal(expectedWinnerBal) {
		t.Errorf("Expected winner cash balance %s, got %s", expectedWinnerBal, winnerBal)
	}

	// Winner and Loser positions should now have 0 shares owned
	var winnerShares, loserShares decimal.Decimal
	_ = pool.QueryRow(ctx, "SELECT shares_owned FROM user_positions WHERE user_id = $1 AND market_id = $2", userWinnerID, testMarketID).Scan(&winnerShares)
	_ = pool.QueryRow(ctx, "SELECT shares_owned FROM user_positions WHERE user_id = $1 AND market_id = $2", userLoserID, testMarketID).Scan(&loserShares)

	if !winnerShares.IsZero() {
		t.Errorf("Expected winner shares to be 0 after settlement, got %s", winnerShares)
	}
	if !loserShares.IsZero() {
		t.Errorf("Expected loser shares to be 0 after settlement, got %s", loserShares)
	}

	// 6. Test Idempotency Replay: calling again with same key must return cached 200 OK
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(resolvePayload))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	req2.Header.Set("Idempotency-Key", idempKey)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on idempotent replay, got %d", w2.Code)
	}
	if w2.Header().Get("X-Cache-Lookup") != "HIT-IDEMPOTENT" {
		t.Errorf("Expected HIT-IDEMPOTENT header on replay")
	}

	// Clean up test data
	_, _ = pool.Exec(ctx, "DELETE FROM markets WHERE id = $1", testMarketID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userWinnerID, userLoserID)
}
