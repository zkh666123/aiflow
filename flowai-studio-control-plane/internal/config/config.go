package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

type Settings struct {
	HTTPAddress              string
	AIAddress                string
	SandboxAddress           string
	GRPCToken                string
	JWTSecret                string
	APIKeyHMACSecret         string
	APIKeyHMACPreviousSecret string
	DatabaseURL              string
	RedisURL                 string
	FrontendURL              string
	JWTExpiration            time.Duration
	HealthTimeout            time.Duration
	ShutdownTimeout          time.Duration
}

func Load() (Settings, error) {
	settings := Settings{
		HTTPAddress:              envOrDefault("FLOWAI_HTTP_ADDR", "127.0.0.1:3001"),
		AIAddress:                envOrDefault("FLOWAI_AI_GRPC_ADDR", "127.0.0.1:50051"),
		SandboxAddress:           envOrDefault("FLOWAI_SANDBOX_GRPC_ADDR", "127.0.0.1:50052"),
		GRPCToken:                os.Getenv("FLOWAI_GRPC_TOKEN"),
		JWTSecret:                os.Getenv("FLOWAI_JWT_SECRET"),
		APIKeyHMACSecret:         os.Getenv("FLOWAI_API_KEY_HMAC_SECRET"),
		APIKeyHMACPreviousSecret: os.Getenv("FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET"),
		DatabaseURL:              os.Getenv("FLOWAI_CONTROL_DATABASE_URL"),
		RedisURL:                 os.Getenv("FLOWAI_REDIS_URL"),
		FrontendURL:              strings.TrimRight(envOrDefault("FLOWAI_FRONTEND_URL", "http://127.0.0.1:5173"), "/"),
		JWTExpiration:            7 * 24 * time.Hour,
		HealthTimeout:            2 * time.Second,
		ShutdownTimeout:          5 * time.Second,
	}

	if err := validateRequiredSecret("FLOWAI_GRPC_TOKEN", settings.GRPCToken); err != nil {
		return Settings{}, err
	}
	if err := validateRequiredSecret("FLOWAI_JWT_SECRET", settings.JWTSecret); err != nil {
		return Settings{}, err
	}
	if err := validateRequiredSecret("FLOWAI_API_KEY_HMAC_SECRET", settings.APIKeyHMACSecret); err != nil {
		return Settings{}, err
	}
	if settings.APIKeyHMACPreviousSecret != "" {
		if err := validateRequiredSecret("FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET", settings.APIKeyHMACPreviousSecret); err != nil {
			return Settings{}, err
		}
	}
	secrets := []struct {
		name  string
		value string
	}{
		{name: "FLOWAI_GRPC_TOKEN", value: settings.GRPCToken},
		{name: "FLOWAI_JWT_SECRET", value: settings.JWTSecret},
		{name: "FLOWAI_API_KEY_HMAC_SECRET", value: settings.APIKeyHMACSecret},
	}
	if settings.APIKeyHMACPreviousSecret != "" {
		secrets = append(secrets, struct {
			name  string
			value string
		}{name: "FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET", value: settings.APIKeyHMACPreviousSecret})
	}
	for left := range secrets {
		for right := left + 1; right < len(secrets); right++ {
			if secrets[left].value == secrets[right].value {
				return Settings{}, fmt.Errorf("%s and %s must use different secrets", secrets[left].name, secrets[right].name)
			}
		}
	}
	if settings.DatabaseURL == "" {
		return Settings{}, fmt.Errorf("FLOWAI_CONTROL_DATABASE_URL is required")
	}
	if settings.RedisURL == "" {
		return Settings{}, fmt.Errorf("FLOWAI_REDIS_URL is required")
	}
	if err := validateLoopbackAddress("FLOWAI_AI_GRPC_ADDR", settings.AIAddress); err != nil {
		return Settings{}, err
	}
	if err := validateLoopbackAddress("FLOWAI_SANDBOX_GRPC_ADDR", settings.SandboxAddress); err != nil {
		return Settings{}, err
	}
	if err := validateFrontendURL(settings.FrontendURL); err != nil {
		return Settings{}, err
	}

	if value := os.Getenv("FLOWAI_JWT_EXPIRATION"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 || duration > 30*24*time.Hour {
			return Settings{}, fmt.Errorf("FLOWAI_JWT_EXPIRATION must be between 0 and 720 hours")
		}
		settings.JWTExpiration = duration
	}

	if value := os.Getenv("FLOWAI_HEALTH_TIMEOUT"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 || duration > 30*time.Second {
			return Settings{}, fmt.Errorf("FLOWAI_HEALTH_TIMEOUT must be between 0 and 30 seconds")
		}
		settings.HealthTimeout = duration
	}

	return settings, nil
}

func validateRequiredSecret(name, value string) error {
	if len(value) < 32 || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must contain at least 32 non-blank characters", name)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func validateLoopbackAddress(name, address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return fmt.Errorf("%s must use loopback host:port", name)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%s must use a loopback address", name)
	}
	return nil
}

func validateFrontendURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return fmt.Errorf("FLOWAI_FRONTEND_URL must be an absolute HTTP(S) URL without credentials")
	}
	return nil
}
