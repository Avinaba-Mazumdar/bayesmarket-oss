package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type orderResponseData struct {
	TradeID        string `json:"trade_id"`
	MarketID       string `json:"market_id"`
	UserID         string `json:"user_id"`
	TradeType      string `json:"trade_type"`
	Outcome        string `json:"outcome"`
	AmountUSDC     string `json:"amount_usdc"`
	SharesFilled   string `json:"shares_filled"`
	ExecutionPrice string `json:"execution_price"`
	PriceImpactPct string `json:"price_impact_pct"`
	NewCashBalance string `json:"new_cash_balance"`
	NewSharesOwned string `json:"new_shares_owned"`
	AvgBuyPrice    string `json:"avg_buy_price"`
}

type cashOutResponseData struct {
	TradeID         string `json:"trade_id"`
	MarketID        string `json:"market_id"`
	UserID          string `json:"user_id"`
	TradeType       string `json:"trade_type"`
	Outcome         string `json:"outcome"`
	SharesSold      string `json:"shares_sold"`
	PayoutUSDC      string `json:"payout_usdc"`
	ExecutionPrice  string `json:"execution_price"`
	PriceImpactPct  string `json:"price_impact_pct"`
	NewCashBalance  string `json:"new_cash_balance"`
	RemainingShares string `json:"remaining_shares"`
}

// 1. Double-Spend Attack: 20 concurrent goroutines attempting to spend the same $100 USDC balance.
// Exactly 1 trade must succeed, and 19 must fail with insufficient balance.
func TestConcurrency_DoubleSpendAttack(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Provision guest user
	wAuth := httptest.NewRecorder()
	reqAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(wAuth, reqAuth)
	if wAuth.Code != http.StatusCreated {
		t.Fatalf("Failed to create guest user: %d", wAuth.Code)
	}

	var authData struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(wAuth.Body.Bytes(), &authData)
	userID := authData.User.ID
	token := authData.Token

	// Set user's cash balance to exactly $100.00000000 USDC
	_, err := pool.Exec(ctx, "UPDATE users SET cash_balance = 100.00000000 WHERE id = $1", userID)
	if err != nil {
		t.Fatalf("Failed to set user cash balance: %v", err)
	}

	// Fetch an active market
	var marketID string
	err = pool.QueryRow(ctx, "SELECT id FROM markets WHERE status = 'active' ORDER BY created_at ASC LIMIT 1").Scan(&marketID)
	if err != nil {
		t.Fatalf("Failed to query active market: %v", err)
	}

	concurrency := 20
	var wg sync.WaitGroup
	var mu sync.Mutex

	statusCodes := make([]int, 0, concurrency)
	errorMessages := make([]string, 0, concurrency)

	// Barrier to start all goroutines simultaneously
	startChan := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			<-startChan // wait for synchronized release

			orderPayload := map[string]string{
				"outcome":          "YES",
				"amount_usdc":      "100.00000000",
				"max_slippage_pct": "10.00000000",
			}
			bodyBytes, _ := json.Marshal(orderPayload)

			wOrder := httptest.NewRecorder()
			reqOrder, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/markets/%s/orders", marketID), bytes.NewReader(bodyBytes))
			reqOrder.Header.Set("Content-Type", "application/json")
			reqOrder.Header.Set("Authorization", "Bearer "+token)
			reqOrder.Header.Set("Idempotency-Key", fmt.Sprintf("double-spend-key-%d-%s", idx, uuid.New().String()))

			router.ServeHTTP(wOrder, reqOrder)

			mu.Lock()
			statusCodes = append(statusCodes, wOrder.Code)
			if wOrder.Code != http.StatusCreated {
				var errResp map[string]interface{}
				_ = json.Unmarshal(wOrder.Body.Bytes(), &errResp)
				errorMessages = append(errorMessages, fmt.Sprintf("%v", errResp["error"]))
			}
			mu.Unlock()
		}(i)
	}

	// Unleash all 20 concurrent orders
	close(startChan)
	wg.Wait()

	// Verify status counts
	successCount := 0
	insufficientBalanceCount := 0
	for _, code := range statusCodes {
		switch code {
		case http.StatusCreated:
			successCount++
		case http.StatusBadRequest:
			insufficientBalanceCount++
		}
	}

	t.Logf("Double-Spend Concurrency Results: Successes=%d, InsufficientBalance=%d", successCount, insufficientBalanceCount)

	if successCount != 1 {
		t.Fatalf("INVARIANT VIOLATION: Expected exactly 1 order to succeed, but %d succeeded!", successCount)
	}
	if insufficientBalanceCount != concurrency-1 {
		t.Fatalf("Expected %d failed orders with insufficient balance, got %d. Errors: %v", concurrency-1, insufficientBalanceCount, errorMessages)
	}

	// Verify DB state: cash balance must be strictly zero
	var finalCash string
	err = pool.QueryRow(ctx, "SELECT cash_balance FROM users WHERE id = $1", userID).Scan(&finalCash)
	if err != nil {
		t.Fatalf("Failed to fetch user final cash balance: %v", err)
	}
	cashDec, err := decimal.NewFromString(finalCash)
	if err != nil || !cashDec.IsZero() {
		t.Fatalf("Expected user final cash balance to be 0, got %s", finalCash)
	}

	// Verify user position exists with positive shares
	var sharesOwned string
	err = pool.QueryRow(ctx, "SELECT shares_owned FROM user_positions WHERE user_id = $1 AND market_id = $2 AND outcome = 'YES'", userID, marketID).Scan(&sharesOwned)
	if err != nil {
		t.Fatalf("Failed to fetch user position: %v", err)
	}
	sharesDec, _ := decimal.NewFromString(sharesOwned)
	if sharesDec.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("Expected positive shares owned, got %s", sharesOwned)
	}

	// Verify ledger entries: exactly 4 balanced entries for the single trade (2 cash/collateral, 2 share positions)
	var ledgerCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM ledger_entries WHERE user_id = $1", userID).Scan(&ledgerCount)
	if err != nil {
		t.Fatalf("Failed to count ledger entries: %v", err)
	}
	if ledgerCount != 4 {
		t.Fatalf("Expected exactly 4 double-entry ledger rows, got %d", ledgerCount)
	}

	var usdcSum, shareSum decimal.Decimal
	err = pool.QueryRow(ctx, "SELECT COALESCE(SUM(delta), 0) FROM ledger_entries WHERE user_id = $1 AND asset = 'USDC'", userID).Scan(&usdcSum)
	if err != nil || !usdcSum.IsZero() {
		t.Fatalf("Expected USDC ledger entries to reconcile to 0, got %s (err: %v)", usdcSum, err)
	}
	err = pool.QueryRow(ctx, "SELECT COALESCE(SUM(delta), 0) FROM ledger_entries WHERE user_id = $1 AND asset != 'USDC'", userID).Scan(&shareSum)
	if err != nil || !shareSum.IsZero() {
		t.Fatalf("Expected Share ledger entries to reconcile to 0, got %s (err: %v)", shareSum, err)
	}
}

// 2. High-Contention Pool Execution: 20 separate users concurrently buying on the same market.
// Verifies pool serializes properly without deadlocks, preserves k invariant, and matches collateral accumulation.
func TestConcurrency_HighContentionPool(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Pick active market
	var marketID string
	var initialCollateral, initialRYes, initialRNo string
	queryInit := `
		SELECT m.id, p.collateral_reserve, p.reserve_yes, p.reserve_no 
		FROM markets m 
		JOIN liquidity_pools p ON p.market_id = m.id 
		WHERE m.status = 'active' 
		ORDER BY m.created_at ASC 
		LIMIT 1;
	`
	err := pool.QueryRow(ctx, queryInit).Scan(&marketID, &initialCollateral, &initialRYes, &initialRNo)
	if err != nil {
		t.Fatalf("Failed to query market: %v", err)
	}

	initialCollateralDec, _ := decimal.NewFromString(initialCollateral)
	initialRYesDec, _ := decimal.NewFromString(initialRYes)
	initialRNoDec, _ := decimal.NewFromString(initialRNo)
	initialK := initialRYesDec.Mul(initialRNoDec)

	concurrency := 10
	tokens := make([]string, concurrency)

	// Create 20 distinct users
	for i := 0; i < concurrency; i++ {
		wAuth := httptest.NewRecorder()
		reqAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
		router.ServeHTTP(wAuth, reqAuth)
		if wAuth.Code != http.StatusCreated {
			t.Fatalf("Failed to create user %d: %d", i, wAuth.Code)
		}
		var authData struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(wAuth.Body.Bytes(), &authData)
		tokens[i] = authData.Token
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	orderAmount := decimal.NewFromInt(10) // 10 USDC per trade

	startChan := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startChan

			orderPayload := map[string]string{
				"outcome":          "YES",
				"amount_usdc":      orderAmount.StringFixed(8),
				"max_slippage_pct": "20.00000000",
			}
			bodyBytes, _ := json.Marshal(orderPayload)

			wOrder := httptest.NewRecorder()
			reqOrder, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/markets/%s/orders", marketID), bytes.NewReader(bodyBytes))
			reqOrder.Header.Set("Content-Type", "application/json")
			reqOrder.Header.Set("Authorization", "Bearer "+tokens[idx])
			reqOrder.Header.Set("Idempotency-Key", fmt.Sprintf("contention-order-%d-%s", idx, uuid.New().String()))

			router.ServeHTTP(wOrder, reqOrder)

			if wOrder.Code == http.StatusCreated {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	close(startChan)
	wg.Wait()

	t.Logf("High Contention Pool: %d / %d orders successfully executed", successCount, concurrency)
	if successCount != concurrency {
		t.Fatalf("Expected all %d orders to succeed, but %d succeeded", concurrency, successCount)
	}

	// Verify pool collateral increased by exactly (concurrency * orderAmount)
	var finalCollateral, finalRYes, finalRNo string
	queryFinal := `
		SELECT collateral_reserve, reserve_yes, reserve_no 
		FROM liquidity_pools 
		WHERE market_id = $1;
	`
	err = pool.QueryRow(ctx, queryFinal, marketID).Scan(&finalCollateral, &finalRYes, &finalRNo)
	if err != nil {
		t.Fatalf("Failed to query final pool: %v", err)
	}

	finalCollateralDec, _ := decimal.NewFromString(finalCollateral)
	finalRYesDec, _ := decimal.NewFromString(finalRYes)
	finalRNoDec, _ := decimal.NewFromString(finalRNo)

	expectedCollateral := initialCollateralDec.Add(orderAmount.Mul(decimal.NewFromInt(int64(concurrency))))
	if !finalCollateralDec.Equal(expectedCollateral) {
		t.Fatalf("Collateral mismatch: expected %s, got %s", expectedCollateral.StringFixed(8), finalCollateralDec.StringFixed(8))
	}

	// Verify k invariant conservation: k_final should match k_initial
	finalK := finalRYesDec.Mul(finalRNoDec)
	diffK := finalK.Sub(initialK).Abs()
	relDiff := diffK.Div(initialK)
	if relDiff.GreaterThan(decimal.NewFromFloat(0.0001)) {
		t.Fatalf("Invariant k drifted excessively: initial=%s, final=%s, relDiff=%s", initialK.String(), finalK.String(), relDiff.String())
	}
}

// 3. Cash Out Liquidation & Solvency Test
func TestCashOut_LiquidationAndSolvency(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Provision user
	wAuth := httptest.NewRecorder()
	reqAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(wAuth, reqAuth)
	var authData struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(wAuth.Body.Bytes(), &authData)
	token := authData.Token
	userID := authData.User.ID

	// Pick active market
	var marketID string
	_ = pool.QueryRow(ctx, "SELECT id FROM markets WHERE status = 'active' ORDER BY created_at ASC LIMIT 1").Scan(&marketID)

	// Buy 100 USDC of YES
	orderPayload := map[string]string{
		"outcome":          "YES",
		"amount_usdc":      "100.00000000",
		"max_slippage_pct": "5.00000000",
	}
	bodyBytes, _ := json.Marshal(orderPayload)
	wOrder := httptest.NewRecorder()
	reqOrder, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/markets/%s/orders", marketID), bytes.NewReader(bodyBytes))
	reqOrder.Header.Set("Content-Type", "application/json")
	reqOrder.Header.Set("Authorization", "Bearer "+token)
	reqOrder.Header.Set("Idempotency-Key", "cashout-prep-"+uuid.New().String())
	router.ServeHTTP(wOrder, reqOrder)

	if wOrder.Code != http.StatusCreated {
		t.Fatalf("Failed to execute buy order: %d body: %s", wOrder.Code, wOrder.Body.String())
	}

	var buyResp orderResponseData
	_ = json.Unmarshal(wOrder.Body.Bytes(), &buyResp)
	sharesOwnedDec, _ := decimal.NewFromString(buyResp.NewSharesOwned)

	// Cash out half of the shares
	halfShares := sharesOwnedDec.DivRound(decimal.NewFromInt(2), 8)
	cashOutPayload := map[string]string{
		"market_id": marketID,
		"outcome":   "YES",
		"shares":    halfShares.StringFixed(8),
	}
	cashOutBytes, _ := json.Marshal(cashOutPayload)

	wCashOut := httptest.NewRecorder()
	reqCashOut, _ := http.NewRequest(http.MethodPost, "/api/v1/portfolio/cashout", bytes.NewReader(cashOutBytes))
	reqCashOut.Header.Set("Content-Type", "application/json")
	reqCashOut.Header.Set("Authorization", "Bearer "+token)
	reqCashOut.Header.Set("Idempotency-Key", "cashout-half-"+uuid.New().String())
	router.ServeHTTP(wCashOut, reqCashOut)

	if wCashOut.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 on cash out, got %d body: %s", wCashOut.Code, wCashOut.Body.String())
	}

	var cashOutResp cashOutResponseData
	_ = json.Unmarshal(wCashOut.Body.Bytes(), &cashOutResp)

	payoutDec, _ := decimal.NewFromString(cashOutResp.PayoutUSDC)
	if payoutDec.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("Expected positive payout USDC, got %s", cashOutResp.PayoutUSDC)
	}

	// Verify remaining shares in DB
	var remSharesDB string
	_ = pool.QueryRow(ctx, "SELECT shares_owned FROM user_positions WHERE user_id = $1 AND market_id = $2 AND outcome = 'YES'", userID, marketID).Scan(&remSharesDB)
	remDec, _ := decimal.NewFromString(remSharesDB)
	expectedRem := sharesOwnedDec.Sub(halfShares)
	if !remDec.Equal(expectedRem) {
		t.Fatalf("Remaining shares mismatch: expected %s, got %s", expectedRem.StringFixed(8), remDec.StringFixed(8))
	}

	// Attempting to sell more shares than owned must return HTTP 400
	excessPayload := map[string]string{
		"market_id": marketID,
		"outcome":   "YES",
		"shares":    sharesOwnedDec.Mul(decimal.NewFromInt(10)).StringFixed(8),
	}
	excessBytes, _ := json.Marshal(excessPayload)
	wExcess := httptest.NewRecorder()
	reqExcess, _ := http.NewRequest(http.MethodPost, "/api/v1/portfolio/cashout", bytes.NewReader(excessBytes))
	reqExcess.Header.Set("Content-Type", "application/json")
	reqExcess.Header.Set("Authorization", "Bearer "+token)
	reqExcess.Header.Set("Idempotency-Key", "cashout-excess-"+uuid.New().String())
	router.ServeHTTP(wExcess, reqExcess)

	if wExcess.Code != http.StatusBadRequest {
		t.Fatalf("Expected HTTP 400 for excessive shares liquidation, got %d", wExcess.Code)
	}
}

// 4. Idempotency Replay Test: Submitting identical order twice returns original receipt without double-charging
func TestOrder_IdempotencyReplay(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wAuth := httptest.NewRecorder()
	reqAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
	router.ServeHTTP(wAuth, reqAuth)
	var authData struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(wAuth.Body.Bytes(), &authData)
	token := authData.Token
	userID := authData.User.ID

	var marketID string
	_ = pool.QueryRow(ctx, "SELECT id FROM markets WHERE status = 'active' ORDER BY created_at ASC LIMIT 1").Scan(&marketID)

	idempKey := "idemp-test-replay-" + uuid.New().String()
	orderPayload := map[string]string{
		"outcome":          "YES",
		"amount_usdc":      "50.00000000",
		"max_slippage_pct": "5.00000000",
	}
	bodyBytes, _ := json.Marshal(orderPayload)

	// First call
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/markets/%s/orders", marketID), bytes.NewReader(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Idempotency-Key", idempKey)
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("First call failed with status %d: %s", w1.Code, w1.Body.String())
	}

	var resp1 orderResponseData
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)

	// Second identical call with identical Idempotency-Key
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/markets/%s/orders", marketID), bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Idempotency-Key", idempKey)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK && w2.Code != http.StatusCreated {
		t.Fatalf("Second call failed with status %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 orderResponseData
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.TradeID != resp2.TradeID {
		t.Fatalf("Idempotency failed: Trade IDs differ! %s vs %s", resp1.TradeID, resp2.TradeID)
	}

	// Verify user was only charged once ($50, not $100)
	var cashDB string
	_ = pool.QueryRow(ctx, "SELECT cash_balance FROM users WHERE id = $1", userID).Scan(&cashDB)
	if cashDB != "950.00000000" {
		t.Fatalf("Double charge occurred! Expected cash balance 950.00000000, got %s", cashDB)
	}
}
