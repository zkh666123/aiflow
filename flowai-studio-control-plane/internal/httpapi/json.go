package httpapi

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
)

func DecodeJSON[T any](c *gin.Context) (T, *APIError) {
	var value T
	if c.Request == nil || c.Request.Body == nil {
		return value, BadRequest("Request body is required")
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		if errors.Is(err, io.EOF) {
			return value, BadRequest("Request body is required")
		}
		return value, BadRequest("Invalid request body")
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return value, BadRequest("Request body must contain one JSON value")
	}
	return value, nil
}
