package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// SeedMarket defines the parameters for prediction markets.
type SeedMarket struct {
	Slug              string
	Title             string
	Description       string
	Category          string
	ImageURL          string
	ResolutionSource  string
	ResolutionDate    time.Time
	ReserveYes        decimal.Decimal
	ReserveNo         decimal.Decimal
	CollateralReserve decimal.Decimal
}

// SeedInitialMarkets is retained for interface compatibility.
// In accordance with system specifications, pre-fed markets are removed;
// all prediction markets are dynamically provisioned by an admin via the Web UI.
func SeedInitialMarkets(ctx context.Context, pool *pgxpool.Pool) error {
	// No-op: all prediction markets must be provisioned dynamically by administrators.
	return nil
}
