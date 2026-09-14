package handlers

import (
	"crypto/subtle"
	"file-converter/internal/auth"
	"file-converter/middleware"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type loginRequest struct {
	// 字段名保持 password 不变：password-x 前端是压缩 bundle 改不动，
	// 它固定 POST {"password": "..."}。服务端把值先当邀请码查表，
	// 未命中再兜底比对 AUTH_PASSWORD（admin 通道）。
	Password string `json:"password"`
}

func LoginHandler(reg *auth.Registry, authPassword, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		username, ok := reg.Lookup(req.Password)
		if !ok {
			if authPassword != "" && subtle.ConstantTimeCompare([]byte(req.Password), []byte(authPassword)) == 1 {
				username, ok = auth.AdminUsername, true
			}
		}
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid invite code"})
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": username,
			"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
		})

		tokenStr, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("token", tokenStr, 7*24*3600, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"message": "ok", "username": username})
	}
}

func LogoutHandler(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// MeHandler 返回当前登录用户，供前端显示身份。
func MeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"username": middleware.Username(c)})
}
