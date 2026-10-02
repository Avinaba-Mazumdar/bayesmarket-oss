package rest

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/bayesmarket/bayesmarket/internal/transport/ws"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type gzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) WriteHeaderNow() {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeaderNow()
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	g.Header().Del("Content-Length")
	return g.writer.Write(data)
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	g.Header().Del("Content-Length")
	return g.writer.Write([]byte(s))
}

// gzipMiddleware compresses HTTP responses using gzip when supported by the client.
func gzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") ||
			!strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") ||
			strings.Contains(c.GetHeader("Connection"), "Upgrade") ||
			strings.HasPrefix(c.Request.URL.Path, "/ws") {
			c.Next()
			return
		}

		gz, err := gzip.NewWriterLevel(c.Writer, gzip.DefaultCompression)
		if err != nil {
			c.Next()
			return
		}
		defer gz.Close()

		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer.Header().Del("Content-Length")
		c.Writer = &gzipWriter{ResponseWriter: c.Writer, writer: gz}
		c.Next()
	}
}

// maxBodySizeMiddleware restricts incoming HTTP request bodies to prevent DoS via payload exhaustion.
func maxBodySizeMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// securityHeadersMiddleware enforces modern OWASP HTTP security headers across all responses.
func securityHeadersMiddleware(isDevOrLocal bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent framing to neutralize clickjacking (OWASP A05)
		c.Header("X-Frame-Options", "DENY")
		// Disable MIME type sniffing (OWASP A05)
		c.Header("X-Content-Type-Options", "nosniff")
		// Disable buggy legacy XSS filter to prevent side-channel leaks
		c.Header("X-XSS-Protection", "0")
		// Restrict referrer leakage on external navigations
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		// Disable unused client browser hardware/sensor APIs
		c.Header("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
		// Restrict framing and script execution within API contexts
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none';")
		// Isolate cross-origin window contexts
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "cross-origin")

		// Enforce HTTP Strict Transport Security (HSTS) in non-dev environments
		if !isDevOrLocal {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		c.Next()
	}
}

func parseAllowedOrigins(corsOrigin string) []string {
	var origins []string
	for _, o := range strings.Split(corsOrigin, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// checkCORSOrigin safely evaluates whether an incoming Origin is permitted, and whether credentials may be exposed.
// Wildcards ('*') are NEVER paired with Access-Control-Allow-Credentials: true to prevent credential leakage.
func checkCORSOrigin(origin string, allowedOrigins []string, isDev bool) (allowed bool, allowCredentials bool, headerOrigin string) {
	if origin == "" {
		return false, false, ""
	}
	lower := strings.ToLower(origin)

	// 1. Explicit whitelisted origins: allow credentials
	for _, allowed := range allowedOrigins {
		if allowed != "*" && strings.EqualFold(origin, allowed) {
			return true, true, origin
		}
	}

	// 2. Development localhost / loopback interfaces: allow credentials
	if isDev {
		if strings.HasPrefix(lower, "http://localhost:") || lower == "http://localhost" ||
			strings.HasPrefix(lower, "http://127.0.0.1:") || lower == "http://127.0.0.1" {
			return true, true, origin
		}
	}

	// 3. Explicit wildcard '*' allowed: permit cross-origin access but STRICTLY DISALLOW credentials
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true, false, "*"
		}
	}

	return false, false, ""
}

// SetupRouter constructs and configures the Gin HTTP engine with all REST routes, WebSocket endpoints, and middleware.
func SetupRouter(pool *pgxpool.Pool, cfg *config.Config, hubOpt ...*ws.Hub) *gin.Engine {
	var hub *ws.Hub
	if len(hubOpt) > 0 && hubOpt[0] != nil {
		hub = hubOpt[0]
	}

	isDevOrLocal := true
	corsOrigin := "*"
	jwtSecret := ""

	if cfg != nil {
		isDevOrLocal = cfg.IsDevOrLocal()
		if cfg.CORSOrigin != "" {
			corsOrigin = cfg.CORSOrigin
		}
		jwtSecret = cfg.JWTSecret
	}

	if !isDevOrLocal {
		gin.SetMode(gin.ReleaseMode)
	}

	if jwtSecret == "" && isDevOrLocal {
		jwtSecret = "bayesmarket-development-hmac-sha256-default-secret-key-32b"
	}

	router := gin.New()
	if cfg != nil && len(cfg.TrustedProxies) > 0 {
		_ = router.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		// When no proxies are explicitly trusted, disable proxy trust completely.
		// This prevents callers from spoofing ClientIP() via X-Forwarded-For or X-Real-IP headers.
		_ = router.SetTrustedProxies(nil)
	}
	router.Use(
		securityHeadersMiddleware(isDevOrLocal),
		redactedLogger(),
		gin.Recovery(),
		gzipMiddleware(),
		maxBodySizeMiddleware(1<<20), // 1MB payload ceiling
	)

	// Strict OWASP-Compliant CORS Middleware
	allowedOrigins := parseAllowedOrigins(corsOrigin)
	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowed, allowCreds, headerOrigin := checkCORSOrigin(origin, allowedOrigins, isDevOrLocal)
		if allowed {
			c.Header("Access-Control-Allow-Origin", headerOrigin)
			if allowCreds {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Idempotency-Key, Accept, X-Requested-With")
		c.Header("Access-Control-Expose-Headers", "Retry-After, Content-Length")

		if c.Request.Method == "OPTIONS" {
			if origin != "" && !allowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// Rate Limiters
	publicReadLimiter := middleware.NewPublicReadLimiter()
	actionLimiter := middleware.NewActionLimiter()
	quoteLimiter := middleware.NewQuoteLimiter()
	if cfg != nil && cfg.DisableRateLimits {
		publicReadLimiter.SetDisabled(true)
		actionLimiter.SetDisabled(true)
		quoteLimiter.SetDisabled(true)
		middleware.SetAdminFailureLimiterDisabled(true)
	}

	// In-memory read-through cache (1-second TTL, event-invalidated on trades/settlements)
	marketCache := NewMarketCache(1 * time.Second)

	// Handlers
	authHandler := NewAuthHandler(pool, cfg)
	marketHandler := NewMarketHandler(pool, marketCache)
	faucetHandler := NewFaucetHandler(pool, isDevOrLocal)
	portfolioHandler := NewPortfolioHandler(pool)
	tradeHandler := NewTradeHandler(pool, hub)
	tradeHandler.SetCache(marketCache)
	adminHandler := NewAdminHandler(pool, hub)
	adminHandler.SetMarketCache(marketCache)

	// WebSocket Endpoints
	if hub != nil {
		wsHandler := ws.NewWSHandler(hub, corsOrigin, isDevOrLocal)
		router.GET("/ws/markets/:id", wsHandler.HandleMarketWS)
		router.GET("/ws/markets", wsHandler.HandleGlobalWS)
		router.GET("/ws", wsHandler.HandleGlobalWS)
	}

	// Health check endpoints (unlimited, for keep-alive cron jobs, load balancers, and orchestrators)
	healthHandler := func(c *gin.Context) {
		dbStatus := "disconnected"
		if pool != nil {
			if err := pool.Ping(c.Request.Context()); err == nil {
				dbStatus = "connected"
			} else {
				dbStatus = "degraded"
			}
		}

		httpStatus := http.StatusOK
		overallStatus := "healthy"
		if dbStatus != "connected" {
			httpStatus = http.StatusServiceUnavailable
			overallStatus = "unhealthy"
		}

		c.JSON(httpStatus, gin.H{
			"status":    overallStatus,
			"service":   "bayesmarket-backend",
			"database":  dbStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}

	router.GET("/healthz", healthHandler)
	router.GET("/health", healthHandler)
	router.HEAD("/healthz", healthHandler)
	router.HEAD("/health", healthHandler)

	// Prometheus metrics endpoint (requires Admin JWT auth)
	router.GET("/metrics", middleware.RequireAdminAuth(jwtSecret, pool), func(c *gin.Context) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		var dbConns int
		if pool != nil {
			dbConns = int(pool.Stat().AcquiredConns())
		}

		metricsText := fmt.Sprintf(`# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines %d
# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.
# TYPE go_memstats_alloc_bytes gauge
go_memstats_alloc_bytes %d
# HELP go_memstats_sys_bytes Number of bytes obtained from system.
# TYPE go_memstats_sys_bytes gauge
go_memstats_sys_bytes %d
# HELP bayesmarket_db_connections_acquired Active database connections in use.
# TYPE bayesmarket_db_connections_acquired gauge
bayesmarket_db_connections_acquired %d
# HELP bayesmarket_up Health status indicator (1 = healthy).
# TYPE bayesmarket_up gauge
bayesmarket_up 1
`, runtime.NumGoroutine(), m.Alloc, m.Sys, dbConns)

		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(metricsText))
	})

	// API v1 Group
	v1 := router.Group("/api/v1")
	{
		// 0. Public System Configuration
		v1.GET("/config", publicReadLimiter.LimitByIP(), func(c *gin.Context) {
			appEnv := "development"
			isDev := true
			googleAuthEnabled := false
			if cfg != nil {
				appEnv = cfg.Environment
				isDev = cfg.IsDevOrLocal()
				googleAuthEnabled = cfg.GoogleClientID != ""
			}
			c.JSON(http.StatusOK, gin.H{
				"app_env":             appEnv,
				"is_dev":              isDev,
				"google_auth_enabled": googleAuthEnabled,
			})
		})

		// 1. Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/guest", actionLimiter.LimitByClientOrUser(), authHandler.HandleGuestAuth)
			auth.POST("/google/verify", middleware.OptionalAuth(jwtSecret), actionLimiter.LimitByClientOrUser(), authHandler.HandleGoogleAuthVerify)
			auth.GET("/google/url", publicReadLimiter.LimitByIP(), authHandler.HandleGoogleAuthURL)
			auth.POST("/google/callback", middleware.OptionalAuth(jwtSecret), actionLimiter.LimitByClientOrUser(), authHandler.HandleGoogleAuthCallback)
			auth.GET("/me", middleware.RequireAuth(jwtSecret), publicReadLimiter.LimitByIP(), authHandler.HandleGetMe)
		}

		// 2. Markets, Quotes & Orders
		markets := v1.Group("/markets")
		{
			markets.GET("", publicReadLimiter.LimitByIP(), marketHandler.HandleGetMarkets)
			markets.GET("/:id", publicReadLimiter.LimitByIP(), marketHandler.HandleGetMarketByID)
			markets.POST("/:id/quote", quoteLimiter.LimitByClientOrUser(), marketHandler.HandleMarketQuote)
			markets.POST("/:id/orders",
				middleware.RequireAuth(jwtSecret),
				actionLimiter.LimitByClientOrUser(),
				tradeHandler.HandlePlaceOrder,
			)
		}

		// 3. Faucet claim (Protected)
		v1.POST("/faucet",
			middleware.RequireAuth(jwtSecret),
			actionLimiter.LimitByClientOrUser(),
			faucetHandler.HandleClaimFaucet,
		)

		// 4. Portfolio read & Cashout (Protected)
		v1.GET("/portfolio",
			middleware.RequireAuth(jwtSecret),
			portfolioHandler.HandleGetPortfolio,
		)
		v1.POST("/portfolio/cashout",
			middleware.RequireAuth(jwtSecret),
			actionLimiter.LimitByClientOrUser(),
			tradeHandler.HandleCashOut,
		)

		// 5. Admin Market Management, Creation, Edit, Delete & Resolution (Protected)
		admin := v1.Group("/admin")
		admin.Use(actionLimiter.LimitByIP())
		{
			admin.GET("/verify",
				middleware.RequireAdminAuth(jwtSecret, pool),
				adminHandler.HandleVerifyAdmin,
			)
			admin.POST("/markets",
				middleware.RequireAdminAuth(jwtSecret, pool),
				adminHandler.HandleCreateMarket,
			)
			admin.PUT("/markets/:id",
				middleware.RequireAdminAuth(jwtSecret, pool),
				adminHandler.HandleEditMarket,
			)
			admin.DELETE("/markets/:id",
				middleware.RequireAdminAuth(jwtSecret, pool),
				adminHandler.HandleDeleteMarket,
			)
			admin.POST("/markets/:id/resolve",
				middleware.RequireAdminAuth(jwtSecret, pool),
				adminHandler.HandleResolveMarket,
			)
		}
	}

	return router
}

// redactedLogger returns a gin.HandlerFunc that redacts sensitive query parameters
// (such as idempotency keys, auth tokens, secrets, credentials) before outputting logs.
func redactedLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if param.IsOutputColor() {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		redactedPath := redactQueryPath(param.Path)

		return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			methodColor, param.Method, resetColor,
			redactedPath,
			param.ErrorMessage,
		)
	})
}

// redactQueryPath parses rawPath, redacts any sensitive query parameters, and returns the sanitized path.
func redactQueryPath(rawPath string) string {
	u, err := url.Parse(rawPath)
	if err != nil {
		return rawPath
	}
	q := u.Query()
	if len(q) == 0 {
		return rawPath
	}

	for key := range q {
		lower := strings.ToLower(key)
		if isSensitiveParam(lower) {
			q.Set(key, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// isSensitiveParam checks whether a query parameter key represents sensitive data that should not appear in server logs.
func isSensitiveParam(k string) bool {
	switch k {
	case "token", "access_token", "id_token", "refresh_token", "code", "state", "secret", "password", "key", "api_key", "apikey", "auth", "authorization", "signature", "sig", "oracle_proof", "idempotency_key", "idempotency-key":
		return true
	}
	return strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password") ||
		strings.Contains(k, "idempotency")
}
