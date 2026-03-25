package handlers

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type loginRequest struct {
	Password string `json:"password"`
}

func LoginHandler(authPassword, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(authPassword)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
		})

		tokenStr, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("token", tokenStr, 7*24*3600, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	}
}

func LogoutHandler(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
