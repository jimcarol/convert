package middleware

import (
	"file-converter/internal/auth"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ContextKeyUsername 是验签后写入 gin.Context 的用户名 key。
const ContextKeyUsername = "username"

// validateToken 校验请求 cookie 中的 JWT;有效则把 sub 写入 context 并返回 true。
func validateToken(c *gin.Context, jwtSecret string) bool {
	tokenStr, err := c.Cookie("token")
	if err != nil || tokenStr == "" {
		return false
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if sub, _ := claims["sub"].(string); sub != "" {
			c.Set(ContextKeyUsername, sub)
		}
	}
	return true
}

func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validateToken(c, jwtSecret) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
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

// activeUser 判定 sub 是否为可用身份：admin 仅在 allowAdmin 时可用，
// 其余用户需在邀请码注册表中仍在册。
func activeUser(reg *auth.Registry, allowAdmin bool, sub string) bool {
	if sub == "" {
		return false
	}
	if sub == auth.AdminUsername {
		return allowAdmin
	}
	return reg.IsActive(sub)
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
		if !activeUser(reg, allowAdmin, sub) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "account revoked"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// PageAuthRequired 保护 HTML 页面：JWT 缺失/无效或账号被撤销时，
// 重定向到首页登录入口，而不是返回 JSON 401。
func PageAuthRequired(jwtSecret string, reg *auth.Registry, allowAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validateToken(c, jwtSecret) || !activeUser(reg, allowAdmin, Username(c)) {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminRequired 仅允许 admin（AUTH_PASSWORD 通道）访问，其余登录用户 403。
// 必须在 AuthRequired 之后使用。
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if Username(c) != auth.AdminUsername {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// PageAdminRequired 页面版 admin 限定：未登录/已撤销 302 回首页（同 PageAuthRequired），
// 已登录但非 admin 返回 403 提示。
func PageAdminRequired(jwtSecret string, reg *auth.Registry, allowAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validateToken(c, jwtSecret) || !activeUser(reg, allowAdmin, Username(c)) {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		if Username(c) != auth.AdminUsername {
			c.Data(http.StatusForbidden, "text/html; charset=utf-8",
				[]byte(`<!DOCTYPE html><html lang="zh"><head><meta charset="UTF-8"><title>403</title></head><body style="font-family:sans-serif;text-align:center;padding-top:4rem;"><h2>403 · 仅限管理员使用</h2><p><a href="/">返回首页</a></p></body></html>`))
			c.Abort()
			return
		}
		c.Next()
	}
}
