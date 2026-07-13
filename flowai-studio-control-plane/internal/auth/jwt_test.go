package auth

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJWTServiceSignsAndVerifiesLegacyClaims(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	service, err := NewJWTService(strings.Repeat("j", 32), 7*24*time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}

	token, err := service.Sign(Principal{UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice"})
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	principal, err := service.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if principal.UserID != "e9f6332d-da39-44b2-917c-da5ff30aca9d" || principal.Username != "alice" {
		t.Fatalf("principal = %#v", principal)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims["userId"] != principal.UserID || claims["username"] != principal.Username {
		t.Fatalf("legacy claims missing: %#v", claims)
	}
	if got := int64(claims["exp"].(float64) - claims["iat"].(float64)); got != int64((7 * 24 * time.Hour).Seconds()) {
		t.Fatalf("token lifetime = %d seconds", got)
	}
}

func TestJWTServiceRejectsExpiredTamperedAndWrongAlgorithmTokens(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	secret := strings.Repeat("j", 32)
	service, err := NewJWTService(secret, time.Minute, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}
	token, err := service.Sign(Principal{UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice"})
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	service.clock = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := service.Verify(token); err == nil {
		t.Fatal("Verify() accepted an expired token")
	}

	service.clock = func() time.Time { return now }
	parts := strings.Split(token, ".")
	tampered := strings.Join([]string{parts[0], parts[1], "invalid"}, ".")
	if _, err := service.Verify(tampered); err == nil {
		t.Fatal("Verify() accepted a tampered token")
	}

	wrongAlgorithm := signHS512Token(t, secret, map[string]any{
		"userId":   "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		"username": "alice",
		"iat":      now.Unix(),
		"exp":      now.Add(time.Minute).Unix(),
	})
	if _, err := service.Verify(wrongAlgorithm); err == nil {
		t.Fatal("Verify() accepted a non-HS256 token")
	}

	if _, err := service.Verify("not-a-jwt"); err == nil {
		t.Fatal("Verify() accepted a malformed token")
	}
}

func signHS512Token(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS512", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
