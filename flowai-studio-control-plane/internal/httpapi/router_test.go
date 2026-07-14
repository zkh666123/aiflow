package httpapi

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

func TestIdentityAccessRouterRegistersAll35Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(func(c *gin.Context) { WriteSuccess(c, http.StatusOK, true) })
	verifier := &stubTokenVerifier{principal: auth.Principal{UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice"}}
	RegisterUserRoutes(router, NewUserHandler(&stubUserOperations{}), verifier)
	RegisterApplicationRoutes(router, NewApplicationHandler(&stubApplicationOperations{}), verifier)
	RegisterTeamRoutes(router, NewTeamHandler(&stubTeamOperations{}), verifier)
	RegisterAPIKeyRoutes(router, NewAPIKeyHandler(&stubAPIKeyOperations{}), verifier)
	RegisterShareRoutes(router, NewShareHandler(&stubShareOperations{}), verifier)

	expected := map[string]struct{}{
		"POST /api/users/register": {}, "POST /api/users/login": {}, "GET /api/users/profile": {}, "PATCH /api/users/profile": {},
		"POST /api/apps": {}, "GET /api/apps": {}, "GET /api/apps/:id": {}, "PATCH /api/apps/:id": {}, "DELETE /api/apps/:id": {},
		"PATCH /api/apps/:id/publish": {}, "PATCH /api/apps/:id/unpublish": {}, "PATCH /api/apps/:id/archive": {}, "PATCH /api/apps/:id/unarchive": {},
		"POST /api/teams": {}, "GET /api/teams": {}, "GET /api/teams/:teamId": {}, "PATCH /api/teams/:teamId": {}, "DELETE /api/teams/:teamId": {},
		"POST /api/teams/:teamId/members": {}, "PATCH /api/teams/:teamId/members/:memberId": {}, "DELETE /api/teams/:teamId/members/:memberId": {},
		"POST /api/teams/:teamId/leave": {}, "POST /api/teams/:teamId/apps": {}, "PATCH /api/teams/:teamId/apps/:teamAppId": {}, "DELETE /api/teams/:teamId/apps/:teamAppId": {},
		"POST /api/api-keys": {}, "GET /api/api-keys": {}, "DELETE /api/api-keys/:keyId": {}, "PATCH /api/api-keys/:keyId/toggle": {},
		"POST /api/apps/:id/share": {}, "GET /api/apps/:id/share": {}, "PATCH /api/apps/:id/share": {}, "DELETE /api/apps/:id/share": {},
		"GET /api/apps/:id/embed": {}, "GET /api/share/:shareLink": {},
	}

	actual := map[string]struct{}{}
	for _, route := range router.Routes() {
		if route.Path == "/api/health" {
			continue
		}
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	if len(actual) != 35 {
		t.Fatalf("registered %d phase routes, want 35: %#v", len(actual), actual)
	}
	for route := range expected {
		if _, exists := actual[route]; !exists {
			t.Fatalf("missing route %s", route)
		}
	}
}
