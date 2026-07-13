package config

import (
	"strings"
	"testing"
	"time"
)

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("FLOWAI_HTTP_ADDR", "127.0.0.1:3001")
	t.Setenv("FLOWAI_AI_GRPC_ADDR", "127.0.0.1:50051")
	t.Setenv("FLOWAI_SANDBOX_GRPC_ADDR", "127.0.0.1:50052")
	t.Setenv("FLOWAI_GRPC_TOKEN", "ttttttttttttttttttttttttttttttttttttttttttt")
	t.Setenv("FLOWAI_JWT_SECRET", strings.Repeat("j", 32))
	t.Setenv("FLOWAI_API_KEY_HMAC_SECRET", strings.Repeat("k", 32))
	t.Setenv("FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET", "")
	t.Setenv("FLOWAI_FRONTEND_URL", "http://127.0.0.1:5173")
	t.Setenv("FLOWAI_JWT_EXPIRATION", "")
	t.Setenv("FLOWAI_CONTROL_DATABASE_URL", "postgres://user:pass@127.0.0.1/db")
	t.Setenv("FLOWAI_REDIS_URL", "redis://127.0.0.1:6379/0")
}

func TestLoadAcceptsNativeLoopbackConfiguration(t *testing.T) {
	setValidEnvironment(t)

	settings, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if settings.HTTPAddress != "127.0.0.1:3001" {
		t.Fatalf("HTTPAddress = %q", settings.HTTPAddress)
	}
	if settings.HealthTimeout.String() != "2s" {
		t.Fatalf("HealthTimeout = %s", settings.HealthTimeout)
	}
	if settings.JWTExpiration != 7*24*time.Hour {
		t.Fatalf("JWTExpiration = %s", settings.JWTExpiration)
	}
	if settings.FrontendURL != "http://127.0.0.1:5173" {
		t.Fatalf("FrontendURL = %q", settings.FrontendURL)
	}
}

func TestLoadRejectsMissingServiceToken(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("FLOWAI_GRPC_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error for a missing service token")
	}
}

func TestLoadRejectsMissingApplicationSecrets(t *testing.T) {
	tests := []string{
		"FLOWAI_JWT_SECRET",
		"FLOWAI_API_KEY_HMAC_SECRET",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv(name, "")
			if _, err := Load(); err == nil {
				t.Fatalf("Load() expected an error for %s", name)
			}
		})
	}
}

func TestLoadRejectsReusedSecrets(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("FLOWAI_JWT_SECRET", strings.Repeat("t", 43))

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error when JWT and gRPC secrets match")
	}

	setValidEnvironment(t)
	t.Setenv("FLOWAI_API_KEY_HMAC_SECRET", strings.Repeat("j", 32))
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error when JWT and API key secrets match")
	}
}

func TestLoadValidatesOptionalPreviousAPIKeySecret(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET", "short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error for a short previous API key secret")
	}
}

func TestLoadValidatesFrontendURLAndJWTExpiration(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("FLOWAI_FRONTEND_URL", "file:///tmp/frontend")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error for a non-HTTP frontend URL")
	}

	setValidEnvironment(t)
	t.Setenv("FLOWAI_JWT_EXPIRATION", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error for a non-positive JWT expiration")
	}
}

func TestLoadRejectsNonLoopbackInternalServices(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{name: "ai runtime", env: "FLOWAI_AI_GRPC_ADDR"},
		{name: "sandbox", env: "FLOWAI_SANDBOX_GRPC_ADDR"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv(test.env, "0.0.0.0:50051")
			if _, err := Load(); err == nil {
				t.Fatalf("Load() expected an error for %s", test.env)
			}
		})
	}
}
