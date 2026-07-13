package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/apikeys"
)

type APIKeyOperations interface {
	Create(context.Context, string, apikeys.CreateInput) (apikeys.CreatedAPIKey, error)
	List(context.Context, string, *string) ([]apikeys.APIKey, error)
	Delete(context.Context, string, string) error
	Toggle(context.Context, string, string, bool) (apikeys.APIKey, error)
}

type APIKeyHandler struct {
	operations APIKeyOperations
}

func NewAPIKeyHandler(operations APIKeyOperations) *APIKeyHandler {
	return &APIKeyHandler{operations: operations}
}

func RegisterAPIKeyRoutes(router *gin.Engine, handler *APIKeyHandler, verifier TokenVerifier) {
	routes := router.Group("/api/api-keys")
	routes.Use(Authentication(verifier))
	routes.POST("", handler.Create)
	routes.GET("", handler.List)
	routes.DELETE("/:keyId", handler.Delete)
	routes.PATCH("/:keyId/toggle", handler.Toggle)
}

type optionalScopes struct {
	Set   bool
	Value []apikeys.Scope
}

func (value *optionalScopes) UnmarshalJSON(data []byte) error {
	value.Set = true
	if string(data) == "null" {
		value.Value = nil
		return nil
	}
	return json.Unmarshal(data, &value.Value)
}

type createAPIKeyRequest struct {
	Name          string         `json:"name"`
	ApplicationID optionalString `json:"applicationId"`
	Scopes        optionalScopes `json:"scopes"`
	ExpiresAt     optionalString `json:"expiresAt"`
}

type toggleAPIKeyRequest struct {
	IsActive *bool `json:"isActive"`
}

type apiKeyToggleData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

func (handler *APIKeyHandler) Create(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[createAPIKeyRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	input, apiErr := createAPIKeyInput(request)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	created, err := handler.operations.Create(c.Request.Context(), principal.UserID, input)
	if err != nil {
		WriteError(c, apiKeyAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, created)
}

func (handler *APIKeyHandler) List(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	applicationID, apiErr := applicationIDFilter(c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	keys, err := handler.operations.List(c.Request.Context(), principal.UserID, applicationID)
	if err != nil {
		WriteError(c, apiKeyAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, keys)
}

func (handler *APIKeyHandler) Delete(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	keyID := c.Param("keyId")
	if !validAPIKeyUUID(keyID) {
		WriteError(c, BadRequest("keyId must be a UUID"))
		return
	}
	if err := handler.operations.Delete(c.Request.Context(), principal.UserID, keyID); err != nil {
		WriteError(c, apiKeyAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, map[string]bool{"success": true})
}

func (handler *APIKeyHandler) Toggle(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	keyID := c.Param("keyId")
	if !validAPIKeyUUID(keyID) {
		WriteError(c, BadRequest("keyId must be a UUID"))
		return
	}
	request, apiErr := DecodeJSON[toggleAPIKeyRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	if request.IsActive == nil {
		WriteError(c, BadRequest("isActive is required"))
		return
	}
	key, err := handler.operations.Toggle(c.Request.Context(), principal.UserID, keyID, *request.IsActive)
	if err != nil {
		WriteError(c, apiKeyAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, apiKeyToggleData{ID: key.ID, Name: key.Name, IsActive: key.IsActive})
}

func createAPIKeyInput(request createAPIKeyRequest) (apikeys.CreateInput, *APIError) {
	input := apikeys.CreateInput{Name: request.Name}
	if request.ApplicationID.Set {
		if request.ApplicationID.Value == nil || !validAPIKeyUUID(*request.ApplicationID.Value) {
			return apikeys.CreateInput{}, BadRequest("applicationId must be a UUID")
		}
		input.ApplicationID = request.ApplicationID.Value
	}
	if request.Scopes.Set {
		if request.Scopes.Value == nil {
			return apikeys.CreateInput{}, BadRequest("scopes must be an array")
		}
		seen := make(map[apikeys.Scope]struct{}, len(request.Scopes.Value))
		for _, scope := range request.Scopes.Value {
			if !apikeys.ValidScope(scope) {
				return apikeys.CreateInput{}, BadRequest("Unsupported API key scope")
			}
			if _, exists := seen[scope]; exists {
				return apikeys.CreateInput{}, BadRequest("API key scopes must be unique")
			}
			seen[scope] = struct{}{}
		}
		input.Scopes = make([]apikeys.Scope, len(request.Scopes.Value))
		copy(input.Scopes, request.Scopes.Value)
	}
	if request.ExpiresAt.Set {
		if request.ExpiresAt.Value == nil {
			return apikeys.CreateInput{}, BadRequest("expiresAt must be an RFC3339 timestamp")
		}
		expiresAt, err := time.Parse(time.RFC3339, *request.ExpiresAt.Value)
		if err != nil {
			return apikeys.CreateInput{}, BadRequest("expiresAt must be an RFC3339 timestamp")
		}
		input.ExpiresAt = &expiresAt
	}
	return input, nil
}

func applicationIDFilter(c *gin.Context) (*string, *APIError) {
	values, exists := c.Request.URL.Query()["applicationId"]
	if !exists {
		return nil, nil
	}
	if len(values) != 1 || !validAPIKeyUUID(values[0]) {
		return nil, BadRequest("applicationId must be a UUID")
	}
	value := values[0]
	return &value, nil
}

func validAPIKeyUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
				return false
			}
		}
	}
	return true
}

func apiKeyAPIError(err error) *APIError {
	var serviceErr *apikeys.ServiceError
	if !errors.As(err, &serviceErr) {
		return Internal(err)
	}
	switch serviceErr.Kind {
	case apikeys.ErrorInvalidInput:
		return BadRequest(serviceErr.Message)
	case apikeys.ErrorForbidden:
		return Forbidden(serviceErr.Message)
	case apikeys.ErrorNotFound:
		return NotFound(serviceErr.Message)
	case apikeys.ErrorInternal:
		return Internal(serviceErr.Cause)
	default:
		return Internal(err)
	}
}
