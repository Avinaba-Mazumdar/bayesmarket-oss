package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFaucet_NATMultiUserAndAbusePrevention verifies:
// 1. Two distinct users sharing the same NAT IP address can both claim without a 24-hour collision.
// 2. The same user attempting to claim again is rejected with 429 faucet_cooldown.
// 3. Excessive claims from the same IP (including loopback) are capped and rejected with faucet_ip_limit_exceeded.
func TestFaucet_NATMultiUserAndAbusePrevention(t *testing.T) {
	pool, _, router := getTestEnv(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	natIP := "198.51.100." + fmt.Sprintf("%d", (time.Now().UnixNano()%200)+1)

	// Obtain guest token helper
	createGuest := func() (string, string) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/guest", nil)
		req.RemoteAddr = natIP + ":12345"
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var resp rest.GuestAuthResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		return resp.Token, resp.User.ID
	}

	claimFaucet := func(token, ip string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/faucet", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.RemoteAddr = ip + ":54321"
		router.ServeHTTP(w, req)
		return w
	}

	tokenUserA, userAID := createGuest()
	tokenUserB, userBID := createGuest()
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM faucet_claims WHERE user_id IN ($1, $2)", userAID, userBID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userAID, userBID)
	}()

	// 1. User A claims from natIP -> Must SUCCEED (200 OK)
	wA := claimFaucet(tokenUserA, natIP)
	assert.Equal(t, http.StatusOK, wA.Code, "User A should succeed on first claim: %s", wA.Body.String())

	// 2. User B claims from the SAME natIP -> Must also SUCCEED (No NAT-cooldown collision!)
	wB := claimFaucet(tokenUserB, natIP)
	assert.Equal(t, http.StatusOK, wB.Code, "User B should succeed from same NAT IP without 24h lockout: %s", wB.Body.String())

	// 3. User A attempts to claim again immediately -> Must fail with account-level 429 faucet_cooldown
	wA2 := claimFaucet(tokenUserA, natIP)
	assert.Equal(t, http.StatusTooManyRequests, wA2.Code)
	var errA2 map[string]interface{}
	_ = json.Unmarshal(wA2.Body.Bytes(), &errA2)
	assert.Equal(t, "faucet_cooldown", errA2["error"])

	// 4. IP Daily Cap: Verify that when an IP reaches daily cap, it is blocked with faucet_ip_limit_exceeded
	testIP := "203.0.113." + fmt.Sprintf("%d", (time.Now().UnixNano()%200)+1)
	// Seed claims in database to reach the cap for testIP
	dummyUID := uuid.New()
	_, _ = pool.Exec(ctx, "INSERT INTO users (id, is_guest, cash_balance) VALUES ($1, true, 100) ON CONFLICT (id) DO NOTHING", dummyUID)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM faucet_claims WHERE ip_address = $1", testIP)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", dummyUID)
	}()

	// In test mode (isDevOrLocal), ipDailyCap is 100. Insert 100 dummy claims for testIP
	_, err := pool.Exec(ctx, "INSERT INTO faucet_claims (user_id, ip_address, amount, claimed_at) SELECT $1, $2, 100, NOW() FROM generate_series(1, 100)", dummyUID, testIP)
	require.NoError(t, err)

	// Now a fresh user attempting to claim from testIP must be blocked by IP daily cap
	tokenUserC, userCID := createGuest()
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM faucet_claims WHERE user_id = $1", userCID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userCID)
	}()

	wC := claimFaucet(tokenUserC, testIP)
	assert.Equal(t, http.StatusTooManyRequests, wC.Code)
	var errC map[string]interface{}
	_ = json.Unmarshal(wC.Body.Bytes(), &errC)
	assert.Equal(t, "faucet_ip_limit_exceeded", errC["error"])
}
