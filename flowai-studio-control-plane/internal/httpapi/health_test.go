package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type staticChecker struct {
	result CheckResult
}

func (checker staticChecker) Check(context.Context) CheckResult {
	return checker.result
}

func performHealthRequest(t *testing.T, checkers map[string]Checker) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/health", NewHealthHandler(checkers, 100*time.Millisecond))

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func healthyCheckers() map[string]Checker {
	return map[string]Checker{
		"database":  staticChecker{result: CheckResult{Status: CheckStatusHealthy}},
		"redis":     staticChecker{result: CheckResult{Status: CheckStatusHealthy}},
		"pgvector":  staticChecker{result: CheckResult{Status: CheckStatusHealthy, Version: "0.8.5"}},
		"aiRuntime": staticChecker{result: CheckResult{Status: CheckStatusHealthy}},
		"sandbox":   staticChecker{result: CheckResult{Status: CheckStatusHealthy}},
	}
}

func TestHealthUsesTheFrozenEnvelopeAndAllDependencies(t *testing.T) {
	response := performHealthRequest(t, healthyCheckers())

	if response["success"] != true || response["code"] != "SUCCESS" || response["message"] != "Success" {
		t.Fatalf("unexpected envelope: %#v", response)
	}
	if _, ok := response["timestamp"].(string); !ok {
		t.Fatalf("outer timestamp missing: %#v", response)
	}
	data := response["data"].(map[string]any)
	if data["status"] != "healthy" {
		t.Fatalf("health status = %v", data["status"])
	}
	checks := data["checks"].(map[string]any)
	if len(checks) != 5 {
		t.Fatalf("checks = %#v", checks)
	}
}

func TestHealthReturnsADegradedBusinessStatusWithHTTP200(t *testing.T) {
	checkers := healthyCheckers()
	checkers["sandbox"] = staticChecker{
		result: CheckResult{Status: CheckStatusNotReady, Message: "runtime unavailable"},
	}

	response := performHealthRequest(t, checkers)
	data := response["data"].(map[string]any)
	if data["status"] != "degraded" {
		t.Fatalf("health status = %v", data["status"])
	}
	checks := data["checks"].(map[string]any)
	sandbox := checks["sandbox"].(map[string]any)
	if sandbox["status"] != "not_ready" {
		t.Fatalf("sandbox status = %v", sandbox["status"])
	}
}
