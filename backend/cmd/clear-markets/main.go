package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Failed to load configuration: %v\n", err)
	}

	if cfg == nil || cfg.DatabaseURL == "" {
		log.Fatal("[FATAL] DATABASE_URL is not configured in .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to database: %v\n", err)
	}
	defer pool.Close()

	var beforeCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM markets").Scan(&beforeCount)
	log.Printf("[INFO] Current market count: %d\n", beforeCount)

	// Clean out market-dependent records safely
	cleanQuery := `
		TRUNCATE TABLE markets, liquidity_pools, user_positions, trades, ledger_entries CASCADE;
	`
	_, err = pool.Exec(ctx, cleanQuery)
	if err != nil {
		// Fallback to explicit DELETE statements if TRUNCATE CASCADE meets permission limits
		log.Printf("[WARN] TRUNCATE CASCADE failed (%v), attempting DELETE cascade...\n", err)
		tx, txErr := pool.Begin(ctx)
		if txErr != nil {
			log.Fatalf("[FATAL] Could not begin transaction: %v\n", txErr)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		commands := []string{
			"DELETE FROM ledger_entries WHERE market_id IS NOT NULL;",
			"DELETE FROM trades;",
			"DELETE FROM user_positions;",
			"DELETE FROM liquidity_pools;",
			"DELETE FROM markets;",
		}
		for _, q := range commands {
			if _, execErr := tx.Exec(ctx, q); execErr != nil {
				log.Fatalf("[FATAL] Failed executing '%s': %v\n", q, execErr)
			}
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			log.Fatalf("[FATAL] Failed to commit delete transaction: %v\n", commitErr)
		}
	}

	var afterCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM markets").Scan(&afterCount)

	fmt.Println("-------------------------------------------------------------------")
	fmt.Printf("[SUCCESS] All prediction markets and related trades/pools deleted!\n")
	fmt.Printf("Markets count: %d -> %d\n", beforeCount, afterCount)
	fmt.Println("Make sure AUTO_SEED=false in .env so default demo markets are not re-seeded.")
	fmt.Println("-------------------------------------------------------------------")
	os.Exit(0)
}
