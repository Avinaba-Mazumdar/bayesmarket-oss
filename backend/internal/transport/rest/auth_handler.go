package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
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

// AuthHandler handles user registration, guest sessions, Google OAuth, and profile management.
type AuthHandler struct {
	pool               *pgxpool.Pool
	jwtSecret          string
	googleClientID     string
	googleClientSecret string
	googleRedirectURI  string
	httpClient         *http.Client
	isDevOrLocal       bool
}

// NewAuthHandler constructs an AuthHandler with configuration.
func NewAuthHandler(pool *pgxpool.Pool, cfg *config.Config) *AuthHandler {
	googleClientID := ""
	googleClientSecret := ""
	googleRedirectURI := "http://localhost:4200/auth/callback"
	jwtSecret := "bayesmarket-development-hmac-sha256-default-secret-key-32b"
	isDevOrLocal := true

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

// GoogleCallbackRequest payload containing authorization code.
type GoogleCallbackRequest struct {
	Code string `json:"code" binding:"required"`
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	clientIP := c.ClientIP()
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
		var guestCount int
		countQuery := `
			SELECT COUNT(*) FROM users
			WHERE is_guest = true AND ip_address = $1 AND created_at > NOW() - INTERVAL '24 hours';
		`
		if err := h.pool.QueryRow(ctx, countQuery, clientIP).Scan(&guestCount); err == nil && guestCount >= dailyCap {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "guest_limit_exceeded",
				"message": "Maximum guest accounts created for this IP address today. Please sign in or try again later.",
			})
			return
		}

		query := `
			INSERT INTO users (is_guest, cash_balance, auth_provider, ip_address)
			VALUES (true, $1, 'guest', $2)
			RETURNING id, is_guest, auth_provider, cash_balance, created_at;
		`
		err := h.pool.QueryRow(ctx, query, initialBalance, clientIP).Scan(
			&userID, &isGuest, &authProvider, &cashBalance, &createdAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to provision guest user session",
			})
			return
		}
	}

	tokenString, err := h.generateJWT(userID.String(), isGuest, "", "", "", authProvider)
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
		googleID = "google-mock-" + uuid.New().String()[:8]
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
		googleID = "google-mock-" + uuid.New().String()[:8]
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "auth_error",
			"message": "Failed to authenticate user profile",
		})
		return
	}

	tokenString, err := h.generateJWT(user.ID, false, email, name, avatarURL, "google")
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

	state := uuid.New().String()
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// If dev mode or mock code, bypass with mock user
	if h.googleClientID == "" || strings.HasPrefix(req.Code, "mock-") {
		user, err := h.upsertGoogleUser(ctx, c, "google-mock-cb", "oauth.trader@bayesmarket.com", "OAuth Trader", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database_error", "message": "Failed to create user session"})
			return
		}
		tokenString, _ := h.generateJWT(user.ID, false, "oauth.trader@bayesmarket.com", "OAuth Trader", "", "google")
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to create or update user profile",
		})
		return
	}

	tokenString, err := h.generateJWT(user.ID, false, tokenInfo.Email, tokenInfo.Name, tokenInfo.Picture, "google")
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
			CashBalance:  "1000.00000000",
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	query := `
		SELECT id, is_guest, auth_provider, email, name, avatar_url, cash_balance, created_at
		FROM users
		WHERE id = $1;
	`
	err := h.pool.QueryRow(ctx, query, userID).Scan(
		&id, &isGuest, &authProvider, &email, &name, &avatarURL, &cashBalance, &createdAt,
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
		AuthProvider: authProvider,
		CashBalance:  cashBalance.StringFixed(8),
		CreatedAt:    createdAt.Format(time.RFC3339),
	})
}

// verifyGoogleIDToken calls Google's tokeninfo endpoint to authenticate the ID token.
func (h *AuthHandler) verifyGoogleIDToken(ctx context.Context, idToken string) (*GoogleTokenInfo, error) {
	verifyURL := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, verifyURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error during Google token verification: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google verification response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google token verification failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenInfo GoogleTokenInfo
	if err := json.Unmarshal(bodyBytes, &tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to parse Google tokeninfo payload: %w", err)
	}

	if tokenInfo.Email == "" || tokenInfo.Sub == "" {
		return nil, fmt.Errorf("Google tokeninfo missing email or sub identifier")
	}

	return &tokenInfo, nil
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
			AuthProvider: "google",
			CashBalance:  "1000.00000000",
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	queryFind := `
		SELECT id, is_guest, auth_provider, email, name, avatar_url, cash_balance, created_at
		FROM users
		WHERE google_id = $1 OR email = $2
		LIMIT 1;
	`
	err := h.pool.QueryRow(ctx, queryFind, googleID, email).Scan(
		&existingID, &existingIsGuest, &existingProvider, &existingEmail, &existingName, &existingAvatar, &existingCashBalance, &existingCreatedAt,
	)

	if err == nil {
		// User exists - update latest name/avatar/google_id and timestamp
		updateQuery := `
			UPDATE users
			SET name = COALESCE(NULLIF($1, ''), name),
			    avatar_url = COALESCE(NULLIF($2, ''), avatar_url),
			    google_id = $3,
			    last_active = NOW()
			WHERE id = $4
			RETURNING name, avatar_url;
		`
		_ = h.pool.QueryRow(ctx, updateQuery, name, avatarURL, googleID, existingID).Scan(&existingName, &existingAvatar)

		return &UserResponse{
			ID:           existingID.String(),
			Email:        existingEmail,
			Name:         existingName,
			AvatarURL:    existingAvatar,
			IsGuest:      false,
			AuthProvider: existingProvider,
			CashBalance:  existingCashBalance.StringFixed(8),
			CreatedAt:    existingCreatedAt.Format(time.RFC3339),
		}, nil
	} else if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("query error looking up user: %w", err)
	}

	// 2. User does not exist by google_id or email.
	// Check if there is an active guest session in Authorization header to upgrade:
	guestID, ok := middleware.GetUserID(c)
	if ok && guestID != uuid.Nil {
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
			RETURNING id, is_guest, auth_provider, cash_balance, created_at;
		`
		var (
			upgradedID       uuid.UUID
			upgradedIsGuest  bool
			upgradedProvider string
			upgradedBalance  decimal.Decimal
			upgradedCreated  time.Time
		)
		uErr := h.pool.QueryRow(ctx, upgradeQuery, googleID, email, name, avatarURL, guestID).Scan(
			&upgradedID, &upgradedIsGuest, &upgradedProvider, &upgradedBalance, &upgradedCreated,
		)
		if uErr == nil {
			return &UserResponse{
				ID:           upgradedID.String(),
				Email:        &email,
				Name:         &name,
				AvatarURL:    &avatarURL,
				IsGuest:      false,
				AuthProvider: upgradedProvider,
				CashBalance:  upgradedBalance.StringFixed(8),
				CreatedAt:    upgradedCreated.Format(time.RFC3339),
			}, nil
		}
	}

	// 3. Insert brand-new registered user with initial $1,000.00 USDC
	initialBalance := decimal.NewFromInt(1000)
	clientIP := c.ClientIP()

	insertQuery := `
		INSERT INTO users (is_guest, auth_provider, google_id, email, name, avatar_url, cash_balance, ip_address)
		VALUES (false, 'google', $1, $2, $3, $4, $5, $6)
		RETURNING id, is_guest, auth_provider, cash_balance, created_at;
	`
	var (
		newID       uuid.UUID
		newIsGuest  bool
		newProvider string
		newBalance  decimal.Decimal
		newCreated  time.Time
	)
	err = h.pool.QueryRow(ctx, insertQuery, googleID, email, name, avatarURL, initialBalance, clientIP).Scan(
		&newID, &newIsGuest, &newProvider, &newBalance, &newCreated,
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
		AuthProvider: newProvider,
		CashBalance:  newBalance.StringFixed(8),
		CreatedAt:    newCreated.Format(time.RFC3339),
	}, nil
}

// generateJWT signs an HMAC-SHA256 JWT containing authenticated user claims.
func (h *AuthHandler) generateJWT(userID string, isGuest bool, email string, name string, avatarURL string, provider string) (string, error) {
	claims := middleware.AuthClaims{
		UserID:       userID,
		IsGuest:      isGuest,
		Email:        email,
		Name:         name,
		AvatarURL:    avatarURL,
		AuthProvider: provider,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bayesmarket",
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(24 * time.Hour)), // 24-hour session
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
