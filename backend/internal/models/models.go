package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// MarketStatus defines the operational lifecycle state of a prediction market.
type MarketStatus string

const (
	MarketStatusDraft     MarketStatus = "draft"
	MarketStatusActive    MarketStatus = "active"
	MarketStatusSuspended MarketStatus = "suspended"
	MarketStatusLocked    MarketStatus = "locked"
	MarketStatusResolved  MarketStatus = "resolved"
	MarketStatusSettled   MarketStatus = "settled"
)

// Outcome represents binary prediction outcomes.
type Outcome string

const (
	OutcomeYes Outcome = "YES"
	OutcomeNo  Outcome = "NO"
)

// TradeType represents an order action.
type TradeType string

const (
	TradeTypeBuy  TradeType = "BUY"
	TradeTypeSell TradeType = "SELL"
)

// User represents an account or ephemeral guest trader.
type User struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	IsGuest     bool            `json:"is_guest" db:"is_guest"`
	IsAdmin     bool            `json:"is_admin" db:"is_admin"`
	CashBalance decimal.Decimal `json:"cash_balance" db:"cash_balance"`
	IPAddress   *string         `json:"ip_address,omitempty" db:"ip_address"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	LastActive  time.Time       `json:"last_active" db:"last_active"`
}

// Market represents a binary prediction market contract.
type Market struct {
	ID               uuid.UUID    `json:"id" db:"id"`
	Slug             string       `json:"slug" db:"slug"`
	Title            string       `json:"title" db:"title"`
	Description      string       `json:"description" db:"description"`
	Category         string       `json:"category" db:"category"`
	ImageURL         *string      `json:"image_url,omitempty" db:"image_url"`
	ResolutionSource string       `json:"resolution_source" db:"resolution_source"`
	ResolutionDate   time.Time    `json:"resolution_date" db:"resolution_date"`
	Status           MarketStatus `json:"status" db:"status"`
	WinningOutcome   *Outcome     `json:"winning_outcome,omitempty" db:"winning_outcome"`
	CreatedAt        time.Time    `json:"created_at" db:"created_at"`
}

// LiquidityPool holds virtual inventory reserves and total collateral backing.
// Preserves invariant k = ReserveYes * ReserveNo under CPMM.
type LiquidityPool struct {
	MarketID          uuid.UUID       `json:"market_id" db:"market_id"`
	ReserveYes        decimal.Decimal `json:"reserve_yes" db:"reserve_yes"`
	ReserveNo         decimal.Decimal `json:"reserve_no" db:"reserve_no"`
	CollateralReserve decimal.Decimal `json:"collateral_reserve" db:"collateral_reserve"`
	KInvariant        decimal.Decimal `json:"k_invariant" db:"k_invariant"`
	TotalVolumeUSDC   decimal.Decimal `json:"total_volume_usdc" db:"total_volume_usdc"`
	LockVersion       int             `json:"lock_version" db:"lock_version"`
	UpdatedAt         time.Time       `json:"updated_at" db:"updated_at"`
}

// UserPosition tracks a trader's outcome shares and average acquisition cost.
type UserPosition struct {
	ID                uuid.UUID       `json:"id" db:"id"`
	UserID            uuid.UUID       `json:"user_id" db:"user_id"`
	MarketID          uuid.UUID       `json:"market_id" db:"market_id"`
	Outcome           Outcome         `json:"outcome" db:"outcome"`
	SharesOwned       decimal.Decimal `json:"shares_owned" db:"shares_owned"`
	AvgBuyPrice       decimal.Decimal `json:"avg_buy_price" db:"avg_buy_price"`
	TotalInvestedUSDC decimal.Decimal `json:"total_invested_usdc" db:"total_invested_usdc"`
	CreatedAt         time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at" db:"updated_at"`
}

// Trade represents an executed transaction audit record.
type Trade struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	MarketID       uuid.UUID       `json:"market_id" db:"market_id"`
	UserID         uuid.UUID       `json:"user_id" db:"user_id"`
	IdempotencyKey string          `json:"idempotency_key" db:"idempotency_key"`
	TradeType      TradeType       `json:"trade_type" db:"trade_type"`
	Outcome        Outcome         `json:"outcome" db:"outcome"`
	AmountUSDC     decimal.Decimal `json:"amount_usdc" db:"amount_usdc"`
	SharesFilled   decimal.Decimal `json:"shares_filled" db:"shares_filled"`
	ExecutionPrice decimal.Decimal `json:"execution_price" db:"execution_price"`
	PriceImpactPct decimal.Decimal `json:"price_impact_pct" db:"price_impact_pct"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

// LedgerEntry represents an append-only double-entry financial audit record.
type LedgerEntry struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	TransactionID uuid.UUID       `json:"transaction_id" db:"transaction_id"`
	UserID        *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	MarketID      *uuid.UUID      `json:"market_id,omitempty" db:"market_id"`
	AccountType   string          `json:"account_type" db:"account_type"` // 'user_cash', 'pool_collateral', 'user_shares_yes', 'user_shares_no'
	Amount        decimal.Decimal `json:"amount" db:"amount"`             // Signed delta
	BalanceAfter  decimal.Decimal `json:"balance_after" db:"balance_after"`
	Description   string          `json:"description" db:"description"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}
