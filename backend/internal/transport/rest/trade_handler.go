package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/amm"
	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/bayesmarket/bayesmarket/internal/transport/ws"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// TelemetryBroadcaster defines the contract for broadcasting real-time market telemetry.
type TelemetryBroadcaster interface {
	BroadcastPriceUpdate(msg ws.PriceUpdateMessage)
	BroadcastTradeEvent(msg ws.TradeEventMessage)
}

// MarketCacheInvalidator defines the contract for invalidating market cache entries.
type MarketCacheInvalidator interface {
	Invalidate(key string)
}

// TradeHandler handles atomic order placements and share cash-out liquidations.
type TradeHandler struct {
	pool  *pgxpool.Pool
	locks *marketLockRegistry
	hub   TelemetryBroadcaster
	cache MarketCacheInvalidator
}

// NewTradeHandler constructs a TradeHandler.
func NewTradeHandler(pool *pgxpool.Pool, hubOpt ...*ws.Hub) *TradeHandler {
	var hub TelemetryBroadcaster
	if len(hubOpt) > 0 && hubOpt[0] != nil {
		hub = hubOpt[0]
	}
	return &TradeHandler{pool: pool, locks: newMarketLockRegistry(), hub: hub}
}

// SetCache attaches a cache invalidator instance for event-driven cache invalidation.
func (h *TradeHandler) SetCache(cache MarketCacheInvalidator) {
	h.cache = cache
}

// PlaceOrderRequest defines the input payload for placing a buy order.
type PlaceOrderRequest struct {
	Outcome        string `json:"outcome"`
	AmountUSDC     string `json:"amount_usdc"`
	MaxSlippagePct string `json:"max_slippage_pct,omitempty"`
}

// OrderResponse represents the authoritative execution receipt for a trade.
type OrderResponse struct {
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
	CreatedAt      string `json:"created_at"`
}

// CashOutRequest defines the input payload for liquidating outcome shares back to USDC.
type CashOutRequest struct {
	MarketID      string `json:"market_id"`
	Outcome       string `json:"outcome"`
	Shares        string `json:"shares"`
	MinPayoutUSDC string `json:"min_payout_usdc,omitempty"`
}

// CashOutResponse represents the authoritative receipt for a share liquidation.
type CashOutResponse struct {
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
	CreatedAt       string `json:"created_at"`
}

// HandlePlaceOrder atomically executes a complete-set CPMM buy order.
//
// POST /api/v1/markets/:id/orders
func (h *TradeHandler) HandlePlaceOrder(c *gin.Context) {
	timeout := 30 * time.Second
	if gin.Mode() == gin.TestMode {
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required"})
		return
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "idempotency_key_required",
			"message": "Idempotency-Key header is required for order mutations",
		})
		return
	}

	marketIDParam := strings.TrimSpace(c.Param("id"))
	if marketIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_market", "message": "Market ID is required"})
		return
	}

	// In-process lock serialization per market to prevent abort storms under high concurrency
	unlock := h.locks.acquire("market:" + marketIDParam)
	defer unlock()

	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Malformed JSON payload"})
		return
	}

	outcome, valErr := ParseOutcome(req.Outcome)
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}

	amount, valErr := ParsePositiveDecimal(req.AmountUSDC, "amount_usdc")
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}

	maxSlippage, valErr := ParseSlippagePct(req.MaxSlippagePct, decimal.NewFromFloat(5.0))
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}

	// 1. Fast path: check idempotency receipt outside tx
	cachedResp, found, err := GetCachedIdempotencyResponse(ctx, h.pool, userID, "place_order", idempotencyKey)
	if err == nil && found {
		c.Data(http.StatusOK, "application/json", cachedResp)
		return
	}

	// 2. Execute with resilient serializable retry loop
	finalResponse, err := ExecuteSerializableWithRetry(ctx, 25, func() (*OrderResponse, error) {
		return h.executeOrderTx(ctx, userID, idempotencyKey, marketIDParam, outcome, amount, maxSlippage)
	})

	if err != nil {
		if errors.Is(err, ErrIdempotencyReplay) {
			if cached, ok, _ := GetCachedIdempotencyResponse(ctx, h.pool, userID, "place_order", idempotencyKey); ok {
				c.Data(http.StatusOK, "application/json", cached)
				return
			}
		}

		var appErr *AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.StatusCode, gin.H{"error": appErr.ErrorCode, "message": appErr.Message})
			return
		}

		if IsSerializationOrDeadlock(err) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "concurrency_conflict",
				"message": "Order transaction experienced contention after retries. Please retry.",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "execution_failed", "message": "Order execution failed. Please retry."})
		return
	}

	c.JSON(http.StatusCreated, finalResponse)
}

func (h *TradeHandler) executeOrderTx(
	ctx context.Context,
	userID uuid.UUID,
	idempotencyKey string,
	marketIDParam string,
	outcome amm.Outcome,
	amount decimal.Decimal,
	maxSlippage decimal.Decimal,
) (*OrderResponse, error) {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Step 1: User lock & sufficiency check (Hierarchy Level 1)
	currentCashBalance, err := lockAndVerifyUserBalance(ctx, tx, userID, amount)
	if err != nil {
		return nil, err
	}

	// Step 2: Market lock & active check (Hierarchy Level 2)
	marketUUID, err := lockAndVerifyActiveMarket(ctx, tx, marketIDParam)
	if err != nil {
		return nil, err
	}

	// Step 3: Liquidity Pool lock (Hierarchy Level 3)
	poolReserves, totalVolume, err := lockLiquidityPool(ctx, tx, marketUUID)
	if err != nil {
		return nil, err
	}

	// Step 4: Mathematical AMM Execution
	quote, err := amm.CalculateCompleteSetBuy(amount, outcome, poolReserves)
	if err != nil {
		log.Printf("[AMM Buy Order] calculation error: %v", err)
		return nil, &AppError{StatusCode: http.StatusBadRequest, ErrorCode: "amm_error", Message: "Invalid trade parameters for buy order"}
	}

	if quote.PriceImpactPct.GreaterThan(maxSlippage) {
		return nil, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "slippage_exceeded",
			Message:    "Price impact (" + quote.PriceImpactPct.StringFixed(4) + "%) exceeds maximum allowed slippage (" + maxSlippage.StringFixed(4) + "%)",
		}
	}

	// Step 5: Check existing user position state
	isNewPos, finalSharesOwned, finalAvgPrice, newInvested, err := computeUpdatedUserPosition(ctx, tx, userID, marketUUID, outcome, quote.SharesReceived, quote.AvgPrice, amount)
	if err != nil {
		return nil, err
	}

	newCashBalance := currentCashBalance.Sub(amount)
	tradeID := uuid.New()
	positionAccount := "position_" + strings.ToLower(string(outcome))

	resp := &OrderResponse{
		TradeID:        tradeID.String(),
		MarketID:       marketUUID.String(),
		UserID:         userID.String(),
		TradeType:      "BUY",
		Outcome:        string(outcome),
		AmountUSDC:     amount.StringFixed(8),
		SharesFilled:   quote.SharesReceived.StringFixed(8),
		ExecutionPrice: quote.AvgPrice.StringFixed(8),
		PriceImpactPct: quote.PriceImpactPct.StringFixed(8),
		NewCashBalance: newCashBalance.StringFixed(8),
		NewSharesOwned: finalSharesOwned.StringFixed(8),
		AvgBuyPrice:    finalAvgPrice.StringFixed(8),
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	respBytes, _ := json.Marshal(resp)

	// Step 6: Pipeline all mutation queries via pgx.Batch
	batch := &pgx.Batch{}

	// 1. Deduct cash
	batch.Queue(`
		UPDATE users 
		SET cash_balance = cash_balance - $1, last_active = NOW() 
		WHERE id = $2;
	`, amount, userID)

	// 2. Update liquidity pool
	batch.Queue(`
		UPDATE liquidity_pools 
		SET reserve_yes = $1, 
		    reserve_no = $2, 
		    collateral_reserve = $3, 
		    total_volume_usdc = total_volume_usdc + $4, 
		    lock_version = lock_version + 1, 
		    updated_at = NOW() 
		WHERE market_id = $5;
	`, quote.NewReserveYes, quote.NewReserveNo, quote.NewCollateral, amount, marketUUID)

	// 3. Insert trade audit record
	batch.Queue(`
		INSERT INTO trades (
			id, market_id, user_id, idempotency_key, trade_type, outcome, 
			amount_usdc, shares_filled, execution_price, price_impact_pct, created_at
		) VALUES ($1, $2, $3, $4, 'BUY', $5, $6, $7, $8, $9, NOW());
	`, tradeID, marketUUID, userID, idempotencyKey, string(outcome),
		amount, quote.SharesReceived, quote.AvgPrice, quote.PriceImpactPct,
	)

	// 4. Upsert user position
	if isNewPos {
		batch.Queue(`
			INSERT INTO user_positions (
				user_id, market_id, outcome, shares_owned, avg_buy_price, total_invested_usdc, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW());
		`, userID, marketUUID, string(outcome), finalSharesOwned, finalAvgPrice, amount)
	} else {
		batch.Queue(`
			UPDATE user_positions 
			SET shares_owned = $1, avg_buy_price = $2, total_invested_usdc = $3, updated_at = NOW() 
			WHERE user_id = $4 AND market_id = $5 AND outcome = $6;
		`, finalSharesOwned, finalAvgPrice, newInvested, userID, marketUUID, string(outcome))
	}

	// 5. Immutable ledger entries (double-entry bookkeeping)
	batch.Queue(`
		INSERT INTO ledger_entries (
			transaction_id, user_id, market_id, account, asset, delta, entry_type, created_at
		) VALUES 
		($1, $2, $3, 'user_cash', 'USDC', $4, 'trade', NOW()),
		($1, $2, $3, 'pool_collateral', 'USDC', $5, 'trade', NOW()),
		($1, $2, $3, $6, $7, $8, 'trade', NOW());
	`, tradeID, userID, marketUUID,
		amount.Neg(),
		amount,
		positionAccount,
		string(outcome),
		quote.SharesReceived,
	)

	// 6. Idempotency receipt
	QueueIdempotencyRecord(batch, userID, "place_order", idempotencyKey, respBytes)

	if err := executeBatchAndCheckIdempotency(ctx, tx, batch); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Cache invalidation & Telemetry Broadcast
	h.invalidateMarketCache(marketUUID.String(), marketIDParam)
	h.broadcastOrderTelemetry(resp, marketUUID, quote.NewReserveYes, quote.NewReserveNo, totalVolume.Add(amount))

	return resp, nil
}

// HandleCashOut atomically liquidates shares of an outcome back to virtual USDC.
//
// POST /api/v1/portfolio/cashout
func (h *TradeHandler) HandleCashOut(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()

	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required"})
		return
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "idempotency_key_required",
			"message": "Idempotency-Key header is required for cashout mutations",
		})
		return
	}

	var req CashOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Malformed JSON payload"})
		return
	}

	marketIDParam := strings.TrimSpace(req.MarketID)
	if marketIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_market", "message": "market_id is required"})
		return
	}

	unlock := h.locks.acquire("market:" + marketIDParam)
	defer unlock()

	outcome, valErr := ParseOutcome(req.Outcome)
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}

	shares, valErr := ParsePositiveDecimal(req.Shares, "shares")
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}

	minPayout, valErr := ParseSlippagePct(req.MinPayoutUSDC, decimal.Zero)
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}

	// 1. Fast path: check idempotency receipt outside tx
	cachedResp, found, err := GetCachedIdempotencyResponse(ctx, h.pool, userID, "cashout", idempotencyKey)
	if err == nil && found {
		c.Data(http.StatusOK, "application/json", cachedResp)
		return
	}

	// 2. Execute with resilient serializable retry loop
	finalResponse, err := ExecuteSerializableWithRetry(ctx, 25, func() (*CashOutResponse, error) {
		return h.executeCashOutTx(ctx, userID, idempotencyKey, marketIDParam, outcome, shares, minPayout)
	})

	if err != nil {
		if errors.Is(err, ErrIdempotencyReplay) {
			if cached, ok, _ := GetCachedIdempotencyResponse(ctx, h.pool, userID, "cashout", idempotencyKey); ok {
				c.Data(http.StatusOK, "application/json", cached)
				return
			}
		}

		var appErr *AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.StatusCode, gin.H{"error": appErr.ErrorCode, "message": appErr.Message})
			return
		}

		if IsSerializationOrDeadlock(err) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "concurrency_conflict",
				"message": "Cashout transaction experienced contention after retries. Please retry.",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "execution_failed", "message": "Cashout execution failed. Please retry."})
		return
	}

	c.JSON(http.StatusOK, finalResponse)
}

func (h *TradeHandler) executeCashOutTx(
	ctx context.Context,
	userID uuid.UUID,
	idempotencyKey string,
	marketIDParam string,
	outcome amm.Outcome,
	shares decimal.Decimal,
	minPayout decimal.Decimal,
) (*CashOutResponse, error) {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Step 1: User lock (Hierarchy Level 1)
	var currentCashBalance decimal.Decimal
	err = tx.QueryRow(ctx, `SELECT cash_balance FROM users WHERE id = $1 FOR UPDATE;`, userID).Scan(&currentCashBalance)
	if err != nil {
		return nil, err
	}

	// Step 2: Market lock (Hierarchy Level 2)
	marketUUID, err := lockAndVerifyActiveMarket(ctx, tx, marketIDParam)
	if err != nil {
		return nil, err
	}

	// Step 3: User Position lock (Hierarchy Level 3)
	ownedShares, _, totalInvested, err := lockAndVerifyUserPosition(ctx, tx, userID, marketUUID, outcome, shares)
	if err != nil {
		return nil, err
	}

	// Step 4: Liquidity Pool lock (Hierarchy Level 4)
	poolReserves, totalVolume, err := lockLiquidityPool(ctx, tx, marketUUID)
	if err != nil {
		return nil, err
	}

	// Step 5: AMM Complete Set Sell Calculation
	quote, err := amm.CalculateCompleteSetSell(shares, outcome, poolReserves)
	if err != nil {
		log.Printf("[AMM Sell Order] calculation error: %v", err)
		return nil, &AppError{StatusCode: http.StatusBadRequest, ErrorCode: "amm_error", Message: "Invalid trade parameters for cashout"}
	}

	if minPayout.GreaterThan(decimal.Zero) && quote.PayoutUSDC.LessThan(minPayout) {
		return nil, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "slippage_exceeded",
			Message:    "Payout (" + quote.PayoutUSDC.StringFixed(4) + " USDC) is below minimum acceptable (" + minPayout.StringFixed(4) + " USDC)",
		}
	}

	// Step 6: Compute remaining shares and weighted invested basis
	newCashBalance := currentCashBalance.Add(quote.PayoutUSDC)
	remainingShares := ownedShares.Sub(shares)
	var remainingInvested decimal.Decimal
	if ownedShares.GreaterThan(decimal.Zero) {
		ratio := remainingShares.DivRound(ownedShares, 8)
		remainingInvested = totalInvested.Mul(ratio).Round(8)
	} else {
		remainingInvested = decimal.Zero
	}

	tradeID := uuid.New()
	positionAccount := "position_" + strings.ToLower(string(outcome))

	resp := &CashOutResponse{
		TradeID:         tradeID.String(),
		MarketID:        marketUUID.String(),
		UserID:          userID.String(),
		TradeType:       "SELL",
		Outcome:         string(outcome),
		SharesSold:      shares.StringFixed(8),
		PayoutUSDC:      quote.PayoutUSDC.StringFixed(8),
		ExecutionPrice:  quote.AvgPrice.StringFixed(8),
		PriceImpactPct:  quote.PriceImpactPct.StringFixed(8),
		NewCashBalance:  newCashBalance.StringFixed(8),
		RemainingShares: remainingShares.StringFixed(8),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	respBytes, _ := json.Marshal(resp)

	// Step 7: Pipeline mutations via pgx.Batch
	batch := &pgx.Batch{}

	// 1. Credit user cash
	batch.Queue(`
		UPDATE users 
		SET cash_balance = cash_balance + $1, last_active = NOW() 
		WHERE id = $2;
	`, quote.PayoutUSDC, userID)

	// 2. Decrement user position
	batch.Queue(`
		UPDATE user_positions 
		SET shares_owned = $1, total_invested_usdc = $2, updated_at = NOW() 
		WHERE user_id = $3 AND market_id = $4 AND outcome = $5;
	`, remainingShares, remainingInvested, userID, marketUUID, string(outcome))

	// 3. Update liquidity pool
	batch.Queue(`
		UPDATE liquidity_pools 
		SET reserve_yes = $1, 
		    reserve_no = $2, 
		    collateral_reserve = $3, 
		    total_volume_usdc = total_volume_usdc + $4, 
		    lock_version = lock_version + 1, 
		    updated_at = NOW() 
		WHERE market_id = $5;
	`, quote.NewReserveYes, quote.NewReserveNo, quote.NewCollateral, quote.PayoutUSDC, marketUUID)

	// 4. Insert trade record (SELL)
	batch.Queue(`
		INSERT INTO trades (
			id, market_id, user_id, idempotency_key, trade_type, outcome, 
			amount_usdc, shares_filled, execution_price, price_impact_pct, created_at
		) VALUES ($1, $2, $3, $4, 'SELL', $5, $6, $7, $8, $9, NOW());
	`, tradeID, marketUUID, userID, idempotencyKey, string(outcome),
		quote.PayoutUSDC, shares, quote.AvgPrice, quote.PriceImpactPct,
	)

	// 5. Immutable double-entry ledger entries
	batch.Queue(`
		INSERT INTO ledger_entries (
			transaction_id, user_id, market_id, account, asset, delta, entry_type, created_at
		) VALUES 
		($1, $2, $3, 'pool_collateral', 'USDC', $4, 'trade', NOW()),
		($1, $2, $3, 'user_cash', 'USDC', $5, 'trade', NOW()),
		($1, $2, $3, $6, $7, $8, 'trade', NOW());
	`, tradeID, userID, marketUUID,
		quote.PayoutUSDC.Neg(),
		quote.PayoutUSDC,
		positionAccount,
		string(outcome),
		shares.Neg(),
	)

	// 6. Store idempotency receipt
	QueueIdempotencyRecord(batch, userID, "cashout", idempotencyKey, respBytes)

	if err := executeBatchAndCheckIdempotency(ctx, tx, batch); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Cache invalidation & Telemetry Broadcast
	h.invalidateMarketCache(marketUUID.String(), marketIDParam)
	h.broadcastCashOutTelemetry(resp, marketUUID, quote.NewReserveYes, quote.NewReserveNo, totalVolume.Add(quote.PayoutUSDC))

	return resp, nil
}

// --- Focused Locking & Data Helpers ---

func lockAndVerifyUserBalance(ctx context.Context, tx pgx.Tx, userID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := tx.QueryRow(ctx, `SELECT cash_balance FROM users WHERE id = $1 FOR UPDATE;`, userID).Scan(&balance)
	if err != nil {
		return decimal.Zero, err
	}
	if balance.LessThan(amount) {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "insufficient_balance",
			Message:    "Insufficient virtual USDC balance to fund order",
		}
	}
	return balance, nil
}

func lockAndVerifyActiveMarket(ctx context.Context, tx pgx.Tx, marketIDParam string) (uuid.UUID, error) {
	var marketUUID uuid.UUID
	var marketStatus string
	var resolutionDate time.Time
	query := `SELECT id, status, resolution_date FROM markets WHERE id::text = $1 OR slug = $1 FOR UPDATE;`
	err := tx.QueryRow(ctx, query, marketIDParam).Scan(&marketUUID, &marketStatus, &resolutionDate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, &AppError{StatusCode: http.StatusNotFound, ErrorCode: "not_found", Message: "Market not found"}
		}
		return uuid.Nil, err
	}
	if marketStatus != "active" {
		return uuid.Nil, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "market_not_active",
			Message:    "Market is not open for trading",
		}
	}
	if time.Now().UTC().After(resolutionDate) {
		return uuid.Nil, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "market_expired",
			Message:    "Market trading has closed: resolution date has passed",
		}
	}
	return marketUUID, nil
}

func lockAndVerifyUserPosition(ctx context.Context, tx pgx.Tx, userID, marketUUID uuid.UUID, outcome amm.Outcome, requiredShares decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	var ownedShares, avgBuyPrice, totalInvested decimal.Decimal
	query := `
		SELECT shares_owned, avg_buy_price, total_invested_usdc 
		FROM user_positions 
		WHERE user_id = $1 AND market_id = $2 AND outcome = $3 
		FOR UPDATE;
	`
	err := tx.QueryRow(ctx, query, userID, marketUUID, string(outcome)).Scan(&ownedShares, &avgBuyPrice, &totalInvested)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || ownedShares.LessThan(requiredShares) {
			return decimal.Zero, decimal.Zero, decimal.Zero, &AppError{
				StatusCode: http.StatusBadRequest,
				ErrorCode:  "insufficient_shares",
				Message:    "Insufficient outcome shares to liquidate",
			}
		}
		return decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	if ownedShares.LessThan(requiredShares) {
		return decimal.Zero, decimal.Zero, decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "insufficient_shares",
			Message:    "Insufficient outcome shares to liquidate",
		}
	}
	return ownedShares, avgBuyPrice, totalInvested, nil
}

func lockLiquidityPool(ctx context.Context, tx pgx.Tx, marketUUID uuid.UUID) (amm.PoolReserves, decimal.Decimal, error) {
	var rYes, rNo, collateral, totalVolume decimal.Decimal
	var lockVersion int
	query := `
		SELECT reserve_yes, reserve_no, collateral_reserve, total_volume_usdc, lock_version 
		FROM liquidity_pools 
		WHERE market_id = $1 
		FOR UPDATE;
	`
	err := tx.QueryRow(ctx, query, marketUUID).Scan(&rYes, &rNo, &collateral, &totalVolume, &lockVersion)
	if err != nil {
		return amm.PoolReserves{}, decimal.Zero, err
	}
	return amm.PoolReserves{
		ReserveYes:        rYes,
		ReserveNo:         rNo,
		CollateralReserve: collateral,
	}, totalVolume, nil
}

func computeUpdatedUserPosition(
	ctx context.Context,
	tx pgx.Tx,
	userID, marketUUID uuid.UUID,
	outcome amm.Outcome,
	sharesReceived, avgPrice, amount decimal.Decimal,
) (bool, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	var existingShares, existingAvg, existingInvested decimal.Decimal
	query := `
		SELECT shares_owned, avg_buy_price, total_invested_usdc 
		FROM user_positions 
		WHERE user_id = $1 AND market_id = $2 AND outcome = $3 
		FOR UPDATE;
	`
	err := tx.QueryRow(ctx, query, userID, marketUUID, string(outcome)).Scan(&existingShares, &existingAvg, &existingInvested)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, sharesReceived, avgPrice, amount, nil
		}
		return false, decimal.Zero, decimal.Zero, decimal.Zero, err
	}

	finalSharesOwned := existingShares.Add(sharesReceived)
	newInvested := existingInvested.Add(amount)
	finalAvgPrice := decimal.Zero
	if finalSharesOwned.GreaterThan(decimal.Zero) {
		finalAvgPrice = newInvested.DivRound(finalSharesOwned, 8)
	}

	return false, finalSharesOwned, finalAvgPrice, newInvested, nil
}

func executeBatchAndCheckIdempotency(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			if IsIdempotencyReplayError(err) {
				_ = br.Close()
				return ErrIdempotencyReplay
			}
			_ = br.Close()
			return err
		}
	}
	return br.Close()
}

func (h *TradeHandler) invalidateMarketCache(keys ...string) {
	if h.cache == nil {
		return
	}
	for _, key := range keys {
		if key != "" {
			h.cache.Invalidate(key)
		}
	}
}

func (h *TradeHandler) broadcastOrderTelemetry(resp *OrderResponse, marketUUID uuid.UUID, rYes, rNo, newTotalVolume decimal.Decimal) {
	if h.hub == nil {
		return
	}
	spotYes, spotNo, _ := amm.CalculateSpotPrices(amm.PoolReserves{ReserveYes: rYes, ReserveNo: rNo})
	h.hub.BroadcastPriceUpdate(ws.PriceUpdateMessage{
		MarketID: marketUUID.String(),
		YesPrice: spotYes.StringFixed(8),
		NoPrice:  spotNo.StringFixed(8),
		Reserves: &ws.ReservesPayload{
			Yes: rYes.StringFixed(8),
			No:  rNo.StringFixed(8),
		},
		TotalVolumeUSDC: newTotalVolume.StringFixed(8),
		Timestamp:       resp.CreatedAt,
	})
	h.hub.BroadcastTradeEvent(ws.TradeEventMessage{
		TradeID:    resp.TradeID,
		MarketID:   resp.MarketID,
		TradeType:  "BUY",
		Outcome:    resp.Outcome,
		Shares:     resp.SharesFilled,
		Price:      resp.ExecutionPrice,
		AmountUSDC: resp.AmountUSDC,
		Timestamp:  resp.CreatedAt,
	})
}

func (h *TradeHandler) broadcastCashOutTelemetry(resp *CashOutResponse, marketUUID uuid.UUID, rYes, rNo, newTotalVolume decimal.Decimal) {
	if h.hub == nil {
		return
	}
	spotYes, spotNo, _ := amm.CalculateSpotPrices(amm.PoolReserves{ReserveYes: rYes, ReserveNo: rNo})
	h.hub.BroadcastPriceUpdate(ws.PriceUpdateMessage{
		MarketID: marketUUID.String(),
		YesPrice: spotYes.StringFixed(8),
		NoPrice:  spotNo.StringFixed(8),
		Reserves: &ws.ReservesPayload{
			Yes: rYes.StringFixed(8),
			No:  rNo.StringFixed(8),
		},
		TotalVolumeUSDC: newTotalVolume.StringFixed(8),
		Timestamp:       resp.CreatedAt,
	})
	h.hub.BroadcastTradeEvent(ws.TradeEventMessage{
		TradeID:    resp.TradeID,
		MarketID:   resp.MarketID,
		TradeType:  "SELL",
		Outcome:    resp.Outcome,
		Shares:     resp.SharesSold,
		Price:      resp.ExecutionPrice,
		AmountUSDC: resp.PayoutUSDC,
		Timestamp:  resp.CreatedAt,
	})
}
