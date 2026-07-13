package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (err *APIError) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}

func WriteError(c *gin.Context, apiErr *APIError) {
	if apiErr == nil {
		apiErr = &APIError{
			Status:  http.StatusInternalServerError,
			Code:    "INTERNAL_ERROR",
			Message: "Internal server error",
		}
	}
	status := apiErr.Status
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	code := apiErr.Code
	if code == "" {
		code = errorCodeForStatus(status)
	}
	message := apiErr.Message
	if message == "" {
		message = http.StatusText(status)
	}
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.RequestURI()
	}
	c.AbortWithStatusJSON(status, ErrorEnvelope{
		Success:   false,
		Code:      code,
		Message:   message,
		Data:      nil,
		Timestamp: time.Now().UTC(),
		Path:      path,
	})
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "VALIDATION_ERROR"
	case http.StatusTooManyRequests:
		return "RATE_LIMIT"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	default:
		return "UNKNOWN_ERROR"
	}
}

func BadRequest(message string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: message}
}

func Unauthorized(message string) *APIError {
	return &APIError{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: message}
}

func Forbidden(message string) *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: message}
}

func NotFound(message string) *APIError {
	return &APIError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: message}
}

func Conflict(message string) *APIError {
	return &APIError{Status: http.StatusConflict, Code: "CONFLICT", Message: message}
}

func Internal(cause error) *APIError {
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL_ERROR",
		Message: "Internal server error",
		Cause:   cause,
	}
}
