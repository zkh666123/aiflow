package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/teams"
)

type TeamOperations interface {
	Create(context.Context, string, teams.CreateInput) (teams.Team, error)
	List(context.Context, string) ([]teams.Team, error)
	Get(context.Context, string, string) (teams.Team, error)
	Update(context.Context, string, string, teams.UpdateInput) (teams.Team, error)
	Delete(context.Context, string, string) error
	AddMember(context.Context, string, string, teams.AddMemberInput) (teams.Member, error)
	UpdateMemberRole(context.Context, string, string, string, rbac.TeamRole) (teams.Member, error)
	RemoveMember(context.Context, string, string, string) error
	Leave(context.Context, string, string) error
	AddApplication(context.Context, string, string, teams.AddApplicationInput) (teams.TeamApplication, error)
	UpdateApplicationPermission(context.Context, string, string, string, rbac.TeamAppPermission) (teams.TeamApplication, error)
	RemoveApplication(context.Context, string, string, string) error
}

type TeamHandler struct {
	operations TeamOperations
}

func NewTeamHandler(operations TeamOperations) *TeamHandler {
	return &TeamHandler{operations: operations}
}

func RegisterTeamRoutes(router *gin.Engine, handler *TeamHandler, verifier TokenVerifier) {
	group := router.Group("/api/teams")
	group.Use(Authentication(verifier))
	group.POST("", handler.Create)
	group.GET("", handler.List)
	group.GET("/:teamId", handler.Get)
	group.PATCH("/:teamId", handler.Update)
	group.DELETE("/:teamId", handler.Delete)
	group.POST("/:teamId/members", handler.AddMember)
	group.PATCH("/:teamId/members/:memberId", handler.UpdateMemberRole)
	group.DELETE("/:teamId/members/:memberId", handler.RemoveMember)
	group.POST("/:teamId/leave", handler.Leave)
	group.POST("/:teamId/apps", handler.AddApplication)
	group.PATCH("/:teamId/apps/:teamAppId", handler.UpdateApplicationPermission)
	group.DELETE("/:teamId/apps/:teamAppId", handler.RemoveApplication)
}

type createTeamRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Avatar      *string `json:"avatar"`
}

type updateTeamRequest struct {
	Name        optionalString `json:"name"`
	Description optionalString `json:"description"`
	Avatar      optionalString `json:"avatar"`
}

type addMemberRequest struct {
	UserID string        `json:"userId"`
	Role   rbac.TeamRole `json:"role"`
}

type updateMemberRoleRequest struct {
	Role rbac.TeamRole `json:"role"`
}

type addTeamApplicationRequest struct {
	ApplicationID string                 `json:"applicationId"`
	Permission    rbac.TeamAppPermission `json:"permission"`
}

type updateTeamApplicationRequest struct {
	Permission rbac.TeamAppPermission `json:"permission"`
}

func (handler *TeamHandler) Create(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[createTeamRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	team, err := handler.operations.Create(c.Request.Context(), principal.UserID, teams.CreateInput{
		Name: request.Name, Description: request.Description, Avatar: request.Avatar,
	})
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, team)
}

func (handler *TeamHandler) List(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	result, err := handler.operations.List(c.Request.Context(), principal.UserID)
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, result)
}

func (handler *TeamHandler) Get(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	team, err := handler.operations.Get(c.Request.Context(), principal.UserID, c.Param("teamId"))
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, team)
}

func (handler *TeamHandler) Update(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[updateTeamRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	if request.Name.Set && request.Name.Value == nil {
		WriteError(c, BadRequest("Name must be a string"))
		return
	}
	team, err := handler.operations.Update(c.Request.Context(), principal.UserID, c.Param("teamId"), teams.UpdateInput{
		Name: request.Name.Value, DescriptionSet: request.Description.Set, Description: request.Description.Value,
		AvatarSet: request.Avatar.Set, Avatar: request.Avatar.Value,
	})
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, team)
}

func (handler *TeamHandler) Delete(c *gin.Context) {
	handler.writeCommand(c, http.StatusOK, func(ctx context.Context, userID string) error {
		return handler.operations.Delete(ctx, userID, c.Param("teamId"))
	})
}

func (handler *TeamHandler) AddMember(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[addMemberRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	member, err := handler.operations.AddMember(c.Request.Context(), principal.UserID, c.Param("teamId"), teams.AddMemberInput{
		UserID: request.UserID, Role: request.Role,
	})
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, member)
}

func (handler *TeamHandler) UpdateMemberRole(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[updateMemberRoleRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	member, err := handler.operations.UpdateMemberRole(
		c.Request.Context(), principal.UserID, c.Param("teamId"), c.Param("memberId"), request.Role,
	)
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, member)
}

func (handler *TeamHandler) RemoveMember(c *gin.Context) {
	handler.writeCommand(c, http.StatusOK, func(ctx context.Context, userID string) error {
		return handler.operations.RemoveMember(ctx, userID, c.Param("teamId"), c.Param("memberId"))
	})
}

func (handler *TeamHandler) Leave(c *gin.Context) {
	handler.writeCommand(c, http.StatusCreated, func(ctx context.Context, userID string) error {
		return handler.operations.Leave(ctx, userID, c.Param("teamId"))
	})
}

func (handler *TeamHandler) AddApplication(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[addTeamApplicationRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	application, err := handler.operations.AddApplication(c.Request.Context(), principal.UserID, c.Param("teamId"), teams.AddApplicationInput{
		ApplicationID: request.ApplicationID, Permission: request.Permission,
	})
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, application)
}

func (handler *TeamHandler) UpdateApplicationPermission(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[updateTeamApplicationRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	application, err := handler.operations.UpdateApplicationPermission(
		c.Request.Context(), principal.UserID, c.Param("teamId"), c.Param("teamAppId"), request.Permission,
	)
	if err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, application)
}

func (handler *TeamHandler) RemoveApplication(c *gin.Context) {
	handler.writeCommand(c, http.StatusOK, func(ctx context.Context, userID string) error {
		return handler.operations.RemoveApplication(ctx, userID, c.Param("teamId"), c.Param("teamAppId"))
	})
}

func (handler *TeamHandler) writeCommand(
	c *gin.Context,
	status int,
	operation func(context.Context, string) error,
) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	if err := operation(c.Request.Context(), principal.UserID); err != nil {
		WriteError(c, teamAPIError(err))
		return
	}
	WriteSuccess(c, status, map[string]bool{"success": true})
}

func teamAPIError(err error) *APIError {
	var serviceErr *teams.ServiceError
	if !errors.As(err, &serviceErr) {
		return Internal(err)
	}
	switch serviceErr.Kind {
	case teams.ErrorInvalidInput:
		return BadRequest(serviceErr.Message)
	case teams.ErrorForbidden:
		return Forbidden(serviceErr.Message)
	case teams.ErrorNotFound:
		return NotFound(serviceErr.Message)
	case teams.ErrorConflict:
		return Conflict(serviceErr.Message)
	case teams.ErrorInternal:
		return Internal(serviceErr.Cause)
	default:
		return Internal(err)
	}
}
