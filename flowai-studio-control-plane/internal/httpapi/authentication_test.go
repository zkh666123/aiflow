package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

type stubTokenVerifier struct {
	principal auth.Principal
	err       error
	token     string
}

func (verifier *stubTokenVerifier) Verify(token string) (auth.Principal, error) {
	verifier.token = token
	return verifier.principal, verifier.err
}

func TestAuthenticationRejectsMissingAndMalformedBearerTokens(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "basic", header: "Basic abc"},
		{name: "empty bearer", header: "Bearer "},
		{name: "extra segment", header: "Bearer one two"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			verifier := &stubTokenVerifier{}
			router := gin.New()
			router.Use(Authentication(verifier))
			router.GET("/protected", func(c *gin.Context) { WriteSuccess(c, http.StatusOK, true) })
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if verifier.token != "" {
				t.Fatalf("verifier received malformed token %q", verifier.token)
			}
		})
	}
}

func TestAuthenticationRejectsInvalidTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := &stubTokenVerifier{err: errors.New("invalid")}
	router := gin.New()
	router.Use(Authentication(verifier))
	router.GET("/protected", func(c *gin.Context) { WriteSuccess(c, http.StatusOK, true) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized || verifier.token != "invalid-token" {
		t.Fatalf("status = %d, token = %q", recorder.Code, verifier.token)
	}
}

func TestAuthenticationStoresTheVerifiedPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	want := auth.Principal{UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice"}
	verifier := &stubTokenVerifier{principal: want}
	router := gin.New()
	router.Use(Authentication(verifier))
	router.GET("/protected", func(c *gin.Context) {
		principal, ok := CurrentPrincipal(c)
		if !ok || principal != want {
			t.Fatalf("principal = %#v, ok = %v", principal, ok)
		}
		WriteSuccess(c, http.StatusOK, principal)
	})
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || verifier.token != "valid-token" {
		t.Fatalf("status = %d, token = %q, body = %s", recorder.Code, verifier.token, recorder.Body.String())
	}
}
