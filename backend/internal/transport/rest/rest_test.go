package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/database"
	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/bayesmarket/bayesmarket/internal/testutil"
	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func getSuperadminToken(t *testing.T, pool *pgxpool.Pool, cfg *config.Config) string {
	adminID := uuid.New()
	if pool != nil {
		_, err := pool.Exec(context.Background(), `
			INSERT INTO users (id, is_guest, is_admin, auth_provider, name, email)
			VALUES ($1, false, true, 'google', 'Test Admin', 'admin@test.local')
			ON CONFLICT (id) DO UPDATE SET is_admin = true;
		`, adminID)
		if err != nil {
			t.Fatalf("failed to insert test admin user: %v", err)
		}
	}
	claims := middleware.AuthClaims{
		UserID:       adminID.String(),
		IsGuest:      false,
		IsAdmin:      true,
		IsSuperadmin: true,
		Email:        "admin@test.local",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("failed to sign admin token: %v", err)
	}
	return signed
}

func getTestEnv(t *testing.T) (*pgxpool.Pool, *config.Config, *gin.Engine) {
	dbURL := testutil.SafeTestDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping live REST tests: cannot connect to test database: %v", err)
	}

	cfg := testutil.TestConfig(dbURL)
	router := rest.SetupRouter(pool, cfg)
	return pool, cfg, router
}

// 1. Health check endpoint test
func TestHealthzEndpoint(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", resp["status"])
	}
	if resp["database"] != "connected" {
		t.Errorf("Expected database 'connected', got %v", resp["database"])
	}
}

// 2. Task 4.1: Guest Authentication & Session Issuance
func TestGuestAuthEndpoint(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var authResp rest.GuestAuthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if authResp.Token == "" {
		t.Error("Expected non-empty JWT token string")
	}
	if !authResp.User.IsGuest {
		t.Error("Expected is_guest to be true")
	}
	if authResp.User.CashBalance != "1000.00000000" {
		t.Errorf("Expected cash_balance '1000.00000000', got '%s'", authResp.User.CashBalance)
	}
	if authResp.User.ID == "" {
		t.Error("Expected valid user UUID")
	}
}

func ensureTestMarket(t *testing.T, pool *pgxpool.Pool, slug, title string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id string
	err := pool.QueryRow(ctx, "SELECT id::text FROM markets WHERE slug = $1", slug).Scan(&id)
	if err == nil {
		return id
	}

	resDate := time.Now().Add(365 * 24 * time.Hour).UTC()
	err = pool.QueryRow(ctx, `
		INSERT INTO markets (slug, title, description, category, resolution_source, resolution_date, status)
		VALUES ($1, $2, 'Test description', 'crypto', 'https://example.com', $3, 'active')
		RETURNING id::text;
	`, slug, title, resDate).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test market: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO liquidity_pools (market_id, reserve_yes, reserve_no, collateral_reserve, k_invariant, total_volume_usdc, lock_version)
		VALUES ($1::uuid, 10000, 10000, 20000, 100000000, 0, 0)
		ON CONFLICT (market_id) DO NOTHING;
	`, id)
	if err != nil {
		t.Fatalf("Failed to insert test liquidity pool: %v", err)
	}
	return id
}

// 3. Task 4.3: Market Discovery Endpoints
func TestMarketDiscoveryEndpoints(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	// Ensure test markets exist for discovery
	ensureTestMarket(t, pool, "test-market-discovery-1", "Test Discovery Market 1")
	ensureTestMarket(t, pool, "test-market-discovery-2", "Test Discovery Market 2")
	ensureTestMarket(t, pool, "test-market-discovery-3", "Test Discovery Market 3")
	ensureTestMarket(t, pool, "test-market-discovery-4", "Test Discovery Market 4")

	// GET /api/v1/markets
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/markets", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var markets []rest.MarketSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &markets); err != nil {
		t.Fatalf("Failed to unmarshal markets: %v", err)
	}

	if len(markets) < 4 {
		t.Fatalf("Expected at least 4 active markets, found %d", len(markets))
	}

	// Verify first market details
	m := markets[0]
	if m.ID == "" || m.Slug == "" || m.Title == "" {
		t.Errorf("Incomplete market metadata: %+v", m)
	}
	if m.ProbabilityYes == "" || m.ProbabilityNo == "" {
		t.Errorf("Missing implied probabilities: %+v", m)
	}

	// GET /api/v1/markets with pagination limit=2
	wPaged := httptest.NewRecorder()
	reqPaged, _ := http.NewRequest(http.MethodGet, "/api/v1/markets?limit=2&offset=0", nil)
	router.ServeHTTP(wPaged, reqPaged)
	if wPaged.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for paged markets, got %d", wPaged.Code)
	}
	var pagedMarkets []rest.MarketSummaryResponse
	if err := json.Unmarshal(wPaged.Body.Bytes(), &pagedMarkets); err != nil {
		t.Fatalf("Failed to unmarshal paged markets: %v", err)
	}
	if len(pagedMarkets) != 2 {
		t.Errorf("Expected exactly 2 markets with limit=2, got %d", len(pagedMarkets))
	}

	// GET /api/v1/markets/:id by Slug
	wSingle := httptest.NewRecorder()
	reqSingle, _ := http.NewRequest(http.MethodGet, "/api/v1/markets/"+m.Slug, nil)
	router.ServeHTTP(wSingle, reqSingle)

	if wSingle.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for single market by slug, got %d", wSingle.Code)
	}

	var singleMarket rest.MarketSummaryResponse
	if err := json.Unmarshal(wSingle.Body.Bytes(), &singleMarket); err != nil {
		t.Fatalf("Failed to unmarshal single market: %v", err)
	}
	if singleMarket.ID != m.ID {
		t.Errorf("Market ID mismatch: got %s, expected %s", singleMarket.ID, m.ID)
	}
}

// 4. Task 4.3: Authoritative CPMM Quote Endpoint
func TestMarketQuoteEndpoint(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	ensureTestMarket(t, pool, "will-bitcoin-hit-125k-in-2026", "Will Bitcoin hit $125k in 2026?")

	// 1. Quote a BUY of 100 USDC on YES for Bitcoin market
	payloadBuy := map[string]string{
		"action":      "BUY",
		"outcome":     "YES",
		"amount_usdc": "100.00000000",
	}
	bodyBytes, _ := json.Marshal(payloadBuy)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/markets/will-bitcoin-hit-125k-in-2026/quote", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for buy quote, got %d. Body: %s", w.Code, w.Body.String())
	}

	var quoteResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &quoteResp); err != nil {
		t.Fatalf("Failed to unmarshal quote response: %v", err)
	}

	if quoteResp["action"] != "BUY" {
		t.Errorf("Expected action 'BUY', got %v", quoteResp["action"])
	}
	if quoteResp["deposit_usdc"] != "100.00000000" {
		t.Errorf("Expected deposit_usdc '100.00000000', got %v", quoteResp["deposit_usdc"])
	}
	if quoteResp["shares_received"] == "" || quoteResp["avg_price"] == "" {
		t.Errorf("Missing expected quote metrics: %+v", quoteResp)
	}

	// 2. Quote a SELL of 50 shares
	payloadSell := map[string]string{
		"action":  "SELL",
		"outcome": "YES",
		"shares":  "50.00000000",
	}
	bodyBytesSell, _ := json.Marshal(payloadSell)

	wSell := httptest.NewRecorder()
	reqSell, _ := http.NewRequest(http.MethodPost, "/api/v1/markets/will-bitcoin-hit-125k-in-2026/quote", bytes.NewReader(bodyBytesSell))
	reqSell.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wSell, reqSell)

	if wSell.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for sell quote, got %d. Body: %s", wSell.Code, wSell.Body.String())
	}

	var quoteSellResp map[string]interface{}
	if err := json.Unmarshal(wSell.Body.Bytes(), &quoteSellResp); err != nil {
		t.Fatalf("Failed to unmarshal sell quote: %v", err)
	}

	if quoteSellResp["action"] != "SELL" {
		t.Errorf("Expected action 'SELL', got %v", quoteSellResp["action"])
	}
	if quoteSellResp["payout_usdc"] == "" {
		t.Error("Expected payout_usdc to be present in sell quote")
	}
}

// 5. Task 4.4: Sandbox Faucet Claim & 5-Minute Cooldown Enforcement
func TestFaucetClaimAndCooldown(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	// First obtain a fresh guest token
	wAuth := httptest.NewRecorder()
	reqAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(wAuth, reqAuth)
	var authResp rest.GuestAuthResponse
	_ = json.Unmarshal(wAuth.Body.Bytes(), &authResp)

	token := authResp.Token

	// Claim 1: Should succeed and return $1,500.00 balance
	wFaucet1 := httptest.NewRecorder()
	reqFaucet1, _ := http.NewRequest(http.MethodPost, "/api/v1/faucet", nil)
	reqFaucet1.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(wFaucet1, reqFaucet1)

	if wFaucet1.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for first faucet claim, got %d. Body: %s", wFaucet1.Code, wFaucet1.Body.String())
	}

	var faucetResp map[string]interface{}
	_ = json.Unmarshal(wFaucet1.Body.Bytes(), &faucetResp)
	if faucetResp["new_balance"] != "1100.00000000" {
		t.Errorf("Expected new balance '1100.00000000', got %v", faucetResp["new_balance"])
	}

	// Claim 2: Immediate second claim must trigger HTTP 429 Cooldown
	wFaucet2 := httptest.NewRecorder()
	reqFaucet2, _ := http.NewRequest(http.MethodPost, "/api/v1/faucet", nil)
	reqFaucet2.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(wFaucet2, reqFaucet2)

	if wFaucet2.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected HTTP 429 for cooldown rejection, got %d. Body: %s", wFaucet2.Code, wFaucet2.Body.String())
	}
}

// 7. Admin Market Creation & Verification Endpoints
func TestAdminCreateMarketAndVerify(t *testing.T) {
	pool, cfg, router := getTestEnv(t)
	defer pool.Close()

	adminToken := getSuperadminToken(t, pool, cfg)

	// 1. Test GET /api/v1/admin/verify unauthorized
	wVerifyFail := httptest.NewRecorder()
	reqVerifyFail, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/verify", nil)
	router.ServeHTTP(wVerifyFail, reqVerifyFail)
	if wVerifyFail.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthorized verify, got %d", wVerifyFail.Code)
	}

	// 2. Test GET /api/v1/admin/verify authorized
	wVerifyOk := httptest.NewRecorder()
	reqVerifyOk, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/verify", nil)
	reqVerifyOk.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(wVerifyOk, reqVerifyOk)
	if wVerifyOk.Code != http.StatusOK {
		t.Errorf("Expected 200 for authorized verify, got %d. Body: %s", wVerifyOk.Code, wVerifyOk.Body.String())
	}

	// 3. Test POST /api/v1/admin/markets
	createPayload := map[string]string{
		"title":                   "Will Mars Sample Return Launch Before 2030?",
		"description":             "Resolves YES if NASA/ESA launches the Mars Sample Return mission spacecraft by Dec 31, 2029.",
		"category":                "science",
		"resolution_source":       "Official NASA / ESA mission press releases.",
		"resolution_date":         "2029-12-31T23:59:59Z",
		"initial_collateral_usdc": "10000.00000000",
		"initial_probability_yes": "0.35000000",
	}
	payloadBytes, _ := json.Marshal(createPayload)

	wCreate := httptest.NewRecorder()
	reqCreate, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/markets", bytes.NewReader(payloadBytes))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(wCreate, reqCreate)

	if wCreate.Code != http.StatusCreated {
		t.Fatalf("Expected 201 for market creation, got %d. Body: %s", wCreate.Code, wCreate.Body.String())
	}

	var createResp map[string]interface{}
	if err := json.Unmarshal(wCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("Failed to parse market creation response: %v", err)
	}

	if createResp["id"] == "" || createResp["slug"] == "" {
		t.Errorf("Expected id and slug in response: %+v", createResp)
	}
	if createResp["probability_yes_pct"] != "35.00" {
		t.Errorf("Expected probability_yes_pct '35.00', got %v", createResp["probability_yes_pct"])
	}
	if createResp["collateral_reserve"] != "10000.00000000" {
		t.Errorf("Expected collateral '10000.00000000', got %v", createResp["collateral_reserve"])
	}
}

// 6. Task 4.4: Portfolio Read Endpoint
func TestPortfolioReadEndpoint(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	// 1. Unauthenticated request should return 401
	wUnauth := httptest.NewRecorder()
	reqUnauth, _ := http.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	router.ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401 for unauthenticated portfolio read, got %d", wUnauth.Code)
	}

	// 2. Authenticated request
	wAuth := httptest.NewRecorder()
	reqAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(wAuth, reqAuth)
	var authResp rest.GuestAuthResponse
	_ = json.Unmarshal(wAuth.Body.Bytes(), &authResp)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 for portfolio read, got %d. Body: %s", w.Code, w.Body.String())
	}

	var portfolio rest.PortfolioResponse
	if err := json.Unmarshal(w.Body.Bytes(), &portfolio); err != nil {
		t.Fatalf("Failed to parse portfolio JSON: %v", err)
	}

	if portfolio.CashBalanceUSDC != "1000.00000000" {
		t.Errorf("Expected cash balance '1000.00000000', got '%s'", portfolio.CashBalanceUSDC)
	}
	if portfolio.TotalPortfolioValue != "1000.00000000" {
		t.Errorf("Expected total portfolio value '1000.00000000', got '%s'", portfolio.TotalPortfolioValue)
	}
}
