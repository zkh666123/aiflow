package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type Settings struct {
	HTTPAddress     string
	AIAddress       string
	SandboxAddress  string
	GRPCToken       string
	DatabaseURL     string
	RedisURL        string
	HealthTimeout   time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Settings, error) {
	settings := Settings{
		HTTPAddress:     envOrDefault("FLOWAI_HTTP_ADDR", "127.0.0.1:3001"),
		AIAddress:       envOrDefault("FLOWAI_AI_GRPC_ADDR", "127.0.0.1:50051"),
		SandboxAddress:  envOrDefault("FLOWAI_SANDBOX_GRPC_ADDR", "127.0.0.1:50052"),
		GRPCToken:       os.Getenv("FLOWAI_GRPC_TOKEN"),
		DatabaseURL:     os.Getenv("FLOWAI_CONTROL_DATABASE_URL"),
		RedisURL:        os.Getenv("FLOWAI_REDIS_URL"),
		HealthTimeout:   2 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	if len(settings.GRPCToken) < 32 || strings.TrimSpace(settings.GRPCToken) == "" {
		return Settings{}, fmt.Errorf("FLOWAI_GRPC_TOKEN must contain at least 32 non-blank characters")
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

	if value := os.Getenv("FLOWAI_HEALTH_TIMEOUT"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 || duration > 30*time.Second {
			return Settings{}, fmt.Errorf("FLOWAI_HEALTH_TIMEOUT must be between 0 and 30 seconds")
		}
		settings.HealthTimeout = duration
	}

	return settings, nil
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
