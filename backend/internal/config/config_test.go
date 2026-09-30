package config_test

import (
	"testing"

	"github.com/bayesmarket/bayesmarket/internal/config"
)

func TestConfig_IsDevOrLocal(t *testing.T) {
	tests := []struct {
		env      string
		expected bool
	}{
		{"development", true},
		{"dev", true},
		{"local", true},
		{"LOCAL", true},
		{"Dev", true},
		{"production", false},
		{"prod", false},
		{"staging", false},
		{"test", false},
		{"", false},
	}

	for _, tt := range tests {
		cfg := &config.Config{Environment: tt.env}
		if got := cfg.IsDevOrLocal(); got != tt.expected {
			t.Errorf("IsDevOrLocal() for env %q = %v; expected %v", tt.env, got, tt.expected)
		}
	}
}

func TestConfig_DefaultEnvironmentIsProduction(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("JWT_SECRET", "this-is-a-valid-production-jwt-secret-key-32-bytes")
	t.Setenv("ADMIN_TOKEN", "valid-admin-token-16-bytes")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected config.Load() to succeed, got %v", err)
	}
	if cfg.Environment != "production" {
		t.Errorf("expected default Environment to be 'production', got %q", cfg.Environment)
	}
	if cfg.IsDevOrLocal() {
		t.Errorf("expected IsDevOrLocal() to be false in default environment")
	}
}
