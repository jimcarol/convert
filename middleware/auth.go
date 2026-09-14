package middleware

import (
	"file-converter/internal/auth"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ContextKeyUsername 是验签后写入 gin.Context 的用户名 key。
const ContextKeyUsername = "username"

func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("token")
		if err != nil || tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if sub, _ := claims["sub"].(string); sub != "" {
				c.Set(ContextKeyUsername, sub)
			}
		}

		c.Next()
	}
}

// Username 返回当前请求的用户名（需先经过 AuthRequired），无则返回 ""。
func Username(c *gin.Context) string {
	if v, ok := c.Get(ContextKeyUsername); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// RequireActiveUser 在 AuthRequired 之后使用：要求 JWT 带 sub 且用户仍在册。
// 撤销邀请码后重启，旧 JWT 立即失效。allowAdmin=false 时（未配置 AUTH_PASSWORD）
// admin 身份同样拒绝。
func RequireActiveUser(reg *auth.Registry, allowAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		sub := Username(c)
		if sub == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "please log in again"})
			c.Abort()
			return
		}
		if sub == auth.AdminUsername && allowAdmin {
			c.Next()
			return
		}
		if !reg.IsActive(sub) || sub == auth.AdminUsername {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "account revoked"})
			c.Abort()
			return
		}
		c.Next()
	}
}
