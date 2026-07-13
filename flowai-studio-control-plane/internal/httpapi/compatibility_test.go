package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type strictBody struct {
	Name string `json:"name"`
}

func TestWriteSuccessUsesTheFrozenEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	WriteSuccess(context, http.StatusCreated, map[string]string{"id": "created"})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["success"] != true || response["code"] != "SUCCESS" || response["message"] != "Success" {
		t.Fatalf("response = %#v", response)
	}
	if response["timestamp"] == nil || response["data"].(map[string]any)["id"] != "created" {
		t.Fatalf("response = %#v", response)
	}
}

func TestWriteErrorUsesStableCodesAndHidesInternalCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/apps?x=1", nil)

	WriteError(context, &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL_ERROR",
		Message: "Internal server error",
		Cause:   errors.New("database password leaked in stack"),
	})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "database password") {
		t.Fatalf("internal cause leaked: %s", recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["success"] != false || response["data"] != nil || response["path"] != "/api/apps?x=1" {
		t.Fatalf("response = %#v", response)
	}
}

func TestDecodeJSONRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"name":"ok","extra":true}`},
		{name: "trailing value", body: `{"name":"ok"}{"name":"second"}`},
		{name: "empty body", body: ``},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(test.body))
			context.Request.Header.Set("Content-Type", "application/json")

			if _, apiErr := DecodeJSON[strictBody](context); apiErr == nil || apiErr.Status != http.StatusBadRequest || apiErr.Code != "BAD_REQUEST" {
				t.Fatalf("DecodeJSON() error = %#v", apiErr)
			}
		})
	}
}

func TestDecodeJSONAcceptsOneStrictObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"name":"ok"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	body, apiErr := DecodeJSON[strictBody](context)
	if apiErr != nil || body.Name != "ok" {
		t.Fatalf("body = %#v, error = %#v", body, apiErr)
	}
}

func TestRecoveryMiddlewareReturnsACompatibleInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RecoveryMiddleware())
	router.GET("/api/panic", func(*gin.Context) { panic("secret stack detail") })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret stack detail") {
		t.Fatalf("panic detail leaked: %s", recorder.Body.String())
	}
}
