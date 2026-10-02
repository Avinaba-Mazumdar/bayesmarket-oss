package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bayesmarket/bayesmarket/internal/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRateLimiter_BurstAndLimit(t *testing.T) {
	// Create a test rate limiter allowing 2 requests per second with a burst of 3
	rl := middleware.NewRateLimiter(rate.Every(500*time.Millisecond), 3, 2)

	router := gin.New()
	router.GET("/test-rate", rl.LimitByIP(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Fire 3 burst requests — all 3 should succeed (HTTP 200)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test-rate", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Request %d in burst failed with code %d", i+1, w.Code)
		}
	}

	// 4th request immediately should exceed burst and return HTTP 429 Too Many Requests
	wExceed := httptest.NewRecorder()
	reqExceed, _ := http.NewRequest(http.MethodGet, "/test-rate", nil)
	reqExceed.RemoteAddr = "192.168.1.100:12345"
	router.ServeHTTP(wExceed, reqExceed)

	if wExceed.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected HTTP 429 Too Many Requests, got %d", wExceed.Code)
	}

	retryAfter := wExceed.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header on HTTP 429 response")
	}

	// A different IP address should still succeed (per-IP isolation)
	wOtherIP := httptest.NewRecorder()
	reqOtherIP, _ := http.NewRequest(http.MethodGet, "/test-rate", nil)
	reqOtherIP.RemoteAddr = "192.168.1.200:12345"
	router.ServeHTTP(wOtherIP, reqOtherIP)

	if wOtherIP.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 for isolated IP, got %d", wOtherIP.Code)
	}
}

func TestRequireAdminAuth_FailedAttemptLimiting(t *testing.T) {
	middleware.SetAdminFailureLimiterDisabled(false)
	defer middleware.SetAdminFailureLimiterDisabled(true)

	router := gin.New()
	router.POST("/admin/test", middleware.RequireSuperadminAuth("jwt-secret"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "admin_granted"})
	})

	ip := "203.0.113.42:12345"

	// 5 failed attempts with invalid token -> 401 Unauthorized
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/admin/test", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		req.RemoteAddr = ip
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Attempt %d: expected 401 Unauthorized, got %d", i+1, w.Code)
		}
	}

	// 6th attempt should be blocked with 429 Too Many Requests
	wBlocked := httptest.NewRecorder()
	reqBlocked, _ := http.NewRequest(http.MethodPost, "/admin/test", nil)
	reqBlocked.Header.Set("Authorization", "Bearer wrong-token")
	reqBlocked.RemoteAddr = ip
	router.ServeHTTP(wBlocked, reqBlocked)

	if wBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected 429 Too Many Requests after exceeding failure burst, got %d", wBlocked.Code)
	}

	retryAfter := wBlocked.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header on 429 response")
	}
}
