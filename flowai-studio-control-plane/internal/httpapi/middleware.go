package httpapi

import "github.com/gin-gonic/gin"

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() == nil {
				return
			}
			if !c.Writer.Written() {
				writeInternalError(c)
			}
			c.Abort()
		}()
		c.Next()
	}
}
