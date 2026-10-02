package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bayesmarket/bayesmarket/internal/config"
	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestSecurityHeaders verifies that all OWASP-recommended HTTP security headers are injected on responses.
func TestSecurityHeaders(t *testing.T) {
	// 1. In dev/local mode
	devCfg := &config.Config{
		Environment: "development",
		CORSOrigin:  "http://localhost:4200",
	}
	devRouter := rest.SetupRouter(nil, devCfg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	devRouter.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Frame-Options":              "DENY",
		"X-Content-Type-Options":       "nosniff",
		"X-XSS-Protection":             "0",
		"Referrer-Policy":              "strict-origin-when-cross-origin",
		"Content-Security-Policy":      "default-src 'none'; frame-ancestors 'none'; base-uri 'none';",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "cross-origin",
	}

	for header, expectedVal := range expectedHeaders {
		actualVal := w.Header().Get(header)
		if actualVal != expectedVal {
			t.Errorf("Expected header %q to be %q, got %q", header, expectedVal, actualVal)
		}
	}

	// In dev mode, HSTS should not be set (avoids breaking local HTTP testing)
	if hsts := w.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("Expected HSTS to be omitted in dev mode, got %q", hsts)
	}

	// 2. In production mode
	prodCfg := &config.Config{
		Environment: "production",
		CORSOrigin:  "https://bayesmarket.com",
		JWTSecret:   "production-super-secret-key-that-is-at-least-32-chars-long",
	}
	prodRouter := rest.SetupRouter(nil, prodCfg)

	wProd := httptest.NewRecorder()
	reqProd, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	prodRouter.ServeHTTP(wProd, reqProd)

	prodHSTS := wProd.Header().Get("Strict-Transport-Security")
	if prodHSTS != "max-age=31536000; includeSubDomains; preload" {
		t.Errorf("Expected production HSTS header, got %q", prodHSTS)
	}
}

// TestCORSOriginHandling tests strict CORS origin verification and prevents credential leakage on wildcards.
func TestCORSOriginHandling(t *testing.T) {
	// A. Whitelisted specific origin
	prodCfg := &config.Config{
		Environment: "production",
		CORSOrigin:  "https://bayesmarket.com,https://app.bayesmarket.com",
		JWTSecret:   "production-super-secret-key-that-is-at-least-32-chars-long",
	}
	prodRouter := rest.SetupRouter(nil, prodCfg)

	// Valid origin OPTIONS
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodOptions, "/api/v1/config", nil)
	req1.Header.Set("Origin", "https://bayesmarket.com")
	prodRouter.ServeHTTP(w1, req1)

	if w1.Code != http.StatusNoContent {
		t.Errorf("Expected 204 for allowed origin OPTIONS, got %d", w1.Code)
	}
	if w1.Header().Get("Access-Control-Allow-Origin") != "https://bayesmarket.com" {
		t.Errorf("Expected Access-Control-Allow-Origin 'https://bayesmarket.com', got %q", w1.Header().Get("Access-Control-Allow-Origin"))
	}
	if w1.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("Expected Access-Control-Allow-Credentials 'true' for whitelisted origin, got %q", w1.Header().Get("Access-Control-Allow-Credentials"))
	}

	// Unauthorized malicious origin OPTIONS
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodOptions, "/api/v1/config", nil)
	req2.Header.Set("Origin", "https://evil-attacker.com")
	prodRouter.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for disallowed origin OPTIONS, got %d", w2.Code)
	}

	// B. Wildcard CORS origin must NEVER set Access-Control-Allow-Credentials: true
	wildcardCfg := &config.Config{
		Environment: "development",
		CORSOrigin:  "*",
	}
	wildcardRouter := rest.SetupRouter(nil, wildcardCfg)

	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	req3.Header.Set("Origin", "https://random-public-site.com")
	wildcardRouter.ServeHTTP(w3, req3)

	if w3.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin '*', got %q", w3.Header().Get("Access-Control-Allow-Origin"))
	}
	if w3.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("Wildcard CORS must NOT set Access-Control-Allow-Credentials, got %q", w3.Header().Get("Access-Control-Allow-Credentials"))
	}
}
