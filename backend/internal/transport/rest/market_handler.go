package rest

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/amm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// MarketHandler handles prediction market discovery and authorative quote requests.
type MarketHandler struct {
	pool  *pgxpool.Pool
	cache *MarketCache
}

// NewMarketHandler constructs a MarketHandler.
func NewMarketHandler(pool *pgxpool.Pool, cacheOpt ...*MarketCache) *MarketHandler {
	var cache *MarketCache
	if len(cacheOpt) > 0 && cacheOpt[0] != nil {
		cache = cacheOpt[0]
	} else {
		cache = NewMarketCache(5 * time.Second)
	}
	return &MarketHandler{pool: pool, cache: cache}
}

// Cache returns the underlying MarketCache instance.
func (h *MarketHandler) Cache() *MarketCache {
	return h.cache
}

// MarketSummaryResponse details a market and its current implied probabilities.
type MarketSummaryResponse struct {
	ID                string           `json:"id"`
	Slug              string           `json:"slug"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	Category          string           `json:"category"`
	ImageURL          *string          `json:"image_url,omitempty"`
	ResolutionSource  string           `json:"resolution_source"`
	ResolutionDate    string           `json:"resolution_date"`
	Status            string           `json:"status"`
	ProbabilityYes    string           `json:"probability_yes"`
	ProbabilityNo     string           `json:"probability_no"`
	ProbabilityYesPct string           `json:"probability_yes_pct"`
	ProbabilityNoPct  string           `json:"probability_no_pct"`
	Reserves          ReservesResponse `json:"reserves"`
	CreatedAt         string           `json:"created_at"`
}

// ReservesResponse holds pool state as canonical decimal strings.
type ReservesResponse struct {
	ReserveYes        string `json:"reserve_yes"`
	ReserveNo         string `json:"reserve_no"`
	CollateralReserve string `json:"collateral_reserve"`
	TotalVolumeUSDC   string `json:"total_volume_usdc"`
}

// QuoteRequest represents the payload for execution quotes.
type QuoteRequest struct {
	Action     string `json:"action"`      // "BUY" (default) or "SELL"
	Outcome    string `json:"outcome"`     // "YES" or "NO"
	AmountUSDC string `json:"amount_usdc"` // Required for BUY
	Shares     string `json:"shares"`      // Required for SELL
}

// HandleGetMarkets returns active prediction markets with category filtering.
//
// GET /api/v1/markets
func (h *MarketHandler) HandleGetMarkets(c *gin.Context) {
	category := strings.ToLower(strings.TrimSpace(c.Query("category")))

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Fast path: check in-memory cache when not paginating offset
	if offset == 0 && h.cache != nil {
		if cached, ok := h.cache.GetMarkets(category); ok {
			if len(cached) > limit {
				cached = cached[:limit]
			}
			c.JSON(http.StatusOK, cached)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	query := `
		SELECT m.id, m.slug, m.title, m.description, m.category, m.image_url, 
		       m.resolution_source, m.resolution_date, m.status, m.created_at,
		       p.reserve_yes, p.reserve_no, p.collateral_reserve, p.total_volume_usdc
		FROM markets m
		JOIN liquidity_pools p ON p.market_id = m.id
		WHERE ($1 = '' OR m.category = $1) AND m.status = 'active'
		ORDER BY m.created_at ASC
		LIMIT $2 OFFSET $3;
	`
	rows, err := h.pool.Query(ctx, query, category, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to query prediction markets",
		})
		return
	}
	defer rows.Close()

	var markets []MarketSummaryResponse
	for rows.Next() {
		var (
			id, slug, title, desc, cat, resSource, status string
			imgURL                                        *string
			resDate, createdAt                            time.Time
			rYes, rNo, collateral, volume                 decimal.Decimal
		)

		err := rows.Scan(
			&id, &slug, &title, &desc, &cat, &imgURL,
			&resSource, &resDate, &status, &createdAt,
			&rYes, &rNo, &collateral, &volume,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to parse market row",
			})
			return
		}

		poolReserves := amm.PoolReserves{
			ReserveYes:        rYes,
			ReserveNo:         rNo,
			CollateralReserve: collateral,
		}
		pYes, pNo, _ := amm.CalculateSpotPrices(poolReserves)
		pYesPct := pYes.Mul(decimal.NewFromInt(100)).StringFixed(2)
		pNoPct := pNo.Mul(decimal.NewFromInt(100)).StringFixed(2)

		summary := MarketSummaryResponse{
			ID:                id,
			Slug:              slug,
			Title:             title,
			Description:       desc,
			Category:          cat,
			ImageURL:          imgURL,
			ResolutionSource:  resSource,
			ResolutionDate:    resDate.Format(time.RFC3339),
			Status:            status,
			ProbabilityYes:    pYes.StringFixed(8),
			ProbabilityNo:     pNo.StringFixed(8),
			ProbabilityYesPct: pYesPct,
			ProbabilityNoPct:  pNoPct,
			Reserves: ReservesResponse{
				ReserveYes:        rYes.StringFixed(8),
				ReserveNo:         rNo.StringFixed(8),
				CollateralReserve: collateral.StringFixed(8),
				TotalVolumeUSDC:   volume.StringFixed(8),
			},
			CreatedAt: createdAt.Format(time.RFC3339),
		}

		markets = append(markets, summary)

		// Populate single-market cache as well
		if h.cache != nil {
			if mUUID, parseErr := uuid.Parse(id); parseErr == nil {
				h.cache.SetMarket(id, slug, summary, poolReserves, mUUID)
			}
		}
	}

	if markets == nil {
		markets = []MarketSummaryResponse{}
	}

	if h.cache != nil {
		h.cache.SetMarkets(category, markets)
	}

	c.JSON(http.StatusOK, markets)
}

// HandleGetMarketByID returns detailed market metadata by UUID or Slug.
//
// GET /api/v1/markets/:id
func (h *MarketHandler) HandleGetMarketByID(c *gin.Context) {
	param := strings.TrimSpace(c.Param("id"))
	if param == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Market identifier is required"})
		return
	}

	// Fast path: check in-memory cache
	if h.cache != nil {
		if cachedSummary, _, _, ok := h.cache.GetMarket(param); ok && cachedSummary != nil {
			c.JSON(http.StatusOK, *cachedSummary)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	query := `
		SELECT m.id, m.slug, m.title, m.description, m.category, m.image_url, 
		       m.resolution_source, m.resolution_date, m.status, m.created_at,
		       p.reserve_yes, p.reserve_no, p.collateral_reserve, p.total_volume_usdc
		FROM markets m
		JOIN liquidity_pools p ON p.market_id = m.id
		WHERE m.id::text = $1 OR m.slug = $1;
	`
	var (
		id, slug, title, desc, cat, resSource, status string
		imgURL                                        *string
		resDate, createdAt                            time.Time
		rYes, rNo, collateral, volume                 decimal.Decimal
	)

	err := h.pool.QueryRow(ctx, query, param).Scan(
		&id, &slug, &title, &desc, &cat, &imgURL,
		&resSource, &resDate, &status, &createdAt,
		&rYes, &rNo, &collateral, &volume,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Prediction market not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to query market"})
		return
	}

	poolReserves := amm.PoolReserves{
		ReserveYes:        rYes,
		ReserveNo:         rNo,
		CollateralReserve: collateral,
	}
	pYes, pNo, _ := amm.CalculateSpotPrices(poolReserves)

	resp := MarketSummaryResponse{
		ID:                id,
		Slug:              slug,
		Title:             title,
		Description:       desc,
		Category:          cat,
		ImageURL:          imgURL,
		ResolutionSource:  resSource,
		ResolutionDate:    resDate.Format(time.RFC3339),
		Status:            status,
		ProbabilityYes:    pYes.StringFixed(8),
		ProbabilityNo:     pNo.StringFixed(8),
		ProbabilityYesPct: pYes.Mul(decimal.NewFromInt(100)).StringFixed(2),
		ProbabilityNoPct:  pNo.Mul(decimal.NewFromInt(100)).StringFixed(2),
		Reserves: ReservesResponse{
			ReserveYes:        rYes.StringFixed(8),
			ReserveNo:         rNo.StringFixed(8),
			CollateralReserve: collateral.StringFixed(8),
			TotalVolumeUSDC:   volume.StringFixed(8),
		},
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	if h.cache != nil {
		if mUUID, parseErr := uuid.Parse(id); parseErr == nil {
			h.cache.SetMarket(id, slug, resp, poolReserves, mUUID)
		}
	}

	c.JSON(http.StatusOK, resp)
}

// HandleMarketQuote computes an authoritative execution quote using the CPMM engine.
//
// POST /api/v1/markets/:id/quote
func (h *MarketHandler) HandleMarketQuote(c *gin.Context) {
	marketIDParam := strings.TrimSpace(c.Param("id"))
	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_payload",
			"message": "Malformed JSON quote request payload",
		})
		return
	}

	outcome := amm.Outcome(strings.ToUpper(strings.TrimSpace(req.Outcome)))
	if outcome != amm.OutcomeYES && outcome != amm.OutcomeNO {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_outcome",
			"message": "Outcome must be 'YES' or 'NO'",
		})
		return
	}

	action := strings.ToUpper(strings.TrimSpace(req.Action))
	if action == "" {
		action = "BUY"
	}
	if action != "BUY" && action != "SELL" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_action",
			"message": "Action must be 'BUY' or 'SELL'",
		})
		return
	}

	// 1. Check in-memory pool reserves first for sub-millisecond calculation
	var marketUUID uuid.UUID
	var poolReserves amm.PoolReserves
	var foundInCache bool

	if h.cache != nil {
		if _, cachedReserves, cachedUUID, ok := h.cache.GetMarket(marketIDParam); ok && cachedReserves != nil && cachedUUID != nil {
			marketUUID = *cachedUUID
			poolReserves = *cachedReserves
			foundInCache = true
		}
	}

	if !foundInCache {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		query := `
			SELECT m.id, m.status, m.resolution_date, p.reserve_yes, p.reserve_no, p.collateral_reserve
			FROM markets m
			JOIN liquidity_pools p ON p.market_id = m.id
			WHERE m.id::text = $1 OR m.slug = $1;
		`
		var marketStatus string
		var resDate time.Time
		var rYes, rNo, collateral decimal.Decimal
		err := h.pool.QueryRow(ctx, query, marketIDParam).Scan(&marketUUID, &marketStatus, &resDate, &rYes, &rNo, &collateral)
		if err != nil {
			if err == pgx.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Market not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to fetch market reserves"})
			return
		}

		if marketStatus != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "market_not_active", "message": "Market is not open for trading"})
			return
		}
		if time.Now().UTC().After(resDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "market_expired", "message": "Market trading has closed: resolution date has passed"})
			return
		}

		poolReserves = amm.PoolReserves{
			ReserveYes:        rYes,
			ReserveNo:         rNo,
			CollateralReserve: collateral,
		}

		if h.cache != nil {
			h.cache.SetMarket(marketUUID.String(), marketIDParam, MarketSummaryResponse{ID: marketUUID.String()}, poolReserves, marketUUID)
		}
	}

	if action == "BUY" {
		depositStr := strings.TrimSpace(req.AmountUSDC)
		if depositStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_amount", "message": "amount_usdc is required for BUY quote"})
			return
		}
		deposit, err := decimal.NewFromString(depositStr)
		if err != nil || deposit.LessThanOrEqual(decimal.Zero) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_amount", "message": "amount_usdc must be a positive decimal string"})
			return
		}

		quote, err := amm.CalculateCompleteSetBuy(deposit, outcome, poolReserves)
		if err != nil {
			log.Printf("[AMM Buy Quote] calculation error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "amm_error", "message": "Invalid trade parameters for buy quote"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"market_id":        marketUUID.String(),
			"action":           "BUY",
			"outcome":          string(outcome),
			"deposit_usdc":     quote.DepositUSDC.StringFixed(8),
			"shares_received":  quote.SharesReceived.StringFixed(8),
			"avg_price":        quote.AvgPrice.StringFixed(8),
			"initial_price":    quote.InitialPrice.StringFixed(8),
			"new_price":        quote.NewPrice.StringFixed(8),
			"price_impact_pct": quote.PriceImpactPct.StringFixed(8),
			"new_reserve_yes":  quote.NewReserveYes.StringFixed(8),
			"new_reserve_no":   quote.NewReserveNo.StringFixed(8),
			"new_collateral":   quote.NewCollateral.StringFixed(8),
		})
	} else {
		sharesStr := strings.TrimSpace(req.Shares)
		if sharesStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_shares", "message": "shares is required for SELL quote"})
			return
		}
		shares, err := decimal.NewFromString(sharesStr)
		if err != nil || shares.LessThanOrEqual(decimal.Zero) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_shares", "message": "shares must be a positive decimal string"})
			return
		}

		quote, err := amm.CalculateCompleteSetSell(shares, outcome, poolReserves)
		if err != nil {
			log.Printf("[AMM Sell Quote] calculation error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "amm_error", "message": "Invalid trade parameters for sell quote"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"market_id":        marketUUID.String(),
			"action":           "SELL",
			"outcome":          string(outcome),
			"shares_to_sell":   quote.SharesToSell.StringFixed(8),
			"payout_usdc":      quote.PayoutUSDC.StringFixed(8),
			"avg_price":        quote.AvgPrice.StringFixed(8),
			"initial_price":    quote.InitialPrice.StringFixed(8),
			"new_price":        quote.NewPrice.StringFixed(8),
			"price_impact_pct": quote.PriceImpactPct.StringFixed(8),
			"new_reserve_yes":  quote.NewReserveYes.StringFixed(8),
			"new_reserve_no":   quote.NewReserveNo.StringFixed(8),
			"new_collateral":   quote.NewCollateral.StringFixed(8),
		})
	}
}
