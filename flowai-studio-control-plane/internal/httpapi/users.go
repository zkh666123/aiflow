package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

type UserOperations interface {
	Register(context.Context, auth.RegisterInput) (auth.User, error)
	Login(context.Context, auth.LoginInput) (auth.LoginResult, error)
	Profile(context.Context, string) (auth.User, error)
	UpdateProfile(context.Context, string, auth.UpdateProfileInput) (auth.User, error)
}

type UserHandler struct {
	operations UserOperations
}

func NewUserHandler(operations UserOperations) *UserHandler {
	return &UserHandler{operations: operations}
}

func RegisterUserRoutes(router *gin.Engine, handler *UserHandler, verifier TokenVerifier) {
	users := router.Group("/api/users")
	users.POST("/register", handler.Register)
	users.POST("/login", handler.Login)

	protected := users.Group("")
	protected.Use(Authentication(verifier))
	protected.GET("/profile", handler.Profile)
	protected.PATCH("/profile", handler.UpdateProfile)
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type optionalString struct {
	Set   bool
	Value *string
}

func (value *optionalString) UnmarshalJSON(data []byte) error {
	value.Set = true
	if string(data) == "null" {
		value.Value = nil
		return nil
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}

type updateProfileRequest struct {
	Username optionalString `json:"username"`
	Avatar   optionalString `json:"avatar"`
}

type registerUserData struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
}

type loginUserData struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type loginData struct {
	User  loginUserData `json:"user"`
	Token string        `json:"token"`
}

type profileData struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Avatar    *string   `json:"avatar"`
	CreatedAt time.Time `json:"createdAt"`
}

type updatedProfileData struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Avatar   *string `json:"avatar"`
}

func (handler *UserHandler) Register(c *gin.Context) {
	request, apiErr := DecodeJSON[registerRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	user, err := handler.operations.Register(c.Request.Context(), auth.RegisterInput{
		Username: request.Username,
		Password: request.Password,
	})
	if err != nil {
		WriteError(c, userAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, registerUserData{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	})
}

func (handler *UserHandler) Login(c *gin.Context) {
	request, apiErr := DecodeJSON[loginRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	result, err := handler.operations.Login(c.Request.Context(), auth.LoginInput{
		Username: request.Username,
		Password: request.Password,
	})
	if err != nil {
		WriteError(c, userAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusCreated, loginData{
		User:  loginUserData{ID: result.User.ID, Username: result.User.Username},
		Token: result.Token,
	})
}

func (handler *UserHandler) Profile(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	user, err := handler.operations.Profile(c.Request.Context(), principal.UserID)
	if err != nil {
		WriteError(c, userAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, profileData{
		ID:        user.ID,
		Username:  user.Username,
		Avatar:    user.Avatar,
		CreatedAt: user.CreatedAt,
	})
}

func (handler *UserHandler) UpdateProfile(c *gin.Context) {
	principal, ok := CurrentPrincipal(c)
	if !ok {
		WriteError(c, Unauthorized("Invalid or expired token"))
		return
	}
	request, apiErr := DecodeJSON[updateProfileRequest](c)
	if apiErr != nil {
		WriteError(c, apiErr)
		return
	}
	if request.Username.Set && request.Username.Value == nil {
		WriteError(c, BadRequest("Username must be a string"))
		return
	}
	user, err := handler.operations.UpdateProfile(c.Request.Context(), principal.UserID, auth.UpdateProfileInput{
		Username:  request.Username.Value,
		AvatarSet: request.Avatar.Set,
		Avatar:    request.Avatar.Value,
	})
	if err != nil {
		WriteError(c, userAPIError(err))
		return
	}
	WriteSuccess(c, http.StatusOK, updatedProfileData{
		ID:       user.ID,
		Username: user.Username,
		Avatar:   user.Avatar,
	})
}

func userAPIError(err error) *APIError {
	var serviceErr *auth.ServiceError
	if !errors.As(err, &serviceErr) {
		return Internal(err)
	}
	switch serviceErr.Kind {
	case auth.ErrorInvalidInput:
		return BadRequest(serviceErr.Message)
	case auth.ErrorUnauthorized:
		return Unauthorized(serviceErr.Message)
	case auth.ErrorConflict:
		return Conflict(serviceErr.Message)
	case auth.ErrorNotFound:
		return NotFound(serviceErr.Message)
	case auth.ErrorInternal:
		return Internal(serviceErr.Cause)
	default:
		return Internal(err)
	}
}
