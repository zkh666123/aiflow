package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Envelope[T any] struct {
	Success   bool      `json:"success"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Data      T         `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func SuccessEnvelope[T any](data T, timestamp time.Time) Envelope[T] {
	return Envelope[T]{
		Success:   true,
		Code:      "SUCCESS",
		Message:   "Success",
		Data:      data,
		Timestamp: timestamp,
	}
}

func WriteSuccess(c *gin.Context, status int, data any) {
	c.JSON(status, SuccessEnvelope(data, time.Now().UTC()))
}

type ErrorEnvelope struct {
	Success   bool      `json:"success"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
}

func writeInternalError(c *gin.Context) {
	WriteError(c, &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL_ERROR",
		Message: "Internal server error",
	})
}
