package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/database"
	"github.com/bayesmarket/bayesmarket/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	dbURL := testutil.SafeTestDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping live database test: cannot connect to test database: %v", err)
	}

	return pool
}

func TestSchemaConstraints_NegativeUserBalance(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt inserting a user with a negative balance (-50.00 USDC)
	query := `INSERT INTO users (cash_balance) VALUES ($1);`
	_, err := pool.Exec(ctx, query, decimal.NewFromFloat(-50.00))

	if err == nil {
		t.Fatal("Expected check constraint violation 'chk_positive_balance', but insert succeeded")
	}

	if !strings.Contains(err.Error(), "chk_positive_balance") {
		t.Fatalf("Expected error to mention 'chk_positive_balance', got: %v", err)
	}
}

func TestSchemaConstraints_ZeroOrNegativeReserves(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First insert a test market
	marketID := uuid.New()
	testSlug := "test-market-" + marketID.String()[:8]
	_, err := pool.Exec(ctx, `
		INSERT INTO markets (id, slug, title, description, category, resolution_source, resolution_date)
		VALUES ($1, $2, 'Test', 'Test Desc', 'crypto', 'Source', NOW() + INTERVAL '1 day')
	`, marketID, testSlug)
	if err != nil {
		t.Fatalf("Failed to insert test market: %v", err)
	}

	// Clean up after test
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM markets WHERE id = $1", marketID)
	}()

	// Attempt inserting liquidity pool with zero reserve_yes
	_, err = pool.Exec(ctx, `
		INSERT INTO liquidity_pools (market_id, reserve_yes, reserve_no, collateral_reserve, k_invariant)
		VALUES ($1, 0, 1000, 1000, 0)
	`, marketID)

	if err == nil {
		t.Fatal("Expected check constraint violation 'chk_positive_reserves', but insert with reserve_yes=0 succeeded")
	}

	if !strings.Contains(err.Error(), "chk_positive_reserves") {
		t.Fatalf("Expected error to mention 'chk_positive_reserves', got: %v", err)
	}
}

func TestSeedMarkets_Verification(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM markets WHERE status = 'active'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query active markets: %v", err)
	}

	if count < 4 {
		t.Fatalf("Expected at least 4 seeded prediction markets, found %d", count)
	}

	// Verify OpenAI GPT-5 market specific CPMM invariant
	var reserveYes, reserveNo, collateral decimal.Decimal
	err = pool.QueryRow(ctx, `
		SELECT p.reserve_yes, p.reserve_no, p.collateral_reserve
		FROM markets m
		JOIN liquidity_pools p ON p.market_id = m.id
		WHERE m.slug = 'will-openai-release-gpt-5-in-2026'
	`).Scan(&reserveYes, &reserveNo, &collateral)
	if err != nil {
		t.Fatalf("Failed to query GPT-5 market pool: %v", err)
	}

	if !reserveYes.Equal(decimal.NewFromInt(12000)) {
		t.Errorf("Expected reserve_yes=12000, got %s", reserveYes.String())
	}
	if !reserveNo.Equal(decimal.NewFromInt(8000)) {
		t.Errorf("Expected reserve_no=8000, got %s", reserveNo.String())
	}
	if !collateral.Equal(decimal.NewFromInt(20000)) {
		t.Errorf("Expected collateral=20000, got %s", collateral.String())
	}
}
