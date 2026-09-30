package middleware

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

const (
	CtxUserIDKey       = "userID"
	CtxIsGuestKey      = "isGuest"
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
				"message": "Invalid, expired, or corrupted authorization token",
			})
			return
		}

		parsedID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Token contains malformed user identifier",
			})
			return
		}

		c.Set(CtxUserIDKey, parsedID)
		c.Set(CtxIsGuestKey, claims.IsGuest)
		c.Set(CtxUserEmailKey, claims.Email)
		c.Set(CtxUserNameKey, claims.Name)
		c.Set(CtxUserAvatarKey, claims.AvatarURL)
		c.Set(CtxAuthProviderKey, claims.AuthProvider)
		c.Set(CtxAuthClaimsKey, claims)
		c.Next()
	}
}

// OptionalAuth parses the Bearer JWT if provided, but does not reject unauthenticated requests.
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
			if parsedID, err := uuid.Parse(claims.UserID); err == nil {
				c.Set(CtxUserIDKey, parsedID)
				c.Set(CtxIsGuestKey, claims.IsGuest)
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

// GetUserID retrieves the authenticated UUID from request context.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(CtxUserIDKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
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

// RequireAdminAuth validates that the request contains an authorized administrative credential.
// It accepts either a static admin token (configured via ADMIN_TOKEN) or a valid non-guest administrative JWT.
func RequireAdminAuth(adminToken string, jwtSecret string) gin.HandlerFunc {
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
					"message":     "Too many failed administrative authorization attempts. Please try again later.",
					"retry_after": 60,
				})
				return
			}
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Administrative authorization header is required",
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

		// Direct match with configured AdminToken (constant-time comparison against timing attacks)
		if adminToken != "" && subtle.ConstantTimeCompare([]byte(tokenString), []byte(adminToken)) == 1 {
			c.Next()
			return
		}

		// Failed authentication: consume failure rate limit and log security alert
		if !disabled {
			limiter.Allow()
		}
		log.Printf("[SECURITY WARNING] Failed admin authorization attempt from IP=%s path=%s", clientIP, c.Request.URL.Path)

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "Admin privileges required to perform this action",
		})
	}
}
