package main

import (
	"context"
	"fmt"
	"log"
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
		log.Fatal("[FATAL] DATABASE_URL is not configured in environment or .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("==> Verifying Database Connection...")
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Database connection failed: %v\n", err)
	}
	defer pool.Close()
	fmt.Println("  ✓ Connected successfully")

	fmt.Println("==> Verifying and Applying Schema Migrations...")
	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("[FATAL] Schema migration failed: %v\n", err)
	}
	fmt.Println("  ✓ Schema migrations verified and up to date")

	fmt.Println("==> Verifying Table Health & Record Counts...")
	var marketCount, poolCount, userCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM markets").Scan(&marketCount); err != nil {
		log.Fatalf("[FATAL] Failed to query markets table: %v\n", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM liquidity_pools").Scan(&poolCount); err != nil {
		log.Fatalf("[FATAL] Failed to query liquidity_pools table: %v\n", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		log.Fatalf("[FATAL] Failed to query users table: %v\n", err)
	}

	fmt.Printf("  ✓ Markets: %d\n", marketCount)
	fmt.Printf("  ✓ Liquidity Pools: %d\n", poolCount)
	fmt.Printf("  ✓ Users: %d\n", userCount)

	fmt.Println("==> [SUCCESS] All database verification checks passed.")
}
