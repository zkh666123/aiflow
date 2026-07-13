package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/applications"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

type stubApplicationOperations struct {
	application applications.Application
	list        []applications.Application
	err         error
	created     applications.CreateInput
	updated     applications.UpdateInput
	deletedID   string
	operation   string
}

func (operations *stubApplicationOperations) Create(_ context.Context, _ string, input applications.CreateInput) (applications.Application, error) {
	operations.created = input
	return operations.application, operations.err
}

func (operations *stubApplicationOperations) List(context.Context, string) ([]applications.Application, error) {
	return operations.list, operations.err
}

func (operations *stubApplicationOperations) Get(context.Context, string, string) (applications.Application, error) {
	return operations.application, operations.err
}

func (operations *stubApplicationOperations) Update(_ context.Context, _ string, _ string, input applications.UpdateInput) (applications.Application, error) {
	operations.updated = input
	return operations.application, operations.err
}

func (operations *stubApplicationOperations) Delete(_ context.Context, _ string, id string) error {
	operations.deletedID = id
	return operations.err
}

func (operations *stubApplicationOperations) Publish(context.Context, string, string) (applications.Application, error) {
	operations.operation = "publish"
	application := operations.application
	application.Status = applications.StatusPublished
	return application, operations.err
}

func (operations *stubApplicationOperations) Unpublish(context.Context, string, string) (applications.Application, error) {
	operations.operation = "unpublish"
	application := operations.application
	application.Status = applications.StatusDraft
	return application, operations.err
}

func (operations *stubApplicationOperations) Archive(context.Context, string, string) (applications.Application, error) {
	operations.operation = "archive"
	application := operations.application
	application.Status = applications.StatusArchived
	return application, operations.err
}

func (operations *stubApplicationOperations) Unarchive(context.Context, string, string) (applications.Application, error) {
	operations.operation = "unarchive"
	application := operations.application
	application.Status = applications.StatusDraft
	return application, operations.err
}

func applicationRouter(operations ApplicationOperations) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	verifier := &stubTokenVerifier{principal: auth.Principal{
		UserID:   "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		Username: "alice",
	}}
	RegisterApplicationRoutes(router, NewApplicationHandler(operations), verifier)
	return router
}

func TestApplicationCRUDRoutesPreserveStatusAndPayloadShapes(t *testing.T) {
	application := sampleApplicationHTTP()
	operations := &stubApplicationOperations{application: application, list: []applications.Application{application}}
	router := applicationRouter(operations)

	create := performJSONRequest(t, router, http.MethodPost, "/api/apps", `{"name":"App","description":"description","icon":"icon"}`, "valid")
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", create.Code, create.Body.String())
	}
	createData := responseData(t, create)
	if createData["id"] != application.ID || createData["accessType"] != nil || createData["userId"] != nil {
		t.Fatalf("create data = %#v", createData)
	}

	list := performJSONRequest(t, router, http.MethodGet, "/api/apps", "", "valid")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}
	var listEnvelope map[string]any
	if err := json.Unmarshal(list.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatal(err)
	}
	listed := listEnvelope["data"].([]any)[0].(map[string]any)
	if listed["accessType"] != "owner" {
		t.Fatalf("listed = %#v", listed)
	}

	get := performJSONRequest(t, router, http.MethodGet, "/api/apps/"+application.ID, "", "valid")
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", get.Code, get.Body.String())
	}
	getData := responseData(t, get)
	if getData["userId"] != application.OwnerID || getData["workflows"] == nil || len(getData["workflows"].([]any)) != 0 {
		t.Fatalf("get data = %#v", getData)
	}

	update := performJSONRequest(t, router, http.MethodPatch, "/api/apps/"+application.ID, `{"name":"Updated","description":null}`, "valid")
	if update.Code != http.StatusOK || operations.updated.Name == nil || *operations.updated.Name != "Updated" || !operations.updated.DescriptionSet || operations.updated.Description != nil {
		t.Fatalf("update status = %d, input = %#v, body = %s", update.Code, operations.updated, update.Body.String())
	}

	deleted := performJSONRequest(t, router, http.MethodDelete, "/api/apps/"+application.ID, "", "valid")
	if deleted.Code != http.StatusOK || operations.deletedID != application.ID || responseData(t, deleted)["success"] != true {
		t.Fatalf("delete status = %d, deleted = %q, body = %s", deleted.Code, operations.deletedID, deleted.Body.String())
	}
}

func TestApplicationStatusRoutesReturnLegacyPartialPayloads(t *testing.T) {
	application := sampleApplicationHTTP()
	tests := []struct {
		path   string
		status applications.Status
	}{
		{path: "publish", status: applications.StatusPublished},
		{path: "unpublish", status: applications.StatusDraft},
		{path: "archive", status: applications.StatusArchived},
		{path: "unarchive", status: applications.StatusDraft},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			operations := &stubApplicationOperations{application: application}
			recorder := performJSONRequest(t, applicationRouter(operations), http.MethodPatch, "/api/apps/"+application.ID+"/"+test.path, "", "valid")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			data := responseData(t, recorder)
			if len(data) != 3 || data["id"] != application.ID || data["status"] != string(test.status) {
				t.Fatalf("data = %#v", data)
			}
		})
	}
}

func TestApplicationRoutesRequireJWTRejectUnknownFieldsAndMapErrors(t *testing.T) {
	application := sampleApplicationHTTP()
	unauthorized := performJSONRequest(t, applicationRouter(&stubApplicationOperations{application: application}), http.MethodPost, "/api/apps", `{"name":"App"}`, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	invalid := performJSONRequest(t, applicationRouter(&stubApplicationOperations{application: application}), http.MethodPost, "/api/apps", `{"name":"App","ownerId":"other"}`, "valid")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}

	tests := []struct {
		kind   applications.ErrorKind
		status int
		code   string
	}{
		{kind: applications.ErrorInvalidInput, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{kind: applications.ErrorForbidden, status: http.StatusForbidden, code: "FORBIDDEN"},
		{kind: applications.ErrorNotFound, status: http.StatusNotFound, code: "NOT_FOUND"},
		{kind: applications.ErrorInternal, status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}
	for _, test := range tests {
		operations := &stubApplicationOperations{
			application: application,
			err:         &applications.ServiceError{Kind: test.kind, Message: "public", Cause: errors.New("private")},
		}
		recorder := performJSONRequest(t, applicationRouter(operations), http.MethodGet, "/api/apps/"+application.ID, "", "valid")
		if recorder.Code != test.status || !containsJSONCode(recorder.Body.Bytes(), test.code) {
			t.Fatalf("kind %s: status = %d, body = %s", test.kind, recorder.Code, recorder.Body.String())
		}
		if test.kind == applications.ErrorInternal && strings.Contains(recorder.Body.String(), "public") {
			t.Fatalf("internal message leaked: %s", recorder.Body.String())
		}
	}
}

func sampleApplicationHTTP() applications.Application {
	application := sampleApplicationForHTTPBase()
	application.AccessType = "owner"
	return application
}

func sampleApplicationForHTTPBase() applications.Application {
	description := "description"
	icon := "icon"
	return applications.Application{
		ID:          "7a611d9a-b555-4469-a289-f1672daefce3",
		Name:        "App",
		Description: &description,
		Icon:        &icon,
		Status:      applications.StatusDraft,
		OwnerID:     "e9f6332d-da39-44b2-917c-da5ff30aca9d",
	}
}

func containsJSONCode(body []byte, code string) bool {
	var response map[string]any
	if json.Unmarshal(body, &response) != nil {
		return false
	}
	return response["code"] == code
}
