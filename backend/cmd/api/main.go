package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/database"
	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/bayesmarket/bayesmarket/internal/transport/ws"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	seedOnly := flag.Bool("seed", false, "Run migrations and seed initial markets, then exit")
	migrateOnly := flag.Bool("migrate", false, "Run pending database migrations, then exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Configuration error: %v\n", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if cfg.ServerPort != "" {
		port = cfg.ServerPort
	}

	// Determine migration and admin database connection URL
	migrationURL := ""
	if cfg != nil {
		if cfg.MigrationDatabaseURL != "" && !strings.Contains(cfg.MigrationDatabaseURL, "ep-cool-pool-123456") {
			migrationURL = cfg.MigrationDatabaseURL
		} else if cfg.DatabaseURL != "" && !strings.Contains(cfg.DatabaseURL, "ep-cool-pool-123456") {
			migrationURL = cfg.DatabaseURL
		}
	}

	// 1. Handle dedicated CLI tasks (-migrate and -seed flags)
	if *migrateOnly || *seedOnly {
		if migrationURL == "" {
			log.Fatal("[FATAL] Cannot execute seed/migrate: no valid MIGRATION_DATABASE_URL or DATABASE_URL configured.")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		migPool, err := database.NewPool(ctx, migrationURL)
		if err != nil {
			log.Fatalf("[FATAL] Failed to connect for migration/seeding: %v\n", err)
		}
		defer migPool.Close()

		log.Println("[INFO] Checking and applying database migrations...")
		if err := database.RunMigrations(ctx, migPool); err != nil {
			log.Fatalf("[FATAL] Database migration failed: %v\n", err)
		}

		if *seedOnly {
			log.Println("[INFO] Seeding initial prediction markets...")
			if err := database.SeedInitialMarkets(ctx, migPool); err != nil {
				log.Fatalf("[FATAL] Seeding failed: %v\n", err)
			}
			log.Println("[INFO] Database seeded successfully.")
			return
		}

		log.Println("[INFO] Migrations completed successfully.")
		return
	}

	// 2. If separate privileged MIGRATION_DATABASE_URL is provided, run migrations before booting runtime server
	if cfg != nil && cfg.AutoMigrate && cfg.MigrationDatabaseURL != "" && !strings.Contains(cfg.MigrationDatabaseURL, "ep-cool-pool-123456") {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		migPool, err := database.NewPool(ctx, cfg.MigrationDatabaseURL)
		if err != nil {
			log.Fatalf("[FATAL] Failed to connect to MIGRATION_DATABASE_URL: %v\n", err)
		}
		log.Println("[INFO] Applying database migrations via privileged MIGRATION_DATABASE_URL...")
		if err := database.RunMigrations(ctx, migPool); err != nil {
			migPool.Close()
			log.Fatalf("[FATAL] Database migration via MIGRATION_DATABASE_URL failed: %v\n", err)
		}
		migPool.Close()
		log.Println("[INFO] Privileged database migrations applied successfully.")
	}

	// 3. Initialize runtime connection pool (using least-privilege DATABASE_URL)
	var dbPool *pgxpool.Pool
	if cfg != nil && cfg.DatabaseURL != "" && !strings.Contains(cfg.DatabaseURL, "ep-cool-pool-123456") {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		pool, err := database.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Printf("[WARN] Could not connect to Neon PostgreSQL: %v\n", err)
			log.Println("[INFO] Continuing server startup in detached mode. Configure valid DATABASE_URL in .env to enable persistence.")
		} else {
			dbPool = pool
			defer dbPool.Close()

			if cfg.AutoMigrate && cfg.MigrationDatabaseURL == "" {
				log.Println("[INFO] Checking and applying database migrations via DATABASE_URL...")
				if err := database.RunMigrations(ctx, dbPool); err != nil {
					log.Fatalf("[FATAL] Database migration failed: %v\n", err)
				}
			} else if !cfg.AutoMigrate {
				log.Println("[INFO] Auto-migrations disabled (AUTO_MIGRATE=false). Operating with runtime database user permissions.")
			}

			// Check if markets need seeding (auto-seed if empty or requested via env)
			var marketCount int
			_ = dbPool.QueryRow(ctx, "SELECT COUNT(*) FROM markets").Scan(&marketCount)
			if (marketCount == 0 && os.Getenv("AUTO_SEED") != "false") || os.Getenv("AUTO_SEED") == "true" {
				log.Println("[INFO] Auto-seeding initial prediction markets...")
				if err := database.SeedInitialMarkets(ctx, dbPool); err != nil {
					log.Printf("[WARN] Auto-seeding warning: %v\n", err)
				} else {
					log.Println("[INFO] Initial prediction markets ready.")
				}
			}
		}
	} else {
		log.Println("[INFO] Placeholder or empty DATABASE_URL detected. Server running in disconnected sandbox mode.")
	}

	// Initialize real-time WebSocket broker hub
	wsHub := ws.NewHub()
	go wsHub.Run()

	router := rest.SetupRouter(dbPool, cfg, wsHub)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[INFO] BayesMarket API server listening on :%s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server listen error: %v\n", err)
		}
	}()

	// Graceful shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] Shutting down server gracefully...")
	wsHub.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[FATAL] Server forced to shutdown: %v\n", err)
	}

	log.Println("[INFO] Server exited successfully.")
}
