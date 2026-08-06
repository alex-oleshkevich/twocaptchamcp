package config

import (
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadRequiresAPIKey(t *testing.T) {
	_, err := Load(env(nil))
	if err == nil {
		t.Fatal("expected error when TWOCAPTCHA_API_KEY is missing")
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"TWOCAPTCHA_API_KEY": "key"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != defaultAddress {
		t.Errorf("Address = %s, want %s", cfg.Address, defaultAddress)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.Timeout != 180*time.Second {
		t.Errorf("Timeout = %v, want 180s", cfg.Timeout)
	}
}

func TestLoadRequiresTokenForNonLoopback(t *testing.T) {
	_, err := Load(env(map[string]string{
		"TWOCAPTCHA_API_KEY":    "key",
		"TWOCAPTCHAMCP_ADDRESS": "0.0.0.0:8080",
	}))
	if err == nil {
		t.Fatal("expected error requiring TWOCAPTCHAMCP_TOKEN for a non-loopback address")
	}
}

func TestLoadAllowsNonLoopbackWithToken(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"TWOCAPTCHA_API_KEY":    "key",
		"TWOCAPTCHAMCP_ADDRESS": "0.0.0.0:8080",
		"TWOCAPTCHAMCP_TOKEN":   "tok",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Token != "tok" {
		t.Errorf("Token = %s, want tok", cfg.Token)
	}
}

func TestLoadInvalidAddress(t *testing.T) {
	_, err := Load(env(map[string]string{"TWOCAPTCHA_API_KEY": "key", "TWOCAPTCHAMCP_ADDRESS": "not-an-address"}))
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestLoadInvalidTimeout(t *testing.T) {
	_, err := Load(env(map[string]string{"TWOCAPTCHA_API_KEY": "key", "TWOCAPTCHAMCP_TIMEOUT": "not-a-duration"}))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestLoadCustomValues(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"TWOCAPTCHA_API_KEY":        "key",
		"TWOCAPTCHA_BASE_URL":       "https://example.test",
		"TWOCAPTCHA_SOFT_ID":        "42",
		"TWOCAPTCHAMCP_MAX_RETRIES": "3",
		"TWOCAPTCHAMCP_TIMEOUT":     "90s",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BaseURL != "https://example.test" || cfg.SoftID != 42 || cfg.MaxRetries != 3 || cfg.Timeout != 90*time.Second {
		t.Errorf("unexpected config: %+v", cfg)
	}
}
