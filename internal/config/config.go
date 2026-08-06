package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/alex-oleshkevich/twocaptchamcp/internal/captcha"
)

const defaultAddress = "127.0.0.1:8080"

type Config struct {
	APIKey     string
	Address    string
	Token      string
	BaseURL    string
	SoftID     int
	MaxRetries int
	Timeout    time.Duration
}

func FromEnvironment() (Config, error) {
	return Load(os.Getenv)
}

func Load(getenv func(string) string) (Config, error) {
	address := valueOr(getenv("TWOCAPTCHAMCP_ADDRESS"), defaultAddress)
	if err := validateAddress(address); err != nil {
		return Config{}, err
	}

	softID, err := parseOptionalInt(getenv("TWOCAPTCHA_SOFT_ID"))
	if err != nil {
		return Config{}, errors.New("TWOCAPTCHA_SOFT_ID must be an integer")
	}

	maxRetries := captcha.DefaultRetries
	if raw := getenv("TWOCAPTCHAMCP_MAX_RETRIES"); raw != "" {
		maxRetries, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, errors.New("TWOCAPTCHAMCP_MAX_RETRIES must be an integer")
		}
	}

	timeout := captcha.DefaultTimeout
	if raw := getenv("TWOCAPTCHAMCP_TIMEOUT"); raw != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return Config{}, errors.New("TWOCAPTCHAMCP_TIMEOUT must be a duration (e.g. 180s)")
		}
	}

	cfg := Config{
		APIKey:     getenv("TWOCAPTCHA_API_KEY"),
		Address:    address,
		Token:      getenv("TWOCAPTCHAMCP_TOKEN"),
		BaseURL:    valueOr(getenv("TWOCAPTCHA_BASE_URL"), captcha.DefaultBaseURL),
		SoftID:     softID,
		MaxRetries: maxRetries,
		Timeout:    timeout,
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("TWOCAPTCHA_API_KEY is required")
	}
	if !isLoopbackAddress(cfg.Address) && cfg.Token == "" {
		return Config{}, errors.New("TWOCAPTCHAMCP_TOKEN is required for a non-loopback address")
	}
	return cfg, nil
}

func validateAddress(address string) error {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return errors.New("TWOCAPTCHAMCP_ADDRESS must be a host:port address")
	}
	return nil
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseOptionalInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
