package testutil

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/bayesmarket/bayesmarket/internal/config"
)

// SafeTestDatabaseURL resolves a safe database connection string for tests.
// It will NEVER load .env or connect to a non-local database host.
func SafeTestDatabaseURL(t *testing.T) string {
	t.Helper()

	// Prioritize explicit test database env var
	dbURL := os.Getenv("BAYESMARKET_TEST_DATABASE_URL")
	if dbURL == "" {
		// Fall back to DATABASE_URL only if present in the process environment
		dbURL = os.Getenv("DATABASE_URL")
	}

	if dbURL == "" || strings.Contains(dbURL, "ep-cool-pool-123456") {
		t.Skip("Skipping live database test: neither BAYESMARKET_TEST_DATABASE_URL nor test DATABASE_URL is set")
	}

	parsed, err := url.Parse(dbURL)
	if err != nil {
		t.Skipf("Skipping live database test: invalid database URL: %v", err)
	}

	host := parsed.Hostname()
	switch host {
	case "localhost", "127.0.0.1", "::1", "postgres":
		return dbURL
	default:
		t.Skipf("REFUSING to run live database tests against remote host %q (protecting production database). To run tests against a local database, set BAYESMARKET_TEST_DATABASE_URL with a localhost connection.", host)
		return ""
	}
}

// TestConfig returns an explicitly constructed test config without loading .env.
func TestConfig(dbURL string) *config.Config {
	return &config.Config{
		DatabaseURL:       dbURL,
		JWTSecret:         "bayesmarket-development-hmac-sha256-default-secret-key-32b",
		DisableRateLimits: true,
		Environment:       "test",
	}
}
