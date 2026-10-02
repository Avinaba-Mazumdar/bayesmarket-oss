package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

const (
	CtxUserIDKey       = "userID"
	CtxIsGuestKey      = "isGuest"
	CtxIsAdminKey      = "isAdmin"
	CtxIsSuperadminKey = "isSuperadmin"
	CtxUserEmailKey    = "userEmail"
	CtxUserNameKey     = "userName"
	CtxUserAvatarKey   = "userAvatar"
	CtxAuthProviderKey = "authProvider"
	CtxAuthClaimsKey   = "authClaims"
)

// AuthClaims defines the cryptographic JWT payload for guest and registered user sessions.
type AuthClaims struct {
	UserID       string `json:"user_id"`
	IsGuest      bool   `json:"is_guest"`
	IsAdmin      bool   `json:"is_admin"`
	IsSuperadmin bool   `json:"is_superadmin,omitempty"`
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	AuthProvider string `json:"auth_provider,omitempty"`
	jwt.RegisteredClaims
}

// GuestClaims is retained as an alias for backwards compatibility.
type GuestClaims = AuthClaims

// RequireAuth validates the Bearer JWT in the Authorization header.
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	secretBytes := []byte(jwtSecret)

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header is required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header must be formatted as 'Bearer <token>'",
			})
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		claims := &GuestClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secretBytes, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or expired session token",
			})
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Malformed user identifier in session claims",
			})
			return
		}

		isAdmin := claims.IsAdmin || claims.IsSuperadmin

		c.Set(CtxUserIDKey, userID)
		c.Set(CtxIsGuestKey, claims.IsGuest)
		c.Set(CtxIsAdminKey, isAdmin)
		c.Set(CtxIsSuperadminKey, isAdmin)
		c.Set(CtxUserEmailKey, claims.Email)
		c.Set(CtxUserNameKey, claims.Name)
		c.Set(CtxUserAvatarKey, claims.AvatarURL)
		c.Set(CtxAuthProviderKey, claims.AuthProvider)
		c.Set(CtxAuthClaimsKey, claims)

		c.Next()
	}
}

// OptionalAuth parses Bearer JWT if provided, but does not abort if absent.
func OptionalAuth(jwtSecret string) gin.HandlerFunc {
	secretBytes := []byte(jwtSecret)

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Next()
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		claims := &GuestClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secretBytes, nil
		})

		if err == nil && token.Valid {
			if userID, parseErr := uuid.Parse(claims.UserID); parseErr == nil {
				isAdmin := claims.IsAdmin || claims.IsSuperadmin
				c.Set(CtxUserIDKey, userID)
				c.Set(CtxIsGuestKey, claims.IsGuest)
				c.Set(CtxIsAdminKey, isAdmin)
				c.Set(CtxIsSuperadminKey, isAdmin)
				c.Set(CtxUserEmailKey, claims.Email)
				c.Set(CtxUserNameKey, claims.Name)
				c.Set(CtxUserAvatarKey, claims.AvatarURL)
				c.Set(CtxAuthProviderKey, claims.AuthProvider)
				c.Set(CtxAuthClaimsKey, claims)
			}
		}

		c.Next()
	}
}

// GetUserID retrieves the authenticated user's UUID from the request context.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(CtxUserIDKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// GetIsGuest retrieves whether the authenticated session is an ephemeral guest.
func GetIsGuest(c *gin.Context) bool {
	val, exists := c.Get(CtxIsGuestKey)
	if !exists {
		return false
	}
	isGuest, ok := val.(bool)
	return ok && isGuest
}

// GetIsAdmin retrieves whether the authenticated user has admin privileges.
func GetIsAdmin(c *gin.Context) bool {
	val, exists := c.Get(CtxIsAdminKey)
	if !exists {
		return false
	}
	isAdmin, ok := val.(bool)
	return ok && isAdmin
}

// GetIsSuperadmin is retained as an alias for GetIsAdmin.
func GetIsSuperadmin(c *gin.Context) bool {
	return GetIsAdmin(c)
}

// GetClaims retrieves the full AuthClaims payload from request context.
func GetClaims(c *gin.Context) (*AuthClaims, bool) {
	val, exists := c.Get(CtxAuthClaimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := val.(*AuthClaims)
	return claims, ok
}

// adminFailureLimiter tracks failed administrative authorization attempts per client IP.
var adminFailureLimiter = NewRateLimiter(rate.Every(12*time.Second), 5, 60*time.Second)

// SetAdminFailureLimiterDisabled enables or disables failed admin attempt limiting (useful for tests).
func SetAdminFailureLimiterDisabled(disabled bool) {
	adminFailureLimiter.SetDisabled(disabled)
}

// RequireAdminAuth validates that the request comes from an authenticated user with is_admin=true.
// Static admin tokens are eliminated; authority derives strictly from the user's Neon DB record.
func RequireAdminAuth(jwtSecret string, poolOpt ...*pgxpool.Pool) gin.HandlerFunc {
	var pool *pgxpool.Pool
	if len(poolOpt) > 0 {
		pool = poolOpt[0]
	}
	secretBytes := []byte(jwtSecret)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		limiter := adminFailureLimiter.getLimiter(clientIP)

		adminFailureLimiter.mu.RLock()
		disabled := adminFailureLimiter.disabled
		adminFailureLimiter.mu.RUnlock()

		if !disabled && limiter.Tokens() < 1 {
			if !limiter.Allow() {
				c.Header("Retry-After", "60")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":       "rate_limit_exceeded",
					"message":     "Too many failed admin authorization attempts. Please try again later.",
					"retry_after": 60,
				})
				return
			}
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header with user Bearer token is required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authorization header must be formatted as 'Bearer <token>'",
			})
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		claims := &AuthClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secretBytes, nil
		})

		if err != nil || !token.Valid {
			if !disabled {
				limiter.Allow()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or expired user session token",
			})
			return
		}

		parsedID, parseErr := uuid.Parse(claims.UserID)
		if parseErr != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Malformed user identifier in session claims",
			})
			return
		}

		isAdmin := claims.IsAdmin || claims.IsSuperadmin

		// Always verify directly against Neon DB if pool is available (guarantees real-time revocation and DB-only authority)
		if pool != nil {
			var dbIsAdmin bool
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			err := pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE id = $1", parsedID).Scan(&dbIsAdmin)
			if err != nil || !dbIsAdmin {
				if !disabled {
					limiter.Allow()
				}
				log.Printf("[SECURITY] User %s attempted admin access without is_admin privilege from IP=%s path=%s", parsedID, clientIP, c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "Admin privileges required (is_admin=true). Admin status can only be granted directly in Neon DB.",
				})
				return
			}
			isAdmin = dbIsAdmin
		}

		if !isAdmin {
			if !disabled {
				limiter.Allow()
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Admin privileges required (is_admin=true)",
			})
			return
		}

		c.Set(CtxUserIDKey, parsedID)
		c.Set(CtxIsGuestKey, claims.IsGuest)
		c.Set(CtxIsAdminKey, true)
		c.Set(CtxIsSuperadminKey, true)
		c.Set(CtxUserEmailKey, claims.Email)
		c.Set(CtxUserNameKey, claims.Name)
		c.Set(CtxUserAvatarKey, claims.AvatarURL)
		c.Set(CtxAuthProviderKey, claims.AuthProvider)
		c.Set(CtxAuthClaimsKey, claims)
		c.Next()
	}
}

// RequireSuperadminAuth is an alias for RequireAdminAuth for backwards compatibility.
func RequireSuperadminAuth(jwtSecret string, poolOpt ...*pgxpool.Pool) gin.HandlerFunc {
	return RequireAdminAuth(jwtSecret, poolOpt...)
}
