package rest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRedactQueryPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		omits    []string
	}{
		{
			name:     "No query string",
			input:    "/api/v1/markets",
			contains: []string{"/api/v1/markets"},
			omits:    []string{"REDACTED"},
		},
		{
			name:     "Non-sensitive query params preserved",
			input:    "/api/v1/markets?status=open&limit=10",
			contains: []string{"status=open", "limit=10"},
			omits:    []string{"REDACTED"},
		},
		{
			name:     "Idempotency key redacted",
			input:    "/api/v1/trade/buy?idempotency_key=my-super-secret-key-123&amount=50",
			contains: []string{"amount=50", "idempotency_key=REDACTED"},
			omits:    []string{"my-super-secret-key-123"},
		},
		{
			name:     "OAuth code and state redacted",
			input:    "/api/v1/auth/callback?code=sensitive_auth_code_xyz&state=secret_state_abc",
			contains: []string{"code=REDACTED", "state=REDACTED"},
			omits:    []string{"sensitive_auth_code_xyz", "secret_state_abc"},
		},
		{
			name:     "Token and secret redacted",
			input:    "/api/v1/test?token=jwt_value&secret=topsecret&admin_token=admin123",
			contains: []string{"token=REDACTED", "secret=REDACTED", "admin_token=REDACTED"},
			omits:    []string{"jwt_value", "topsecret", "admin123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactQueryPath(tt.input)
			for _, c := range tt.contains {
				if !strings.Contains(got, c) {
					t.Errorf("redactQueryPath(%q) = %q; expected to contain %q", tt.input, got, c)
				}
			}
			for _, o := range tt.omits {
				if strings.Contains(got, o) {
					t.Errorf("redactQueryPath(%q) = %q; expected NOT to contain %q", tt.input, got, o)
				}
			}
		})
	}
}

func TestRedactedLoggerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logBuf bytes.Buffer
	gin.DefaultWriter = &logBuf

	r := gin.New()
	r.Use(redactedLogger())
	r.GET("/api/v1/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test?idempotency_key=secret-key-999&token=my-token-abc&category=crypto", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logged := logBuf.String()
	if strings.Contains(logged, "secret-key-999") {
		t.Errorf("Expected idempotency_key to be redacted from log output, got: %s", logged)
	}
	if strings.Contains(logged, "my-token-abc") {
		t.Errorf("Expected token to be redacted from log output, got: %s", logged)
	}
	if !strings.Contains(logged, "REDACTED") {
		t.Errorf("Expected REDACTED in log output, got: %s", logged)
	}
	if !strings.Contains(logged, "category=crypto") {
		t.Errorf("Expected category=crypto in log output, got: %s", logged)
	}
}
