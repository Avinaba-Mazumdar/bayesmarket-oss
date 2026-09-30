package rest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/bayesmarket/bayesmarket/internal/transport/ws"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// SystemAdminID is the designated system administrator UUID used for admin-mediated resolutions without a user JWT.
var SystemAdminID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// CreateMarketRequest defines input payload for creating a new prediction market.
type CreateMarketRequest struct {
	Title                 string `json:"title" binding:"required"`
	Description           string `json:"description" binding:"required"`
	Category              string `json:"category" binding:"required"`
	ResolutionSource      string `json:"resolution_source" binding:"required"`
	ResolutionDate        string `json:"resolution_date" binding:"required"`
	ImageURL              string `json:"image_url"`
	InitialCollateralUSDC string `json:"initial_collateral_usdc"` // e.g. "10000.00000000" (default 10000)
	InitialProbabilityYes string `json:"initial_probability_yes"` // e.g. "0.50000000" or "50" (default 0.5)
}

// ResolveMarketRequest defines input payload for settling a prediction market.
type ResolveMarketRequest struct {
	WinningOutcome string `json:"winning_outcome" binding:"required"`
	OracleProof    string `json:"oracle_proof" binding:"required"`
}

// ResolveMarketResponse defines the settlement receipt returned upon successful resolution.
type ResolveMarketResponse struct {
	Status           string `json:"status"`
	MarketID         string `json:"market_id"`
	WinningOutcome   string `json:"winning_outcome"`
	TotalPayoutUSDC  string `json:"total_payout_usdc"`
	WinnersCredited  int    `json:"winners_credited"`
	OracleProof      string `json:"oracle_proof"`
	ProofHash        string `json:"proof_hash,omitempty"`
	SettlementDigest string `json:"settlement_digest,omitempty"`
	ResolvedAt       string `json:"resolved_at"`
}

// oracleCacheEntry holds in-memory cached oracle proof verification with a 60-second TTL.
type oracleCacheEntry struct {
	proofHash  string
	outcome    string
	verifiedAt time.Time
}

type oracleCache struct {
	mu    sync.RWMutex
	items map[string]oracleCacheEntry
}

func newOracleCache() *oracleCache {
	return &oracleCache{items: make(map[string]oracleCacheEntry)}
}

func (c *oracleCache) get(marketID string) (oracleCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[marketID]
	if !ok || time.Since(entry.verifiedAt) > 60*time.Second {
		return oracleCacheEntry{}, false
	}
	return entry, true
}

func (c *oracleCache) set(marketID, outcome, proofHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[marketID] = oracleCacheEntry{
		proofHash:  proofHash,
		outcome:    outcome,
		verifiedAt: time.Now(),
	}
}

// AdminHandler manages administrative operations including market resolution and payout distribution.
type AdminHandler struct {
	pool        *pgxpool.Pool
	hub         *ws.Hub
	locks       *marketLockRegistry
	cache       *oracleCache
	marketCache *MarketCache
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(pool *pgxpool.Pool, hub *ws.Hub) *AdminHandler {
	return &AdminHandler{
		pool:  pool,
		hub:   hub,
		locks: newMarketLockRegistry(),
		cache: newOracleCache(),
	}
}

// SetMarketCache attaches a MarketCache instance for resolution invalidation.
func (h *AdminHandler) SetMarketCache(mc *MarketCache) {
	h.marketCache = mc
}

// HandleResolveMarket resolves a prediction market, credits winning share holders, and records double-entry ledger audits.
//
// POST /api/v1/admin/markets/:id/resolve
func (h *AdminHandler) HandleResolveMarket(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	marketIDParam := strings.TrimSpace(c.Param("id"))
	marketID, err := uuid.Parse(marketIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_market_id", "message": "Market ID must be a valid UUID"})
		return
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "idempotency_key_required",
			"message": "Idempotency-Key header is required for market resolution",
		})
		return
	}

	var req ResolveMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Payload must include winning_outcome ('YES'|'NO') and oracle_proof"})
		return
	}

	outcome, valErr := ParseOutcome(req.WinningOutcome)
	if valErr != nil {
		c.JSON(valErr.StatusCode, gin.H{"error": valErr.ErrorCode, "message": valErr.Message})
		return
	}
	winningOutcome := string(outcome)

	oracleProof := strings.TrimSpace(req.OracleProof)
	if oracleProof == "" || len(oracleProof) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_proof", "message": "oracle_proof must contain substantive resolution evidence"})
		return
	}
	proofHash := fmt.Sprintf("%x", sha256.Sum256([]byte(oracleProof)))

	// Determine actor UUID for idempotency registration and ledger audit.
	// If the request authenticated via static ADMIN_TOKEN without a user JWT, attribute to the
	// deterministic System Administrator account (never a random user from the users table).
	actorID, exists := middleware.GetUserID(c)
	if !exists || actorID == uuid.Nil {
		actorID = SystemAdminID
		if h.pool != nil {
			ensureAdminQuery := `
				INSERT INTO users (id, is_guest, auth_provider, name, email)
				VALUES ($1, false, 'system', 'System Administrator', 'admin@bayesmarket.internal')
				ON CONFLICT (id) DO NOTHING;
			`
			_, _ = h.pool.Exec(ctx, ensureAdminQuery, SystemAdminID)
		}
	}

	// Acquire in-process mutex lock to prevent concurrent mutation collisions
	unlock := h.locks.acquire("market:" + marketID.String())
	defer unlock()

	// Check idempotency cache first
	cachedResp, found, err := GetCachedIdempotencyResponse(ctx, h.pool, actorID, "resolve", idempotencyKey)
	if err == nil && found {
		c.Header("X-Cache-Lookup", "HIT-IDEMPOTENT")
		c.Data(http.StatusOK, "application/json", cachedResp)
		return
	}

	// 60-second in-memory oracle cache evaluation
	if entry, cached := h.cache.get(marketID.String()); cached {
		if entry.outcome != winningOutcome {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "oracle_proof_conflict",
				"message": fmt.Sprintf("Recent cached oracle verification indicated outcome '%s', conflicting with requested '%s'", entry.outcome, winningOutcome),
			})
			return
		}
		if entry.proofHash != proofHash {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "oracle_proof_conflict",
				"message": "Recent cached oracle verification utilized a conflicting proof hash for this market",
			})
			return
		}
	}

	// Begin SERIALIZABLE database transaction for atomic payout settlement
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to initiate settlement transaction"})
		return
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Lock and verify market state
	var currentStatus, currentWinningOutcome, resolutionSource string
	marketQuery := `
		SELECT status, COALESCE(winning_outcome, ''), resolution_source
		FROM markets
		WHERE id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, marketQuery, marketID).Scan(&currentStatus, &currentWinningOutcome, &resolutionSource)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "market_not_found", "message": "Market does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to lock market row"})
		return
	}

	if currentStatus == "resolved" {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "market_already_resolved",
			"message": fmt.Sprintf("Market is already resolved with outcome '%s'", currentWinningOutcome),
		})
		return
	}

	// 2. Lock liquidity pool collateral
	var collateralReserve decimal.Decimal
	poolQuery := `
		SELECT collateral_reserve
		FROM liquidity_pools
		WHERE market_id = $1
		FOR UPDATE;
	`
	err = tx.QueryRow(ctx, poolQuery, marketID).Scan(&collateralReserve)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to lock liquidity pool collateral"})
		return
	}

	// 3. Mark market as resolved
	updateMarketQuery := `
		UPDATE markets
		SET status = 'resolved', winning_outcome = $1
		WHERE id = $2;
	`
	if _, err := tx.Exec(ctx, updateMarketQuery, winningOutcome, marketID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to update market status"})
		return
	}

	// 4. Distribute complete-set collateral ($1.00 per share) to winning positions
	winningPositionsQuery := `
		SELECT id, user_id, shares_owned
		FROM user_positions
		WHERE market_id = $1 AND outcome = $2 AND shares_owned > 0
		FOR UPDATE;
	`
	rows, err := tx.Query(ctx, winningPositionsQuery, marketID, winningOutcome)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to query winning positions"})
		return
	}

	type winRecord struct {
		posID  uuid.UUID
		userID uuid.UUID
		shares decimal.Decimal
	}
	var winners []winRecord
	for rows.Next() {
		var w winRecord
		if scanErr := rows.Scan(&w.posID, &w.userID, &w.shares); scanErr == nil {
			winners = append(winners, w)
		}
	}
	rows.Close()

	totalPayout := decimal.Zero
	for _, w := range winners {
		totalPayout = totalPayout.Add(w.shares.Truncate(8))
	}

	// Solvency assertion: pool collateral must be sufficient to back all winning payouts
	if totalPayout.GreaterThan(collateralReserve) {
		log.Printf("[CRITICAL RESOLUTION SOLVENCY FAILURE] MarketID=%s totalPayout=%s exceeds collateralReserve=%s",
			marketID.String(), totalPayout.StringFixed(8), collateralReserve.StringFixed(8))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "collateral_insolvency",
			"message": fmt.Sprintf("Settlement total payout (%s USDC) exceeds collateral reserve (%s USDC)", totalPayout.StringFixed(4), collateralReserve.StringFixed(4)),
		})
		return
	}

	settlementTxID := uuid.New()

	for _, w := range winners {
		// Each winning share redeems for exactly $1.00000000 USDC
		payout := w.shares.Truncate(8)

		// Credit user cash balance
		creditUserQuery := `
			UPDATE users
			SET cash_balance = cash_balance + $1, last_active = NOW()
			WHERE id = $2;
		`
		if _, err := tx.Exec(ctx, creditUserQuery, payout, w.userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to credit winning trader"})
			return
		}

		// Zero out winning shares
		zeroPosQuery := `
			UPDATE user_positions
			SET shares_owned = 0, updated_at = NOW()
			WHERE id = $1;
		`
		if _, err := tx.Exec(ctx, zeroPosQuery, w.posID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to zero winning position"})
			return
		}

		// Double-entry bookkeeping ledger
		insertLedger := `
			INSERT INTO ledger_entries (transaction_id, user_id, market_id, account, asset, delta, entry_type)
			VALUES 
				($1, $2, $3, 'user_cash', 'USDC', $4, 'settlement'),
				($1, $2, $3, 'pool_collateral', 'USDC', $5, 'settlement'),
				($1, $2, $3, $6, $7, $8, 'settlement'),
				($1, $2, $3, 'pool_shares', $7, $9, 'settlement');
		`
		posAccount := "position_" + strings.ToLower(winningOutcome)
		negPayout := payout.Neg()
		negShares := w.shares.Neg()
		if _, err := tx.Exec(ctx, insertLedger,
			settlementTxID, w.userID, marketID,
			payout, negPayout, posAccount, winningOutcome, negShares, w.shares,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to record settlement ledger entries"})
			return
		}
	}

	// 5. Zero out all losing positions for audit hygiene
	losingPositionsQuery := `
		SELECT id, user_id, outcome, shares_owned
		FROM user_positions
		WHERE market_id = $1 AND outcome != $2 AND shares_owned > 0
		FOR UPDATE;
	`
	loseRows, err := tx.Query(ctx, losingPositionsQuery, marketID, winningOutcome)
	if err == nil {
		type loseRecord struct {
			posID   uuid.UUID
			userID  uuid.UUID
			outcome string
			shares  decimal.Decimal
		}
		var losers []loseRecord
		for loseRows.Next() {
			var l loseRecord
			if scanErr := loseRows.Scan(&l.posID, &l.userID, &l.outcome, &l.shares); scanErr == nil {
				losers = append(losers, l)
			}
		}
		loseRows.Close()

		for _, l := range losers {
			zeroLoseQuery := `
				UPDATE user_positions
				SET shares_owned = 0, updated_at = NOW()
				WHERE id = $1;
			`
			_, _ = tx.Exec(ctx, zeroLoseQuery, l.posID)

			insertLoseLedger := `
				INSERT INTO ledger_entries (transaction_id, user_id, market_id, account, asset, delta, entry_type)
				VALUES 
					($1, $2, $3, $4, $5, $6, 'settlement'),
					($1, $2, $3, 'pool_shares', $5, $7, 'settlement');
			`
			posAccount := "position_" + strings.ToLower(l.outcome)
			negShares := l.shares.Neg()
			_, _ = tx.Exec(ctx, insertLoseLedger, settlementTxID, l.userID, marketID, posAccount, l.outcome, negShares, l.shares)
		}
	}

	// 6. Deduct total payout from liquidity pool collateral reserve
	remainingCollateral := collateralReserve.Sub(totalPayout)
	updatePoolQuery := `
		UPDATE liquidity_pools
		SET collateral_reserve = $1, updated_at = NOW()
		WHERE market_id = $2;
	`
	if _, err := tx.Exec(ctx, updatePoolQuery, remainingCollateral, marketID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to update pool collateral reserve"})
		return
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	settlementPayload := fmt.Sprintf("%s:%s:%s:%s:%s", marketID.String(), winningOutcome, totalPayout.StringFixed(8), proofHash, nowStr)
	settlementDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(settlementPayload)))

	log.Printf("[AUTHORITATIVE RESOLUTION AUDIT] MarketID=%s WinningOutcome=%s TotalPayout=%s Winners=%d ProofHash=%s SettlementDigest=%s ActorID=%s ResolutionSource=%q",
		marketID.String(), winningOutcome, totalPayout.StringFixed(8), len(winners), proofHash, settlementDigest, actorID.String(), resolutionSource)

	responseObj := ResolveMarketResponse{
		Status:           "resolved",
		MarketID:         marketID.String(),
		WinningOutcome:   winningOutcome,
		TotalPayoutUSDC:  totalPayout.StringFixed(8),
		WinnersCredited:  len(winners),
		OracleProof:      oracleProof,
		ProofHash:        proofHash,
		SettlementDigest: settlementDigest,
		ResolvedAt:       nowStr,
	}

	// 7. Store idempotency receipt
	respJSON, err := json.Marshal(responseObj)
	if err == nil && actorID != uuid.Nil {
		insertIdempQuery := `
			INSERT INTO idempotency_keys (actor_id, operation, idempotency_key, response)
			VALUES ($1, 'resolve', $2, $3)
			ON CONFLICT (actor_id, operation, idempotency_key) DO NOTHING;
		`
		_, _ = tx.Exec(ctx, insertIdempQuery, actorID, idempotencyKey, respJSON)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to commit resolution transaction"})
		return
	}

	if h.marketCache != nil {
		h.marketCache.Invalidate(marketID.String())
	}

	// 8. Cache verified oracle response for 60 seconds
	h.cache.set(marketID.String(), winningOutcome, proofHash)

	// 9. Broadcast MARKET_RESOLVED WebSocket frame to connected clients
	if h.hub != nil {
		h.hub.BroadcastMarketResolved(ws.MarketResolvedMessage{
			Type:           ws.MessageTypeMarketResolved,
			MarketID:       marketID.String(),
			WinningOutcome: winningOutcome,
			Timestamp:      nowStr,
		})
	}

	c.JSON(http.StatusOK, responseObj)
}

// HandleVerifyAdmin validates the admin credential and returns 200 OK.
//
// GET /api/v1/admin/verify
func (h *AdminHandler) HandleVerifyAdmin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "authorized",
		"message": "Admin credential is valid",
	})
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyTitle(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = slugRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
		slug = strings.Trim(slug, "-")
	}
	if slug == "" {
		slug = "market-" + uuid.New().String()[:8]
	}
	return slug
}

func parseResolutionDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid resolution date; expected RFC3339 or ISO-8601 (e.g. 2026-12-31T23:59:59Z)")
}

// HandleCreateMarket provisions a new prediction market and calibrates its CPMM liquidity pool.
//
// POST /api/v1/admin/markets
func (h *AdminHandler) HandleCreateMarket(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var req CreateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Required fields: title, description, category, resolution_source, resolution_date",
		})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Category = strings.ToLower(strings.TrimSpace(req.Category))
	req.ResolutionSource = strings.TrimSpace(req.ResolutionSource)
	if req.Title == "" || req.Description == "" || req.Category == "" || req.ResolutionSource == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Title, description, category, and resolution_source cannot be empty",
		})
		return
	}

	resolutionDate, err := parseResolutionDate(req.ResolutionDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_date",
			"message": err.Error(),
		})
		return
	}
	if resolutionDate.Before(time.Now().UTC()) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_date",
			"message": "Resolution date must be in the future",
		})
		return
	}

	// 1. Initial Collateral (default 10,000 USDC)
	collateral := decimal.NewFromInt(10000)
	if req.InitialCollateralUSDC != "" {
		cDec, err := decimal.NewFromString(strings.TrimSpace(req.InitialCollateralUSDC))
		if err != nil || cDec.LessThan(decimal.NewFromInt(100)) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_collateral",
				"message": "Initial pool collateral must be at least 100 USDC",
			})
			return
		}
		collateral = cDec
	}

	// 2. Initial Probability for YES (default 0.50 / 50%)
	probYes := decimal.NewFromFloat(0.50)
	if req.InitialProbabilityYes != "" {
		pDec, err := decimal.NewFromString(strings.TrimSpace(req.InitialProbabilityYes))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_probability",
				"message": "Initial probability must be a decimal (0.01 to 0.99) or percentage (1 to 99)",
			})
			return
		}
		if pDec.GreaterThan(decimal.NewFromInt(1)) {
			pDec = pDec.Div(decimal.NewFromInt(100))
		}
		if pDec.LessThan(decimal.NewFromFloat(0.01)) || pDec.GreaterThan(decimal.NewFromFloat(0.99)) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_probability",
				"message": "Initial probability must be between 1% (0.01) and 99% (0.99)",
			})
			return
		}
		probYes = pDec
	}

	probNo := decimal.NewFromInt(1).Sub(probYes)

	// Fixed-point CPMM inventory derivation:
	// Setting R_yes = Collateral * (1 - P_yes) and R_no = Collateral * P_yes guarantees:
	// R_yes + R_no = Collateral, so P_yes = R_no / Collateral
	reserveYes := collateral.Mul(probNo).Truncate(8)
	reserveNo := collateral.Mul(probYes).Truncate(8)
	kInvariant := reserveYes.Mul(reserveNo)

	// 3. Unique slug generation & atomic database insertion with collision retry
	baseSlug := slugifyTitle(req.Title)
	imageURL := strings.TrimSpace(req.ImageURL)
	if imageURL == "" {
		imageURL = fmt.Sprintf("/assets/markets/%s.webp", req.Category)
	}

	var (
		marketID  uuid.UUID
		createdAt time.Time
		slug      string
	)

	queryMarket := `
		INSERT INTO markets (slug, title, description, category, image_url, resolution_source, resolution_date, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		RETURNING id, created_at;
	`
	queryPool := `
		INSERT INTO liquidity_pools (market_id, reserve_yes, reserve_no, collateral_reserve, k_invariant, total_volume_usdc, lock_version)
		VALUES ($1, $2, $3, $4, $5, 0, 0);
	`

	for attempt := 0; attempt < 5; attempt++ {
		if attempt == 0 {
			slug = baseSlug
		} else {
			slug = fmt.Sprintf("%s-%s", baseSlug, uuid.New().String()[:6])
		}

		tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to begin transaction",
			})
			return
		}

		err = tx.QueryRow(ctx, queryMarket,
			slug, req.Title, req.Description, req.Category, imageURL, req.ResolutionSource, resolutionDate,
		).Scan(&marketID, &createdAt)

		if err != nil {
			_ = tx.Rollback(ctx)
			// If unique slug constraint violation occurs, retry with an appended unique suffix.
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "23505") {
				continue
			}
			log.Printf("[Admin CreateMarket] failed to insert market record: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to insert market record",
			})
			return
		}

		if _, err := tx.Exec(ctx, queryPool, marketID, reserveYes, reserveNo, collateral, kInvariant); err != nil {
			_ = tx.Rollback(ctx)
			log.Printf("[Admin CreateMarket] failed to insert liquidity pool: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to insert liquidity pool",
			})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			log.Printf("[Admin CreateMarket] failed to commit market creation: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to commit market creation transaction",
			})
			return
		}

		// Market creation succeeded!
		break
	}

	if marketID == uuid.Nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "slug_conflict",
			"message": "Failed to allocate a unique market slug after multiple attempts",
		})
		return
	}

	if h.marketCache != nil {
		h.marketCache.InvalidateAll()
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":                  marketID.String(),
		"slug":                slug,
		"title":               req.Title,
		"description":         req.Description,
		"category":            req.Category,
		"image_url":           imageURL,
		"resolution_source":   req.ResolutionSource,
		"resolution_date":     resolutionDate.Format(time.RFC3339),
		"status":              "active",
		"reserve_yes":         reserveYes.StringFixed(8),
		"reserve_no":          reserveNo.StringFixed(8),
		"collateral_reserve":  collateral.StringFixed(8),
		"k_invariant":         kInvariant.StringFixed(16),
		"probability_yes":     probYes.StringFixed(4),
		"probability_no":      probNo.StringFixed(4),
		"probability_yes_pct": probYes.Mul(decimal.NewFromInt(100)).StringFixed(2),
		"probability_no_pct":  probNo.Mul(decimal.NewFromInt(100)).StringFixed(2),
		"created_at":          createdAt.Format(time.RFC3339),
	})
}
