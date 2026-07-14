package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/shares"
)

type ShareOperations interface {
	Generate(context.Context, string, string) (shares.Share, error)
	Get(context.Context, string, string) (shares.Share, error)
	Update(context.Context, string, string, shares.UpdateInput) (shares.Share, error)
	Revoke(context.Context, string, string) error
	Embed(context.Context, string, string) (shares.EmbedCode, error)
	GetPublic(context.Context, string) (shares.PublicApplication, error)
}

type ShareHandler struct {
	operations ShareOperations
}

func NewShareHandler(operations ShareOperations) *ShareHandler {
	return &ShareHandler{operations: operations}
}

func RegisterShareRoutes(router *gin.Engine, handler *ShareHandler, verifier TokenVerifier) {
	apps := router.Group("/api/apps")
	apps.Use(Authentication(verifier))
	apps.POST("/:id/share", handler.Generate)
	apps.GET("/:id/share", handler.Get)
	apps.PATCH("/:id/share", handler.Update)
	apps.DELETE("/:id/share", handler.Revoke)
	apps.GET("/:id/embed", handler.Embed)

	router.GET("/api/share/:shareLink", handler.GetPublic)
}

type updateShareRequest struct {
	IsPublic    *bool               `json:"isPublic"`
	EmbedConfig *shares.EmbedConfig `json:"embedConfig"`
}

func (handler *ShareHandler) Generate(c *gin.Context) {
	principal, applicationID, ok := sharePrincipalAndApplication(c)
	if !ok {
		return
	}
	share, err := handler.operations.Generate(c.Request.Context(), principal.UserID, applicationID)
	if err != nil {
		WriteError(c, shareAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, share)
}

func (handler *ShareHandler) Get(c *gin.Context) {
	principal, applicationID, ok := sharePrincipalAndApplication(c)
	if !ok {
		return
	}
	share, err := handler.operations.Get(c.Request.Context(), principal.UserID, applicationID)
	if err != nil {
		WriteError(c, shareAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, share)
}

func (handler *ShareHandler) Update(c *gin.Context) {
	principal, applicationID, ok := sharePrincipalAndApplication(c)
	if !ok {
		return
	}
	request, apiErr := DecodeJSON[updateShareRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	if request.IsPublic == nil && request.EmbedConfig == nil {
		WriteError(c, BadRequest("At least one share setting is required"))
		return
	}
	input := shares.UpdateInput{}
	if request.IsPublic != nil {
		input.SetIsPublic = true
		input.IsPublic = *request.IsPublic
	}
	if request.EmbedConfig != nil {
		input.SetEmbedConfig = true
		input.EmbedConfig = request.EmbedConfig
	}
	share, err := handler.operations.Update(c.Request.Context(), principal.UserID, applicationID, input)
	if err != nil {
		WriteError(c, shareAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, share)
}

func (handler *ShareHandler) Revoke(c *gin.Context) {
	principal, applicationID, ok := sharePrincipalAndApplication(c)
	if !ok {
		return
	}
	if err := handler.operations.Revoke(c.Request.Context(), principal.UserID, applicationID); err != nil {
		WriteError(c, shareAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, map[string]bool{"success": true})
}

func (handler *ShareHandler) Embed(c *gin.Context) {
	principal, applicationID, ok := sharePrincipalAndApplication(c)
	if !ok {
		return
	}
	embed, err := handler.operations.Embed(c.Request.Context(), principal.UserID, applicationID)
	if err != nil {
		WriteError(c, shareAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, embed)
}

func (handler *ShareHandler) GetPublic(c *gin.Context) {
	application, err := handler.operations.GetPublic(c.Request.Context(), c.Param("shareLink"))
	if err != nil {
		WriteError(c, shareAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, application)
}

func sharePrincipalAndApplication(c *gin.Context) (principal auth.Principal, applicationID string, ok bool) {
	current, exists := CurrentPrincipal(c)
	if !exists {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return principal, "", false
	}
	applicationID = c.Param("id")
	if !validAPIKeyUUID(applicationID) {
		WriteError(c, BadRequest("appId must be a UUID"))
		return principal, "", false
	}
	return current, applicationID, true
}

func shareAPIError(err error) *APIError {
	var serviceErr *shares.ServiceError
	if !errors.As(err, &serviceErr) {
		return Internal(err)
	}
	switch serviceErr.Kind {
	case shares.ErrorInvalidInput:
		return BadRequest(serviceErr.Message)
	case shares.ErrorForbidden:
		return Forbidden(serviceErr.Message)
	case shares.ErrorNotFound:
		return NotFound(serviceErr.Message)
	case shares.ErrorInternal:
		return Internal(serviceErr.Cause)
	default:
		return Internal(err)
	}
}
