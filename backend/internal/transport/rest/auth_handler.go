package rest

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ErrAccountConflict is returned when attempting to associate an identity with an email that is already registered to a different user.
var ErrAccountConflict = errors.New("an account with this email already exists; please sign in with your primary credential to link identities")

// AuthHandler handles user registration, guest sessions, Google OAuth, and profile management.
type AuthHandler struct {
	pool               *pgxpool.Pool
	jwtSecret          string
	googleClientID     string
	googleClientSecret string
	googleRedirectURI  string
	httpClient         *http.Client
	isDevOrLocal       bool

	jwksMu     sync.RWMutex
	jwksKeys   map[string]*rsa.PublicKey
	jwksExpiry time.Time
}

// NewAuthHandler constructs an AuthHandler with configuration.
func NewAuthHandler(pool *pgxpool.Pool, cfg *config.Config) *AuthHandler {
	googleClientID := ""
	googleClientSecret := ""
	googleRedirectURI := "http://localhost:4200/auth/callback"
	jwtSecret := "bayesmarket-development-hmac-sha256-default-secret-key-32b"
	isDevOrLocal := false

	if cfg != nil {
		if cfg.JWTSecret != "" {
			jwtSecret = cfg.JWTSecret
		}
		googleClientID = cfg.GoogleClientID
		googleClientSecret = cfg.GoogleClientSecret
		if cfg.GoogleRedirectURI != "" {
			googleRedirectURI = cfg.GoogleRedirectURI
		}
		isDevOrLocal = cfg.IsDevOrLocal()
	}

	return &AuthHandler{
		pool:               pool,
		jwtSecret:          jwtSecret,
		googleClientID:     googleClientID,
		googleClientSecret: googleClientSecret,
		googleRedirectURI:  googleRedirectURI,
		isDevOrLocal:       isDevOrLocal,
		jwksKeys:           make(map[string]*rsa.PublicKey),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// UserResponse represents the public user JSON structure with canonical profile fields.
type UserResponse struct {
	ID           string  `json:"id"`
	Email        *string `json:"email,omitempty"`
	Name         *string `json:"name,omitempty"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
	IsGuest      bool    `json:"is_guest"`
	IsAdmin      bool    `json:"is_admin"`
	AuthProvider string  `json:"auth_provider"`
	CashBalance  string  `json:"cash_balance"`
	CreatedAt    string  `json:"created_at"`
}

// AuthResponse holds the minted JWT token and user profile.
type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// GuestAuthResponse alias for backwards compatibility.
type GuestAuthResponse = AuthResponse

// GoogleVerifyRequest payload containing Google Identity Services ID token.
type GoogleVerifyRequest struct {
	IDToken string `json:"id_token" binding:"required"`
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
}

// GoogleCallbackRequest payload containing authorization code and state.
type GoogleCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state"`
}

// GoogleTokenInfo represents Google's public tokeninfo response.
type GoogleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
	Error         string `json:"error_description"`
}

// HandleGuestAuth provisions a new guest account seeded with $1,000.00 virtual USDC.
//
// POST /api/v1/auth/guest
func (h *AuthHandler) HandleGuestAuth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	initialBalance := decimal.NewFromInt(1000)

	var (
		userID       uuid.UUID
		isGuest      bool
		authProvider string
		cashBalance  decimal.Decimal
		createdAt    time.Time
	)

	if h.pool == nil {
		userID = uuid.New()
		isGuest = true
		authProvider = "guest"
		cashBalance = initialBalance
		createdAt = time.Now().UTC()
	} else {
		// Enforce daily cap on guest account creations per IP to prevent sybil wallet abuse
		dailyCap := 5
		if h.isDevOrLocal {
			dailyCap = 100
		}

		// Atomic conditional insert eliminates TOCTOU races under concurrent requests
		query := `
			INSERT INTO users (is_guest, cash_balance, auth_provider, ip_address)
			SELECT true, $1, 'guest', $2::varchar
			WHERE (
				SELECT COUNT(*) FROM users
				WHERE is_guest = true AND ip_address = $2::varchar AND created_at > NOW() - INTERVAL '24 hours'
			) < $3
			RETURNING id, is_guest, auth_provider, cash_balance, created_at;
		`
		err := h.pool.QueryRow(ctx, query, initialBalance, clientIP, dailyCap).Scan(
			&userID, &isGuest, &authProvider, &cashBalance, &createdAt,
		)
		if err != nil {
			log.Printf("[HandleGuestAuth] Database insert error: %v (clientIP: %s)\n", err, clientIP)
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "guest_limit_exceeded",
					"message": "Maximum guest accounts created for this IP address today. Please sign in or try again later.",
				})
				return
			}
			resp := gin.H{
				"error":   "database_error",
				"message": "Failed to provision guest user session",
			}
			if h.isDevOrLocal {
				resp["detail"] = err.Error()
			}
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
	}

	tokenString, err := h.generateJWT(userID.String(), isGuest, false, "", "", "", authProvider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "auth_error",
			"message": "Failed to sign authentication token",
		})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		Token: tokenString,
		User: UserResponse{
			ID:           userID.String(),
			IsGuest:      isGuest,
			IsAdmin:      false,
			AuthProvider: authProvider,
			CashBalance:  cashBalance.StringFixed(8),
			CreatedAt:    createdAt.Format(time.RFC3339),
		},
	})
}

// HandleGoogleAuthVerify verifies a Google ID token and creates or logs in the user.
// Supports upgrading an existing guest session if provided in the Authorization header.
//
// POST /api/v1/auth/google/verify
func (h *AuthHandler) HandleGoogleAuthVerify(c *gin.Context) {
	var req GoogleVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "id_token is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	var (
		googleID  string
		email     string
		name      string
		avatarURL string
	)

	// Check if this is local dev simulation mode or mock token
	isMockToken := strings.HasPrefix(req.IDToken, "mock-") || strings.HasPrefix(req.IDToken, "dev-")
	if isMockToken {
		if !h.isDevOrLocal {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "dev_mode_disabled",
				"message": "Dev mock authentication tokens are only allowed when APP_ENV=local or dev",
			})
			return
		}
		googleID = "google-mock-" + fmt.Sprintf("%x", sha256.Sum256([]byte(req.IDToken)))[:16]
		email = "trader@bayesmarket.com"
		if req.Email != "" {
			email = req.Email
		}
		name = "Institutional Trader"
		if req.Name != "" {
			name = req.Name
		}
		avatarURL = "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=128&auto=format&fit=crop&q=80"
	} else if h.googleClientID == "" {
		if !h.isDevOrLocal {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "oauth_not_configured",
				"message": "Google OAuth is not configured in this environment",
			})
			return
		}
		googleID = "google-mock-" + fmt.Sprintf("%x", sha256.Sum256([]byte(req.IDToken)))[:16]
		email = "trader@bayesmarket.com"
		if req.Email != "" {
			email = req.Email
		}
		name = "Institutional Trader"
		if req.Name != "" {
			name = req.Name
		}
		avatarURL = "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=128&auto=format&fit=crop&q=80"
	} else {
		// Verify token with Google TokenInfo endpoint
		tokenInfo, err := h.verifyGoogleIDToken(ctx, req.IDToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Failed to verify Google identity token",
			})
			return
		}

		if h.googleClientID != "" && tokenInfo.Aud != h.googleClientID {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_audience",
				"message": "Google token audience mismatch",
			})
			return
		}

		googleID = tokenInfo.Sub
		email = tokenInfo.Email
		name = tokenInfo.Name
		avatarURL = tokenInfo.Picture
	}

	user, err := h.upsertGoogleUser(ctx, c, googleID, email, name, avatarURL)
	if err != nil {
		log.Printf("[HandleGoogleAuthVerify] upsertGoogleUser error: %v\n", err)
		if errors.Is(err, ErrAccountConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "account_conflict",
				"message": err.Error(),
			})
			return
		}
		resp := gin.H{
			"error":   "auth_error",
			"message": "Failed to authenticate user profile",
		}
		if h.isDevOrLocal {
			resp["detail"] = err.Error()
		}
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	tokenString, err := h.generateJWT(user.ID, false, user.IsAdmin, email, name, avatarURL, "google")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "auth_error",
			"message": "Failed to sign authentication token",
		})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token: tokenString,
		User:  *user,
	})
}

// HandleGoogleAuthURL returns the OAuth 2.0 authorization URL for redirect flows.
//
// GET /api/v1/auth/google/url
func (h *AuthHandler) HandleGoogleAuthURL(c *gin.Context) {
	if h.googleClientID == "" {
		if !h.isDevOrLocal {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":     "oauth_not_configured",
				"message":   "Google OAuth is not configured in this environment",
				"simulated": false,
				"url":       "",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"url":       "",
			"simulated": true,
			"message":   "Google Client ID is not configured. Development simulation mode is available via /api/v1/auth/google/verify.",
		})
		return
	}

	state := h.generateOAuthState()
	redirectURI := url.QueryEscape(h.googleRedirectURI)
	scope := url.QueryEscape("openid email profile")

	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s&access_type=offline&prompt=select_account",
		h.googleClientID,
		redirectURI,
		scope,
		state,
	)

	c.JSON(http.StatusOK, gin.H{
		"url":       authURL,
		"state":     state,
		"simulated": false,
	})
}

// generateOAuthState produces a cryptographically HMAC-SHA256 signed, timestamped state token.
func (h *AuthHandler) generateOAuthState() string {
	timestamp := time.Now().Unix()
	nonce := uuid.New().String()
	data := fmt.Sprintf("%s:%d", nonce, timestamp)
	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(data))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", base64.RawURLEncoding.EncodeToString([]byte(data)), sig)
}

// validateOAuthState validates that the provided state token was signed with jwtSecret and has not expired.
func (h *AuthHandler) validateOAuthState(state string) bool {
	if state == "" {
		return false
	}
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return false
	}
	dataBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write(dataBytes)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return false
	}
	subParts := strings.Split(string(dataBytes), ":")
	if len(subParts) != 2 {
		return false
	}
	ts, err := strconv.ParseInt(subParts[1], 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	// State is valid for 15 minutes (900 seconds), allow 60 seconds future clock skew
	if (now-ts) > 900 || (ts-now) > 60 {
		return false
	}
	return true
}

// HandleGoogleAuthCallback exchanges an OAuth 2.0 authorization code for tokens and authenticates the user.
//
// POST /api/v1/auth/google/callback
func (h *AuthHandler) HandleGoogleAuthCallback(c *gin.Context) {
	var req GoogleCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "code is required",
		})
		return
	}

	// Validate OAuth state parameter to prevent cross-site request forgery
	if req.State == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_state",
			"message": "OAuth state parameter is required",
		})
		return
	}

	isMockCode := strings.HasPrefix(req.Code, "mock-")
	if !h.validateOAuthState(req.State) {
		if !(h.isDevOrLocal && (req.State == "mock-state" || isMockCode)) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_state",
				"message": "OAuth state is invalid or has expired",
			})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// If dev mode or mock code, bypass with mock user (only allowed in local/dev)
	if h.googleClientID == "" || isMockCode {
		if !h.isDevOrLocal {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "dev_mode_disabled",
				"message": "Dev mock authentication codes are only allowed when APP_ENV=local or dev",
			})
			return
		}
		mockGoogleID := "google-mock-" + fmt.Sprintf("%x", sha256.Sum256([]byte(req.Code)))[:16]
		user, err := h.upsertGoogleUser(ctx, c, mockGoogleID, "oauth.trader@bayesmarket.com", "OAuth Trader", "")
		if err != nil {
			log.Printf("[HandleGoogleAuthCallback] mock upsertGoogleUser error: %v\n", err)
			if errors.Is(err, ErrAccountConflict) {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "account_conflict",
					"message": err.Error(),
				})
				return
			}
			resp := gin.H{"error": "database_error", "message": "Failed to create user session"}
			if h.isDevOrLocal {
				resp["detail"] = err.Error()
			}
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
		tokenString, _ := h.generateJWT(user.ID, false, user.IsAdmin, "oauth.trader@bayesmarket.com", "OAuth Trader", "", "google")
		c.JSON(http.StatusOK, AuthResponse{Token: tokenString, User: *user})
		return
	}

	// Exchange code for token
	data := url.Values{}
	data.Set("code", req.Code)
	data.Set("client_id", h.googleClientID)
	data.Set("client_secret", h.googleClientSecret)
	data.Set("redirect_uri", h.googleRedirectURI)
	data.Set("grant_type", "authorization_code")

	resp, err := h.httpClient.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "oauth_error",
			"message": "Failed to communicate with Google token endpoint",
		})
		return
	}
	defer resp.Body.Close()

	var tokenRes struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil || tokenRes.IDToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "oauth_error",
			"message": "Failed to exchange authorization code for Google token",
		})
		return
	}

	// Verify the received ID token
	tokenInfo, err := h.verifyGoogleIDToken(ctx, tokenRes.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "Failed to verify exchanged Google token",
		})
		return
	}

	user, err := h.upsertGoogleUser(ctx, c, tokenInfo.Sub, tokenInfo.Email, tokenInfo.Name, tokenInfo.Picture)
	if err != nil {
		log.Printf("[Auth GoogleLogin] user profile upsert error: %v", err)
		if errors.Is(err, ErrAccountConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "account_conflict",
				"message": err.Error(),
			})
			return
		}
		resp := gin.H{
			"error":   "database_error",
			"message": "Failed to create or update user profile",
		}
		if h.isDevOrLocal {
			resp["detail"] = err.Error()
		}
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	tokenString, err := h.generateJWT(user.ID, false, user.IsAdmin, tokenInfo.Email, tokenInfo.Name, tokenInfo.Picture, "google")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "auth_error",
			"message": "Failed to sign authentication token",
		})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token: tokenString,
		User:  *user,
	})
}

// HandleGetMe returns the authenticated user's profile and current balances.
//
// GET /api/v1/auth/me
func (h *AuthHandler) HandleGetMe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "Authentication required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var (
		id           uuid.UUID
		isGuest      bool
		isAdmin      bool
		authProvider string
		email        *string
		name         *string
		avatarURL    *string
		cashBalance  decimal.Decimal
		createdAt    time.Time
	)

	if h.pool == nil {
		nameStr := "Guest Trader"
		c.JSON(http.StatusOK, UserResponse{
			ID:           userID.String(),
			Name:         &nameStr,
			AuthProvider: "guest",
			IsGuest:      true,
			IsAdmin:      false,
			CashBalance:  "1000.00000000",
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	query := `
		SELECT id, is_guest, auth_provider, email, name, avatar_url, cash_balance, created_at, is_admin
		FROM users
		WHERE id = $1;
	`
	err := h.pool.QueryRow(ctx, query, userID).Scan(
		&id, &isGuest, &authProvider, &email, &name, &avatarURL, &cashBalance, &createdAt, &isAdmin,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "user_not_found",
				"message": "User profile not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to retrieve user profile",
		})
		return
	}

	c.JSON(http.StatusOK, UserResponse{
		ID:           id.String(),
		Email:        email,
		Name:         name,
		AvatarURL:    avatarURL,
		IsGuest:      isGuest,
		IsAdmin:      isAdmin,
		AuthProvider: authProvider,
		CashBalance:  cashBalance.StringFixed(8),
		CreatedAt:    createdAt.Format(time.RFC3339),
	})
}

// GoogleJWKSURL is the Google public certificates endpoint for RS256 ID token verification.
var GoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

// getGooglePublicKey fetches and caches Google's public RSA keys from its official JWKS endpoint.
func (h *AuthHandler) getGooglePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	h.jwksMu.RLock()
	if time.Now().Before(h.jwksExpiry) {
		if key, ok := h.jwksKeys[kid]; ok {
			h.jwksMu.RUnlock()
			return key, nil
		}
	}
	h.jwksMu.RUnlock()

	h.jwksMu.Lock()
	defer h.jwksMu.Unlock()

	// Recheck cache under write lock
	if time.Now().Before(h.jwksExpiry) {
		if key, ok := h.jwksKeys[kid]; ok {
			return key, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GoogleJWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error fetching Google JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to parse Google JWKS JSON: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.N, "="))
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.E, "="))
		if err != nil {
			continue
		}
		var eInt int
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}
		newKeys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: eInt,
		}
	}

	h.jwksKeys = newKeys

	// Parse max-age from Cache-Control header, default to 1 hour
	ttl := 1 * time.Hour
	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl != "" {
		for _, part := range strings.Split(cacheControl, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "max-age=") {
				if secs, err := strconv.Atoi(strings.TrimPrefix(part, "max-age=")); err == nil && secs > 0 {
					ttl = time.Duration(secs) * time.Second
				}
			}
		}
	}
	h.jwksExpiry = time.Now().Add(ttl)

	key, ok := h.jwksKeys[kid]
	if !ok {
		return nil, fmt.Errorf("key id %q not found in Google JWKS", kid)
	}
	return key, nil
}

// verifyGoogleIDToken cryptographically verifies a Google ID token against Google's JWKS public keys,
// strictly checking RS256 signature, issuer, audience, and email_verified claim.
func (h *AuthHandler) verifyGoogleIDToken(ctx context.Context, idToken string) (*GoogleTokenInfo, error) {
	if h.isDevOrLocal && (strings.HasPrefix(idToken, "mock-") || strings.HasPrefix(idToken, "dev-")) {
		return &GoogleTokenInfo{
			Sub:           "google-mock-" + fmt.Sprintf("%x", sha256.Sum256([]byte(idToken)))[:16],
			Email:         "trader@bayesmarket.com",
			EmailVerified: "true",
			Name:          "Institutional Trader",
			Aud:           h.googleClientID,
		}, nil
	}

	parsedToken, err := jwt.Parse(idToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, errors.New("missing kid header in Google ID token")
		}
		return h.getGooglePublicKey(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("cryptographic token verification failed: %w", err)
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Validate Issuer
	iss, _ := claims["iss"].(string)
	if iss != "accounts.google.com" && iss != "https://accounts.google.com" {
		return nil, fmt.Errorf("invalid token issuer: %q", iss)
	}

	// Validate Audience if googleClientID is configured
	aud, _ := claims["aud"].(string)
	if h.googleClientID != "" && aud != h.googleClientID {
		return nil, fmt.Errorf("token audience mismatch: got %q, expected %q", aud, h.googleClientID)
	}

	// Validate Subject (Google User ID)
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("missing sub claim in Google ID token")
	}

	// Validate Email
	email, _ := claims["email"].(string)
	if email == "" {
		return nil, errors.New("missing email claim in Google ID token")
	}

	// Validate Email Verified (CRITICAL: prevent account takeover via unverified email)
	var emailVerified bool
	switch ev := claims["email_verified"].(type) {
	case bool:
		emailVerified = ev
	case string:
		emailVerified = (strings.ToLower(ev) == "true")
	}
	if !emailVerified {
		return nil, errors.New("google email is not verified; rejected to prevent account takeover")
	}

	name, _ := claims["name"].(string)
	picture, _ := claims["picture"].(string)

	return &GoogleTokenInfo{
		Sub:           sub,
		Email:         email,
		EmailVerified: "true",
		Name:          name,
		Picture:       picture,
		Aud:           aud,
	}, nil
}

// upsertGoogleUser finds an existing user by Google ID or email, upgrades a guest session if applicable,
// or inserts a newly registered Google user.
func (h *AuthHandler) upsertGoogleUser(
	ctx context.Context,
	c *gin.Context,
	googleID string,
	email string,
	name string,
	avatarURL string,
) (*UserResponse, error) {
	// 1. Check if user already exists by google_id or email
	var (
		existingID          uuid.UUID
		existingIsGuest     bool
		existingIsAdmin     bool
		existingProvider    string
		existingCashBalance decimal.Decimal
		existingCreatedAt   time.Time
		existingEmail       *string
		existingName        *string
		existingAvatar      *string
	)

	if h.pool == nil {
		mockID := uuid.New().String()
		return &UserResponse{
			ID:           mockID,
			Email:        &email,
			Name:         &name,
			AvatarURL:    &avatarURL,
			IsGuest:      false,
			IsAdmin:      false,
			AuthProvider: "google",
			CashBalance:  "1000.00000000",
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	queryFind := `
		SELECT id, is_guest, auth_provider, email, name, avatar_url, cash_balance, created_at, is_admin
		FROM users
		WHERE google_id = $1
		LIMIT 1;
	`
	err := h.pool.QueryRow(ctx, queryFind, googleID).Scan(
		&existingID, &existingIsGuest, &existingProvider, &existingEmail, &existingName, &existingAvatar, &existingCashBalance, &existingCreatedAt, &existingIsAdmin,
	)

	if err == nil {
		// User exists by stable Google sub identifier - update latest name/avatar/email and timestamp
		updateQuery := `
			UPDATE users
			SET name = COALESCE(NULLIF($1, ''), name),
			    avatar_url = COALESCE(NULLIF($2, ''), avatar_url),
			    email = COALESCE(NULLIF($3, ''), email),
			    last_active = NOW()
			WHERE id = $4
			RETURNING name, avatar_url, email;
		`
		_ = h.pool.QueryRow(ctx, updateQuery, name, avatarURL, email, existingID).Scan(&existingName, &existingAvatar, &existingEmail)

		return &UserResponse{
			ID:           existingID.String(),
			Email:        existingEmail,
			Name:         existingName,
			AvatarURL:    existingAvatar,
			IsGuest:      false,
			IsAdmin:      existingIsAdmin,
			AuthProvider: existingProvider,
			CashBalance:  existingCashBalance.StringFixed(8),
			CreatedAt:    existingCreatedAt.Format(time.RFC3339),
		}, nil
	} else if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("query error looking up user: %w", err)
	}

	// 2. User does not exist by google_id.
	// Check if there is an active authenticated guest session to upgrade (explicit account linking):
	guestID, ok := middleware.GetUserID(c)
	if ok && guestID != uuid.Nil {
		if email != "" {
			var conflictID uuid.UUID
			cErr := h.pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND id != $2 LIMIT 1", email, guestID).Scan(&conflictID)
			if cErr == nil {
				return nil, ErrAccountConflict
			}
		}

		// Upgrade the guest session
		upgradeQuery := `
			UPDATE users
			SET is_guest = false,
			    auth_provider = 'google',
			    google_id = $1,
			    email = $2,
			    name = $3,
			    avatar_url = $4,
			    last_active = NOW()
			WHERE id = $5 AND is_guest = true
			RETURNING id, is_guest, auth_provider, cash_balance, created_at, is_admin;
		`
		var (
			upgradedID       uuid.UUID
			upgradedIsGuest  bool
			upgradedIsAdmin  bool
			upgradedProvider string
			upgradedBalance  decimal.Decimal
			upgradedCreated  time.Time
		)
		uErr := h.pool.QueryRow(ctx, upgradeQuery, googleID, email, name, avatarURL, guestID).Scan(
			&upgradedID, &upgradedIsGuest, &upgradedProvider, &upgradedBalance, &upgradedCreated, &upgradedIsAdmin,
		)
		if uErr == nil {
			return &UserResponse{
				ID:           upgradedID.String(),
				Email:        &email,
				Name:         &name,
				AvatarURL:    &avatarURL,
				IsGuest:      false,
				IsAdmin:      upgradedIsAdmin,
				AuthProvider: upgradedProvider,
				CashBalance:  upgradedBalance.StringFixed(8),
				CreatedAt:    upgradedCreated.Format(time.RFC3339),
			}, nil
		}
	}

	// 3. User does not exist by google_id and no authenticated guest session to link.
	// Ensure the email is not already claimed by another account before registering.
	if email != "" {
		var conflictID uuid.UUID
		cErr := h.pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 LIMIT 1", email).Scan(&conflictID)
		if cErr == nil {
			return nil, ErrAccountConflict
		}
	}

	// 3. Insert brand-new registered user with initial $1,000.00 USDC
	initialBalance := decimal.NewFromInt(1000)
	clientIP := c.ClientIP()

	insertQuery := `
		INSERT INTO users (is_guest, auth_provider, google_id, email, name, avatar_url, cash_balance, ip_address)
		VALUES (false, 'google', $1, $2, $3, $4, $5, $6)
		RETURNING id, is_guest, auth_provider, cash_balance, created_at, is_admin;
	`
	var (
		newID       uuid.UUID
		newIsGuest  bool
		newIsAdmin  bool
		newProvider string
		newBalance  decimal.Decimal
		newCreated  time.Time
	)
	err = h.pool.QueryRow(ctx, insertQuery, googleID, email, name, avatarURL, initialBalance, clientIP).Scan(
		&newID, &newIsGuest, &newProvider, &newBalance, &newCreated, &newIsAdmin,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert new registered user: %w", err)
	}

	return &UserResponse{
		ID:           newID.String(),
		Email:        &email,
		Name:         &name,
		AvatarURL:    &avatarURL,
		IsGuest:      false,
		IsAdmin:      newIsAdmin,
		AuthProvider: newProvider,
		CashBalance:  newBalance.StringFixed(8),
		CreatedAt:    newCreated.Format(time.RFC3339),
	}, nil
}

// generateJWT signs an HMAC-SHA256 JWT containing authenticated user claims.
func (h *AuthHandler) generateJWT(userID string, isGuest bool, isAdmin bool, email string, name string, avatarURL string, provider string) (string, error) {
	claims := middleware.AuthClaims{
		UserID:       userID,
		IsGuest:      isGuest,
		IsAdmin:      isAdmin,
		Email:        email,
		Name:         name,
		AvatarURL:    avatarURL,
		AuthProvider: provider,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bayesmarket",
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(1 * time.Hour)), // 1-hour session to mitigate non-revocable token blast radius
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
