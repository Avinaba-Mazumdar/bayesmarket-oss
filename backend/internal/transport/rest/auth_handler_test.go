package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/database"
	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthHandler_GuestAndGoogleOAuth(t *testing.T) {
	cfg, err := config.Load()
	if err != nil || cfg.DatabaseURL == "" {
		t.Skip("Skipping AuthHandler tests: DATABASE_URL not set")
	}
	cfg.DisableRateLimits = true

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	require.NoError(t, err)
	defer pool.Close()

	// Apply migrations
	err = database.RunMigrations(ctx, pool)
	require.NoError(t, err)

	router := rest.SetupRouter(pool, cfg, nil)

	var guestToken string
	var guestUserID string

	// 1. Test Guest Auth
	t.Run("HandleGuestAuth generates a guest user with $1,000", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var res rest.AuthResponse
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Token)
		assert.NotEmpty(t, res.User.ID)
		assert.True(t, res.User.IsGuest)
		assert.Equal(t, "guest", res.User.AuthProvider)
		assert.Equal(t, "1000.00000000", res.User.CashBalance)

		guestToken = res.Token
		guestUserID = res.User.ID
	})

	// 2. Test GetMe with Guest Token
	t.Run("HandleGetMe returns authenticated guest profile", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+guestToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var user rest.UserResponse
		err := json.Unmarshal(w.Body.Bytes(), &user)
		require.NoError(t, err)
		assert.Equal(t, guestUserID, user.ID)
		assert.True(t, user.IsGuest)
		assert.Equal(t, "guest", user.AuthProvider)
	})

	// 3. Test Google Auth Verify (Simulation / Dev mode)
	var googleToken string
	uniqueEmail := "alice." + uuid.New().String()[:8] + "@bayesmarket.com"
	t.Run("HandleGoogleAuthVerify registers verified Google account", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"id_token": "dev-token-xyz-123",
			"email":    uniqueEmail,
			"name":     "Alice Quant",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/google/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var res rest.AuthResponse
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Token)
		assert.False(t, res.User.IsGuest)
		assert.Equal(t, "google", res.User.AuthProvider)
		require.NotNil(t, res.User.Email)
		assert.Equal(t, uniqueEmail, *res.User.Email)
		require.NotNil(t, res.User.Name)
		assert.Equal(t, "Alice Quant", *res.User.Name)

		googleToken = res.Token
	})

	// 4. Test GetMe with Google Token
	t.Run("HandleGetMe returns Google profile with registered details", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+googleToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var user rest.UserResponse
		err := json.Unmarshal(w.Body.Bytes(), &user)
		require.NoError(t, err)
		assert.False(t, user.IsGuest)
		assert.Equal(t, "google", user.AuthProvider)
		require.NotNil(t, user.Email)
		assert.Equal(t, uniqueEmail, *user.Email)
	})

	// 5. Test prevention of account takeover: different Google identity with existing email is rejected
	t.Run("HandleGoogleAuthVerify rejects hijacking account with existing email", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"id_token": "dev-token-attacker-456",
			"email":    uniqueEmail,
			"name":     "Attacker",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/google/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	// 6. Test Google Auth URL
	t.Run("HandleGoogleAuthURL returns OAuth endpoint or simulated status", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/google/url", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// 7. Test Google Callback state validation
	t.Run("HandleGoogleAuthCallback requires valid state parameter", func(t *testing.T) {
		// Missing state
		body, _ := json.Marshal(map[string]string{
			"code": "mock-auth-code-123",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/google/callback", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Valid state in dev mode
		validBody, _ := json.Marshal(map[string]string{
			"code":  "mock-auth-code-123",
			"state": "mock-state",
		})
		req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/google/callback", bytes.NewBuffer(validBody))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)
	})
}
