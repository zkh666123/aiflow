package config

import "testing"

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("FLOWAI_HTTP_ADDR", "127.0.0.1:3001")
	t.Setenv("FLOWAI_AI_GRPC_ADDR", "127.0.0.1:50051")
	t.Setenv("FLOWAI_SANDBOX_GRPC_ADDR", "127.0.0.1:50052")
	t.Setenv("FLOWAI_GRPC_TOKEN", "ttttttttttttttttttttttttttttttttttttttttttt")
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
}

func TestLoadRejectsMissingServiceToken(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("FLOWAI_GRPC_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error for a missing service token")
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
