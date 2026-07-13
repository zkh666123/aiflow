package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/applications"
)

type ApplicationOperations interface {
	Create(context.Context, string, applications.CreateInput) (applications.Application, error)
	List(context.Context, string) ([]applications.Application, error)
	Get(context.Context, string, string) (applications.Application, error)
	Update(context.Context, string, string, applications.UpdateInput) (applications.Application, error)
	Delete(context.Context, string, string) error
	Publish(context.Context, string, string) (applications.Application, error)
	Unpublish(context.Context, string, string) (applications.Application, error)
	Archive(context.Context, string, string) (applications.Application, error)
	Unarchive(context.Context, string, string) (applications.Application, error)
}

type ApplicationHandler struct {
	operations ApplicationOperations
}

func NewApplicationHandler(operations ApplicationOperations) *ApplicationHandler {
	return &ApplicationHandler{operations: operations}
}

func RegisterApplicationRoutes(router *gin.Engine, handler *ApplicationHandler, verifier TokenVerifier) {
	apps := router.Group("/api/apps")
	apps.Use(Authentication(verifier))
	apps.POST("", handler.Create)
	apps.GET("", handler.List)
	apps.GET("/:id", handler.Get)
	apps.PATCH("/:id", handler.Update)
	apps.DELETE("/:id", handler.Delete)
	apps.PATCH("/:id/publish", handler.Publish)
	apps.PATCH("/:id/unpublish", handler.Unpublish)
	apps.PATCH("/:id/archive", handler.Archive)
	apps.PATCH("/:id/unarchive", handler.Unarchive)
}

type createApplicationRequest struct {
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	Icon        *string             `json:"icon"`
	Status      applications.Status `json:"status"`
}

type updateApplicationRequest struct {
	Name        optionalString `json:"name"`
	Description optionalString `json:"description"`
	Icon        optionalString `json:"icon"`
	Status      optionalString `json:"status"`
}

type applicationData struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	Icon        *string             `json:"icon"`
	Status      applications.Status `json:"status"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

type applicationListData struct {
	applicationData
	AccessType string `json:"accessType"`
}

type applicationDetailData struct {
	applicationData
	ShareLink *string `json:"shareLink"`
	UserID    string  `json:"userId"`
	Workflows []any   `json:"workflows"`
}

type applicationStatusData struct {
	ID     string              `json:"id"`
	Name   string              `json:"name"`
	Status applications.Status `json:"status"`
}

func (handler *ApplicationHandler) Create(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[createApplicationRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	application, err := handler.operations.Create(c.Request.Context(), principal.UserID, applications.CreateInput{
		Name:        request.Name,
		Description: request.Description,
		Icon:        request.Icon,
		Status:      request.Status,
	})
	if err != nil {
		WriteError(c, applicationAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, publicApplication(application))
}

func (handler *ApplicationHandler) List(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	items, err := handler.operations.List(c.Request.Context(), principal.UserID)
	if err != nil {
		WriteError(c, applicationAPIError(err))
		return
	}
	data := make([]applicationListData, 0, len(items))
	for _, item := range items {
		data = append(data, applicationListData{applicationData: publicApplication(item), AccessType: item.AccessType})
	}
	WriteSuccess(c, http.StatusOK, data)
}

func (handler *ApplicationHandler) Get(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	application, err := handler.operations.Get(c.Request.Context(), principal.UserID, c.Param("id"))
	if err != nil {
		WriteError(c, applicationAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, applicationDetailData{
		applicationData: publicApplication(application),
		ShareLink:       application.ShareLink,
		UserID:          application.OwnerID,
		Workflows:       []any{},
	})
}

func (handler *ApplicationHandler) Update(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[updateApplicationRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	if request.Name.Set && request.Name.Value == nil {
		WriteError(c, BadRequest("Name must be a string"))
		return
	}
	var status *applications.Status
	if request.Status.Set {
		if request.Status.Value == nil {
			WriteError(c, BadRequest("Status must be a string"))
			return
		}
		value := applications.Status(*request.Status.Value)
		status = &value
	}
	application, err := handler.operations.Update(c.Request.Context(), principal.UserID, c.Param("id"), applications.UpdateInput{
		Name:           request.Name.Value,
		DescriptionSet: request.Description.Set,
		Description:    request.Description.Value,
		IconSet:        request.Icon.Set,
		Icon:           request.Icon.Value,
		Status:         status,
	})
	if err != nil {
		WriteError(c, applicationAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, publicApplication(application))
}

func (handler *ApplicationHandler) Delete(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	if err := handler.operations.Delete(c.Request.Context(), principal.UserID, c.Param("id")); err != nil {
		WriteError(c, applicationAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, map[string]bool{"success": true})
}

func (handler *ApplicationHandler) Publish(c *gin.Context) {
	handler.writeStatus(c, handler.operations.Publish)
}

func (handler *ApplicationHandler) Unpublish(c *gin.Context) {
	handler.writeStatus(c, handler.operations.Unpublish)
}

func (handler *ApplicationHandler) Archive(c *gin.Context) {
	handler.writeStatus(c, handler.operations.Archive)
}

func (handler *ApplicationHandler) Unarchive(c *gin.Context) {
	handler.writeStatus(c, handler.operations.Unarchive)
}

func (handler *ApplicationHandler) writeStatus(
	c *gin.Context,
	operation func(context.Context, string, string) (applications.Application, error),
) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	application, err := operation(c.Request.Context(), principal.UserID, c.Param("id"))
	if err != nil {
		WriteError(c, applicationAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, applicationStatusData{
		ID:     application.ID,
		Name:   application.Name,
		Status: application.Status,
	})
}

func publicApplication(application applications.Application) applicationData {
	return applicationData{
		ID:          application.ID,
		Name:        application.Name,
		Description: application.Description,
		Icon:        application.Icon,
		Status:      application.Status,
		CreatedAt:   application.CreatedAt,
		UpdatedAt:   application.UpdatedAt,
	}
}

func applicationAPIError(err error) *APIError {
	var serviceErr *applications.ServiceError
	if !errors.As(err, &serviceErr) {
		return Internal(err)
	}
	switch serviceErr.Kind {
	case applications.ErrorInvalidInput:
		return BadRequest(serviceErr.Message)
	case applications.ErrorForbidden:
		return Forbidden(serviceErr.Message)
	case applications.ErrorNotFound:
		return NotFound(serviceErr.Message)
	case applications.ErrorInternal:
		return Internal(serviceErr.Cause)
	default:
		return Internal(err)
	}
}
