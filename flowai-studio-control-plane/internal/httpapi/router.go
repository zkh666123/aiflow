package httpapi

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

func NewRouter(health gin.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Use(RecoveryMiddleware(), requestIDMiddleware())
	router.GET("/api/health", health)
	return router
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			value := make([]byte, 16)
			if _, err := rand.Read(value); err == nil {
				requestID = hex.EncodeToString(value)
			}
		}
		if requestID != "" {
			c.Header(requestIDHeader, requestID)
		}
		c.Next()
	}
}
