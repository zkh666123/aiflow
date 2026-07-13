package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
)

const principalContextKey = "flowai.auth.principal"

type TokenVerifier interface {
	Verify(string) (auth.Principal, error)
}

func Authentication(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			WriteError(c, Unauthorized("Access token is missing"))
			return
		}

		principal, err := verifier.Verify(parts[1])
		if err != nil {
			WriteError(c, Unauthorized("Invalid or expired token"))
			return
		}
		c.Set(principalContextKey, principal)
		c.Next()
	}
}

func CurrentPrincipal(c *gin.Context) (auth.Principal, bool) {
	value, ok := c.Get(principalContextKey)
	if !ok {
		return auth.Principal{}, false
	}
	principal, ok := value.(auth.Principal)
	return principal, ok
}
