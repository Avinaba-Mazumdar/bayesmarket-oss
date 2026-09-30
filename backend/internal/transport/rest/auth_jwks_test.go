package rest_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoogleIDToken_JWKSVerification validates RS256 signature verification, issuer, audience, and email_verified checks.
func TestGoogleIDToken_JWKSVerification(t *testing.T) {
	// 1. Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kid := "test-google-kid-1"
	nStr := base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes())
	eBytes := big.NewInt(int64(privateKey.E)).Bytes()
	eStr := base64.RawURLEncoding.EncodeToString(eBytes)

	// 2. Mock Google JWKS HTTP Server
	jwksHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"kid": kid,
					"n":   nStr,
					"e":   eStr,
				},
			},
		})
	})
	jwksServer := httptest.NewServer(jwksHandler)
	defer jwksServer.Close()

	// Override JWKS URL to test server
	oldURL := rest.GoogleJWKSURL
	rest.GoogleJWKSURL = jwksServer.URL
	defer func() { rest.GoogleJWKSURL = oldURL }()

	clientID := "test-client-id.apps.googleusercontent.com"
	cfg := &config.Config{
		GoogleClientID: clientID,
		Environment:    "production", // Enforce production mode (no mock bypasses)
	}
	handler := rest.NewAuthHandler(nil, cfg)

	createToken := func(claims jwt.MapClaims, signingKey *rsa.PrivateKey, keyID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = keyID
		signedStr, err := token.SignedString(signingKey)
		require.NoError(t, err)
		return signedStr
	}

	ctx := context.Background()

	// Scenario A: Valid Google ID Token with email_verified = true
	t.Run("Valid token succeeds", func(t *testing.T) {
		tokenStr := createToken(jwt.MapClaims{
			"iss":            "https://accounts.google.com",
			"aud":            clientID,
			"sub":            "google-sub-12345",
			"email":          "alice@example.com",
			"email_verified": true,
			"name":           "Alice Trader",
			"exp":            time.Now().Add(1 * time.Hour).Unix(),
		}, privateKey, kid)

		// Call HandleGoogleAuthVerify via HTTP or check token verification
		tokenInfo, err := rest.ExportVerifyGoogleIDToken(handler, ctx, tokenStr)
		require.NoError(t, err)
		assert.Equal(t, "google-sub-12345", tokenInfo.Sub)
		assert.Equal(t, "alice@example.com", tokenInfo.Email)
		assert.Equal(t, "Alice Trader", tokenInfo.Name)
	})

	// Scenario B: Forged token signed with different RSA key
	t.Run("Forged signature rejected", func(t *testing.T) {
		attackerKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		tokenStr := createToken(jwt.MapClaims{
			"iss":            "https://accounts.google.com",
			"aud":            clientID,
			"sub":            "victim-sub-99999",
			"email":          "victim@example.com",
			"email_verified": true,
			"exp":            time.Now().Add(1 * time.Hour).Unix(),
		}, attackerKey, kid) // Attacker signs with own key but claims kid

		_, err = rest.ExportVerifyGoogleIDToken(handler, ctx, tokenStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cryptographic token verification failed")
	})

	// Scenario C: Unverified email rejected (Account takeover vector blocked)
	t.Run("Unverified email rejected", func(t *testing.T) {
		tokenStr := createToken(jwt.MapClaims{
			"iss":            "https://accounts.google.com",
			"aud":            clientID,
			"sub":            "unverified-sub",
			"email":          "victim@example.com",
			"email_verified": false, // Not verified!
			"exp":            time.Now().Add(1 * time.Hour).Unix(),
		}, privateKey, kid)

		_, err := rest.ExportVerifyGoogleIDToken(handler, ctx, tokenStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "google email is not verified")
	})

	// Scenario D: Invalid Issuer rejected
	t.Run("Invalid issuer rejected", func(t *testing.T) {
		tokenStr := createToken(jwt.MapClaims{
			"iss":            "https://attacker-idp.example.com",
			"aud":            clientID,
			"sub":            "attacker-sub",
			"email":          "victim@example.com",
			"email_verified": true,
			"exp":            time.Now().Add(1 * time.Hour).Unix(),
		}, privateKey, kid)

		_, err := rest.ExportVerifyGoogleIDToken(handler, ctx, tokenStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token issuer")
	})

	// Scenario E: Audience mismatch rejected
	t.Run("Audience mismatch rejected", func(t *testing.T) {
		tokenStr := createToken(jwt.MapClaims{
			"iss":            "https://accounts.google.com",
			"aud":            "other-app-client-id.apps.googleusercontent.com",
			"sub":            "victim-sub",
			"email":          "victim@example.com",
			"email_verified": true,
			"exp":            time.Now().Add(1 * time.Hour).Unix(),
		}, privateKey, kid)

		_, err := rest.ExportVerifyGoogleIDToken(handler, ctx, tokenStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token audience mismatch")
	})
}
