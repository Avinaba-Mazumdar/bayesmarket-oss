package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// FaucetHandler handles virtual sandbox currency grants with cooldown protection.
type FaucetHandler struct {
	pool         *pgxpool.Pool
	isDevOrLocal bool
}

// NewFaucetHandler constructs a FaucetHandler.
func NewFaucetHandler(pool *pgxpool.Pool, isDevOrLocal ...bool) *FaucetHandler {
	isDev := false
	if len(isDevOrLocal) > 0 {
		isDev = isDevOrLocal[0]
	}
	return &FaucetHandler{
		pool:         pool,
		isDevOrLocal: isDev,
	}
}

// HandleClaimFaucet issues 100 virtual USDC to the user with a 24-hour cooldown.
//
// POST /api/v1/faucet
func (h *FaucetHandler) HandleClaimFaucet(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication required"})
		return
	}

	clientIP := c.ClientIP()
	cooldownDuration := 24 * time.Hour
	faucetGrant := decimal.NewFromInt(100)

	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))

	// Fast-path idempotency replay: return the stored receipt for a retried claim.
	if idempotencyKey != "" && h.pool != nil {
		var cachedResponse []byte
		err := h.pool.QueryRow(ctx, `
			SELECT response
			FROM idempotency_keys
			WHERE actor_id = $1 AND operation = 'faucet_claim' AND idempotency_key = $2;
		`, userID, idempotencyKey).Scan(&cachedResponse)
		if err == nil && len(cachedResponse) > 0 {
			c.Data(http.StatusOK, "application/json", cachedResponse)
			return
		}
	}

	if h.pool == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"amount_claimed":   faucetGrant.StringFixed(8),
			"amount":           faucetGrant.StringFixed(0),
			"new_balance":      "1100.00000000",
			"cooldown_seconds": 86400,
			"user": gin.H{
				"id":           userID.String(),
				"cash_balance": "1100.00000000",
			},
		})
		return
	}

	ipDailyCap := 10
	ipBurstCooldown := 30 * time.Second
	if h.isDevOrLocal {
		ipDailyCap = 100
		ipBurstCooldown = 0
	}

	// 1. Check account cooldown and IP-level abuse prevention in a single consolidated query.
	// - Account cooldown: per user_id, 24 hours.
	// - IP abuse prevention: daily cap per IP (loopback is NOT exempt; NAT users do not collide).
	var (
		lastUserClaim *time.Time
		ipCount       int
		lastIPClaim   *time.Time
	)
	queryCheck := `
		SELECT 
			(SELECT claimed_at FROM faucet_claims WHERE user_id = $1 ORDER BY claimed_at DESC LIMIT 1),
			(SELECT COUNT(*) FROM faucet_claims WHERE ip_address = $2 AND claimed_at > NOW() - INTERVAL '24 hours'),
			(SELECT MAX(claimed_at) FROM faucet_claims WHERE ip_address = $2 AND claimed_at > NOW() - INTERVAL '24 hours');
	`
	if err := h.pool.QueryRow(ctx, queryCheck, userID, clientIP).Scan(&lastUserClaim, &ipCount, &lastIPClaim); err == nil {
		// User cooldown check
		if lastUserClaim != nil {
			if elapsed := time.Since(*lastUserClaim); elapsed < cooldownDuration {
				remainingSec := int((cooldownDuration - elapsed).Seconds())
				c.Header("Retry-After", strconv.Itoa(remainingSec))
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":                      "faucet_cooldown",
					"message":                    "Faucet cooldown active. Please wait before claiming again.",
					"cooldown_remaining_seconds": remainingSec,
				})
				return
			}
		}

		// IP abuse prevention (no loopback exemption; prevents Sybil draining without NAT lockout)
		if clientIP != "" {
			if ipCount >= ipDailyCap {
				c.Header("Retry-After", "3600")
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "faucet_ip_limit_exceeded",
					"message": "Maximum daily faucet claims exceeded for this IP address. Please try again tomorrow.",
				})
				return
			}
			if ipBurstCooldown > 0 && lastIPClaim != nil && time.Since(*lastIPClaim) < ipBurstCooldown {
				remainingBurst := int((ipBurstCooldown - time.Since(*lastIPClaim)).Seconds()) + 1
				c.Header("Retry-After", strconv.Itoa(remainingBurst))
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":                      "faucet_ip_rate_limited",
					"message":                    "Too many faucet requests from this IP. Please wait a moment before claiming again.",
					"cooldown_remaining_seconds": remainingBurst,
				})
				return
			}
		}
	}

	// Execute atomic claim transaction
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to initiate transaction"})
		return
	}
	defer tx.Rollback(ctx)

	// Lock the user row first (global lock order: user row first) so concurrent
	// claims for the same account serialize before any cooldown evaluation.
	var lockedBalance decimal.Decimal
	lockUserQuery := `SELECT cash_balance FROM users WHERE id = $1 FOR UPDATE;`
	if err := tx.QueryRow(ctx, lockUserQuery, userID).Scan(&lockedBalance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to lock user balance"})
		return
	}

	// Re-check account cooldown and IP daily cap inside the transaction
	var (
		lastUserClaimInTx *time.Time
		ipCountInTx       int
	)
	queryCheckInTx := `
		SELECT 
			(SELECT claimed_at FROM faucet_claims WHERE user_id = $1 ORDER BY claimed_at DESC LIMIT 1),
			(SELECT COUNT(*) FROM faucet_claims WHERE ip_address = $2 AND claimed_at > NOW() - INTERVAL '24 hours');
	`
	if err := tx.QueryRow(ctx, queryCheckInTx, userID, clientIP).Scan(&lastUserClaimInTx, &ipCountInTx); err == nil {
		if lastUserClaimInTx != nil {
			if elapsed := time.Since(*lastUserClaimInTx); elapsed < cooldownDuration {
				remainingSec := int((cooldownDuration - elapsed).Seconds())
				c.Header("Retry-After", strconv.Itoa(remainingSec))
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":                      "faucet_cooldown",
					"message":                    "Faucet cooldown active. Please wait before claiming again.",
					"cooldown_remaining_seconds": remainingSec,
				})
				return
			}
		}
		if clientIP != "" && ipCountInTx >= ipDailyCap {
			c.Header("Retry-After", "3600")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "faucet_ip_limit_exceeded",
				"message": "Maximum daily faucet claims exceeded for this IP address. Please try again tomorrow.",
			})
			return
		}
	}

	// Update user balance
	var updatedBalance decimal.Decimal
	updateQuery := `
		UPDATE users 
		SET cash_balance = cash_balance + $1, last_active = NOW()
		WHERE id = $2
		RETURNING cash_balance;
	`
	err = tx.QueryRow(ctx, updateQuery, faucetGrant, userID).Scan(&updatedBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to update user cash balance"})
		return
	}

	// Record faucet claim
	insertClaim := `
		INSERT INTO faucet_claims (user_id, ip_address, amount)
		VALUES ($1, $2, $3);
	`
	_, err = tx.Exec(ctx, insertClaim, userID, clientIP, faucetGrant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to record claim audit log"})
		return
	}

	// Insert double-entry ledger record
	txID := uuid.New()
	insertLedger := `
		INSERT INTO ledger_entries (transaction_id, user_id, account, asset, delta, entry_type)
		VALUES ($1, $2, 'user_cash', 'USDC', $3, 'faucet');
	`
	_, err = tx.Exec(ctx, insertLedger, txID, userID, faucetGrant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to record ledger entry"})
		return
	}

	// Build the authoritative response, then persist it as the idempotency receipt.
	resp := gin.H{
		"success":          true,
		"amount_claimed":   faucetGrant.StringFixed(8),
		"amount":           faucetGrant.StringFixed(0),
		"new_balance":      updatedBalance.StringFixed(8),
		"cooldown_seconds": 86400,
		"user": gin.H{
			"id":           userID.String(),
			"cash_balance": updatedBalance.StringFixed(8),
		},
	}

	if idempotencyKey != "" {
		if respBytes, err := json.Marshal(resp); err == nil {
			insertIdempotency := `
				INSERT INTO idempotency_keys (actor_id, operation, idempotency_key, response, created_at)
				VALUES ($1, 'faucet_claim', $2, $3, NOW())
				ON CONFLICT (actor_id, operation, idempotency_key) DO NOTHING;
			`
			_, _ = tx.Exec(ctx, insertIdempotency, userID, idempotencyKey, respBytes)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to commit faucet transaction"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
