package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

func AuthRequired(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := getBearerToken(c)
		if token == "" {
			model.FailStatus(c, http.StatusUnauthorized, 401, "缺少认证令牌")
			c.Abort()
			return
		}
		claims, err := authService.ParseToken(token)
		if err != nil {
			model.FailStatus(c, http.StatusUnauthorized, 401, "invalid token")
			c.Abort()
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := c.MustGet("claims").(*service.Claims)
		if claims.Role != model.RoleSuperAdmin && claims.Role != model.RoleTenantAdmin {
			model.FailStatus(c, http.StatusForbidden, 403, "admin role required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := c.MustGet("claims").(*service.Claims)
		if claims.Role != model.RoleSuperAdmin {
			model.FailStatus(c, http.StatusForbidden, 403, "super admin role required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func getBearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if len(header) < 8 || header[:7] != "Bearer " {
		return ""
	}
	return header[7:]
}
