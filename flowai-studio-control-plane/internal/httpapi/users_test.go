package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

type stubUserOperations struct {
	user          auth.User
	login         auth.LoginResult
	err           error
	registerInput auth.RegisterInput
	loginInput    auth.LoginInput
	updateInput   auth.UpdateProfileInput
}

func (operations *stubUserOperations) Register(_ context.Context, input auth.RegisterInput) (auth.User, error) {
	operations.registerInput = input
	return operations.user, operations.err
}

func (operations *stubUserOperations) Login(_ context.Context, input auth.LoginInput) (auth.LoginResult, error) {
	operations.loginInput = input
	return operations.login, operations.err
}

func (operations *stubUserOperations) Profile(context.Context, string) (auth.User, error) {
	return operations.user, operations.err
}

func (operations *stubUserOperations) UpdateProfile(
	_ context.Context,
	_ string,
	input auth.UpdateProfileInput,
) (auth.User, error) {
	operations.updateInput = input
	user := operations.user
	if input.Username != nil {
		user.Username = *input.Username
	}
	if input.AvatarSet {
		user.Avatar = input.Avatar
	}
	return user, operations.err
}

func testUser() auth.User {
	avatar := "https://example.test/avatar.png"
	return auth.User{
		ID:        "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		Username:  "alice",
		Avatar:    &avatar,
		CreatedAt: time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC),
	}
}

func userRouter(operations UserOperations) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	verifier := &stubTokenVerifier{principal: auth.Principal{
		UserID:   "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		Username: "alice",
	}}
	RegisterUserRoutes(router, NewUserHandler(operations), verifier)
	return router
}

func performJSONRequest(t *testing.T, router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func responseData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", response["data"])
	}
	return data
}

func TestUserRegisterAndLoginPreservePOST201AndPayloads(t *testing.T) {
	user := testUser()
	operations := &stubUserOperations{user: user, login: auth.LoginResult{User: user, Token: "signed-token"}}
	router := userRouter(operations)

	register := performJSONRequest(t, router, http.MethodPost, "/api/users/register", `{"username":"alice","password":"secret1"}`, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", register.Code, register.Body.String())
	}
	registerData := responseData(t, register)
	if registerData["id"] != user.ID || registerData["username"] != "alice" || registerData["createdAt"] == nil {
		t.Fatalf("register data = %#v", registerData)
	}
	if _, exists := registerData["updatedAt"]; exists {
		t.Fatalf("register leaked updatedAt: %#v", registerData)
	}

	login := performJSONRequest(t, router, http.MethodPost, "/api/users/login", `{"username":"alice","password":"secret1"}`, "")
	if login.Code != http.StatusCreated {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	loginData := responseData(t, login)
	if loginData["token"] != "signed-token" {
		t.Fatalf("login data = %#v", loginData)
	}
	loginUser := loginData["user"].(map[string]any)
	if len(loginUser) != 2 || loginUser["id"] != user.ID || loginUser["username"] != "alice" {
		t.Fatalf("login user = %#v", loginUser)
	}
}

func TestUserProfileRoutesRequireJWTAndPreserveFieldShapes(t *testing.T) {
	user := testUser()
	operations := &stubUserOperations{user: user}
	router := userRouter(operations)

	unauthorized := performJSONRequest(t, router, http.MethodGet, "/api/users/profile", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	profile := performJSONRequest(t, router, http.MethodGet, "/api/users/profile", "", "valid")
	if profile.Code != http.StatusOK {
		t.Fatalf("profile status = %d, body = %s", profile.Code, profile.Body.String())
	}
	profileData := responseData(t, profile)
	if profileData["createdAt"] == nil || profileData["avatar"] == nil {
		t.Fatalf("profile data = %#v", profileData)
	}
	if _, exists := profileData["updatedAt"]; exists {
		t.Fatalf("profile leaked updatedAt: %#v", profileData)
	}

	updated := performJSONRequest(t, router, http.MethodPatch, "/api/users/profile", `{"username":"alice","avatar":null}`, "valid")
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updated.Code, updated.Body.String())
	}
	updatedData := responseData(t, updated)
	if len(updatedData) != 3 || updatedData["id"] != user.ID || updatedData["avatar"] != nil {
		t.Fatalf("updated data = %#v", updatedData)
	}
	if !operations.updateInput.AvatarSet || operations.updateInput.Avatar != nil {
		t.Fatalf("update input = %#v", operations.updateInput)
	}
}

func TestUserRoutesRejectUnknownFieldsAndMapServiceErrors(t *testing.T) {
	router := userRouter(&stubUserOperations{})
	invalid := performJSONRequest(t, router, http.MethodPost, "/api/users/register", `{"username":"alice","password":"secret1","admin":true}`, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}

	tests := []struct {
		kind   auth.ErrorKind
		status int
		code   string
	}{
		{kind: auth.ErrorInvalidInput, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{kind: auth.ErrorUnauthorized, status: http.StatusUnauthorized, code: "UNAUTHORIZED"},
		{kind: auth.ErrorConflict, status: http.StatusConflict, code: "CONFLICT"},
		{kind: auth.ErrorInternal, status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}
	for _, test := range tests {
		operations := &stubUserOperations{err: &auth.ServiceError{Kind: test.kind, Message: "public message", Cause: errors.New("private")}}
		recorder := performJSONRequest(t, userRouter(operations), http.MethodPost, "/api/users/register", `{"username":"alice","password":"secret1"}`, "")
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("kind %s: status = %d, body = %s", test.kind, recorder.Code, recorder.Body.String())
		}
		if test.kind == auth.ErrorInternal && strings.Contains(recorder.Body.String(), "public message") {
			t.Fatalf("internal detail leaked: %s", recorder.Body.String())
		}
	}
}
