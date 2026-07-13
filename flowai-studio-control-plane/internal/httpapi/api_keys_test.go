package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/apikeys"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

type stubAPIKeyOperations struct {
	created           apikeys.CreatedAPIKey
	list              []apikeys.APIKey
	toggled           apikeys.APIKey
	err               error
	createInput       apikeys.CreateInput
	listApplicationID *string
	deletedUserID     string
	deletedKeyID      string
	toggledUserID     string
	toggledKeyID      string
	toggledActive     bool
	calls             int
}

func (operations *stubAPIKeyOperations) Create(_ context.Context, _ string, input apikeys.CreateInput) (apikeys.CreatedAPIKey, error) {
	operations.calls++
	operations.createInput = input
	return operations.created, operations.err
}

func (operations *stubAPIKeyOperations) List(_ context.Context, _ string, applicationID *string) ([]apikeys.APIKey, error) {
	operations.calls++
	operations.listApplicationID = applicationID
	return operations.list, operations.err
}

func (operations *stubAPIKeyOperations) Delete(_ context.Context, userID, keyID string) error {
	operations.calls++
	operations.deletedUserID = userID
	operations.deletedKeyID = keyID
	return operations.err
}

func (operations *stubAPIKeyOperations) Toggle(_ context.Context, userID, keyID string, active bool) (apikeys.APIKey, error) {
	operations.calls++
	operations.toggledUserID = userID
	operations.toggledKeyID = keyID
	operations.toggledActive = active
	return operations.toggled, operations.err
}

func apiKeyRouter(operations APIKeyOperations) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAPIKeyRoutes(router, NewAPIKeyHandler(operations), &stubTokenVerifier{principal: auth.Principal{
		UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice",
	}})
	return router
}

func TestAPIKeyRoutesPreserveLegacyContractsAndNeverExposeSecrets(t *testing.T) {
	key := sampleAPIKeyHTTP()
	rawKey := "sk-" + strings.Repeat("a", 64)
	operations := &stubAPIKeyOperations{
		created: apikeys.CreatedAPIKey{APIKey: key, Key: rawKey},
		list:    []apikeys.APIKey{key},
		toggled: key,
	}
	router := apiKeyRouter(operations)

	created := performJSONRequest(t, router, http.MethodPost, "/api/api-keys", `{
		"name":"deploy",
		"applicationId":"7a611d9a-b555-4469-a289-f1672daefce3",
		"scopes":["app:read","workflow:execute"],
		"expiresAt":"2026-07-14T17:00:00Z"
	}`, "valid")
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	createdData := responseData(t, created)
	if createdData["key"] != rawKey || createdData["keyPrefix"] != "sk-aaaa" || createdData["applicationId"] != "7a611d9a-b555-4469-a289-f1672daefce3" {
		t.Fatalf("create data = %#v", createdData)
	}
	if _, exists := createdData["keyHash"]; exists {
		t.Fatalf("create leaked keyHash: %#v", createdData)
	}
	if operations.createInput.ApplicationID == nil || operations.createInput.ExpiresAt == nil || len(operations.createInput.Scopes) != 2 {
		t.Fatalf("create input = %#v", operations.createInput)
	}
	assertSuccessEnvelope(t, created.Body.Bytes())

	listed := performJSONRequest(t, router, http.MethodGet, "/api/api-keys?applicationId=7a611d9a-b555-4469-a289-f1672daefce3", "", "valid")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Code, listed.Body.String())
	}
	var listedEnvelope map[string]any
	if err := json.Unmarshal(listed.Body.Bytes(), &listedEnvelope); err != nil {
		t.Fatal(err)
	}
	listedData := listedEnvelope["data"].([]any)[0].(map[string]any)
	for _, forbidden := range []string{"key", "digest", "keyHash"} {
		if _, exists := listedData[forbidden]; exists {
			t.Fatalf("list exposed %s: %#v", forbidden, listedData)
		}
	}
	if listedData["keyPrefix"] != "sk-aaaa" || listedData["isActive"] != true || operations.listApplicationID == nil {
		t.Fatalf("list data = %#v, application ID = %#v", listedData, operations.listApplicationID)
	}

	deleted := performJSONRequest(t, router, http.MethodDelete, "/api/api-keys/4ab1ea39-b2a4-4850-b719-ae5ad57773f1", "", "valid")
	if deleted.Code != http.StatusOK || responseData(t, deleted)["success"] != true {
		t.Fatalf("delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}
	if operations.deletedUserID != "e9f6332d-da39-44b2-917c-da5ff30aca9d" || operations.deletedKeyID != "4ab1ea39-b2a4-4850-b719-ae5ad57773f1" {
		t.Fatalf("delete owner/key = %q/%q", operations.deletedUserID, operations.deletedKeyID)
	}

	toggled := performJSONRequest(t, router, http.MethodPatch, "/api/api-keys/4ab1ea39-b2a4-4850-b719-ae5ad57773f1/toggle", `{"isActive":false}`, "valid")
	if toggled.Code != http.StatusOK || operations.toggledActive {
		t.Fatalf("toggle status = %d, body = %s", toggled.Code, toggled.Body.String())
	}
	toggledData := responseData(t, toggled)
	if len(toggledData) != 3 || toggledData["id"] != key.ID || toggledData["name"] != key.Name || toggledData["isActive"] != true {
		t.Fatalf("toggle data = %#v", toggledData)
	}
}

func TestAPIKeyRoutesRequireJWTAndStrictlyValidateInput(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
		status int
	}{
		{name: "JWT required", method: http.MethodGet, path: "/api/api-keys", status: http.StatusUnauthorized},
		{name: "unknown JSON field", method: http.MethodPost, path: "/api/api-keys", body: `{"name":"key","rawKey":"secret"}`, token: "valid", status: http.StatusBadRequest},
		{name: "invalid application UUID", method: http.MethodPost, path: "/api/api-keys", body: `{"name":"key","applicationId":"not-a-uuid"}`, token: "valid", status: http.StatusBadRequest},
		{name: "invalid expiration", method: http.MethodPost, path: "/api/api-keys", body: `{"name":"key","expiresAt":"tomorrow"}`, token: "valid", status: http.StatusBadRequest},
		{name: "unknown scope", method: http.MethodPost, path: "/api/api-keys", body: `{"name":"key","scopes":["admin:all"]}`, token: "valid", status: http.StatusBadRequest},
		{name: "duplicate scope", method: http.MethodPost, path: "/api/api-keys", body: `{"name":"key","scopes":["app:read","app:read"]}`, token: "valid", status: http.StatusBadRequest},
		{name: "invalid filter UUID", method: http.MethodGet, path: "/api/api-keys?applicationId=invalid", token: "valid", status: http.StatusBadRequest},
		{name: "invalid delete UUID", method: http.MethodDelete, path: "/api/api-keys/invalid", token: "valid", status: http.StatusBadRequest},
		{name: "missing active flag", method: http.MethodPatch, path: "/api/api-keys/4ab1ea39-b2a4-4850-b719-ae5ad57773f1/toggle", body: `{}`, token: "valid", status: http.StatusBadRequest},
		{name: "wrong active type", method: http.MethodPatch, path: "/api/api-keys/4ab1ea39-b2a4-4850-b719-ae5ad57773f1/toggle", body: `{"isActive":"false"}`, token: "valid", status: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operations := &stubAPIKeyOperations{}
			recorder := performJSONRequest(t, apiKeyRouter(operations), test.method, test.path, test.body, test.token)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if test.status == http.StatusBadRequest && operations.calls != 0 {
				t.Fatalf("operations called %d times for invalid input", operations.calls)
			}
		})
	}
}

func TestAPIKeyCreatePreservesExplicitEmptyScopes(t *testing.T) {
	operations := &stubAPIKeyOperations{created: apikeys.CreatedAPIKey{APIKey: sampleAPIKeyHTTP(), Key: "sk-" + strings.Repeat("a", 64)}}
	recorder := performJSONRequest(t, apiKeyRouter(operations), http.MethodPost, "/api/api-keys", `{"name":"no-access","scopes":[]}`, "valid")
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if operations.createInput.Scopes == nil || len(operations.createInput.Scopes) != 0 {
		t.Fatalf("create scopes = %#v", operations.createInput.Scopes)
	}
}

func TestAPIKeyRoutesMapServiceErrorsWithoutLeakingCredentials(t *testing.T) {
	tests := []struct {
		kind   apikeys.ErrorKind
		status int
		code   string
	}{
		{kind: apikeys.ErrorInvalidInput, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{kind: apikeys.ErrorForbidden, status: http.StatusForbidden, code: "FORBIDDEN"},
		{kind: apikeys.ErrorNotFound, status: http.StatusNotFound, code: "NOT_FOUND"},
		{kind: apikeys.ErrorInternal, status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}
	for _, test := range tests {
		operations := &stubAPIKeyOperations{err: &apikeys.ServiceError{
			Kind: test.kind, Message: "public", Cause: errors.New("private sk-credential"),
		}}
		recorder := performJSONRequest(t, apiKeyRouter(operations), http.MethodGet, "/api/api-keys", "", "valid")
		if recorder.Code != test.status || !containsJSONCode(recorder.Body.Bytes(), test.code) {
			t.Fatalf("kind %s: status = %d, body = %s", test.kind, recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), "credential") || strings.Contains(recorder.Body.String(), "private") {
			t.Fatalf("credential leaked: %s", recorder.Body.String())
		}
	}
}

func sampleAPIKeyHTTP() apikeys.APIKey {
	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	applicationID := "7a611d9a-b555-4469-a289-f1672daefce3"
	return apikeys.APIKey{
		ID: "4ab1ea39-b2a4-4850-b719-ae5ad57773f1", Name: "deploy", KeyPrefix: "sk-aaaa",
		Scopes: []apikeys.Scope{apikeys.ScopeAppRead, apikeys.ScopeWorkflowExecute}, IsActive: true,
		LastUsedAt: &now, ExpiresAt: &expiresAt, ApplicationID: &applicationID, CreatedAt: now,
	}
}

func assertSuccessEnvelope(t *testing.T, body []byte) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["success"] != true || envelope["code"] != "SUCCESS" || envelope["message"] != "Success" || envelope["timestamp"] == nil {
		t.Fatalf("envelope = %#v", envelope)
	}
}
