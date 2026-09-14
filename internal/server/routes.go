package server

import (
	"file-converter/handlers"
	"file-converter/internal/auth"
	"file-converter/middleware"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.Default()
	r.Use(cors.Default())
	return r
}

func RegisterPublicWebRoutes(r *gin.Engine) {
	r.Static("/static", "./static")
	r.Static("/assets/svg", "./static/svg")
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	r.GET("/online-note", func(c *gin.Context) {
		c.HTML(http.StatusOK, "notes.html", nil)
	})
	r.HEAD("/online-note", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/password-x", func(c *gin.Context) {
		c.File("./static/vault.html")
	})
	r.GET("/vault", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/password-x")
	})
}

// RegisterProtectedWebRoutes 注册需要登录才能访问的 HTML 页面；
// 未登录访问会被 PageAuthRequired 重定向到首页登录入口。
func RegisterProtectedWebRoutes(r *gin.Engine, reg *auth.Registry, allowAdmin bool, jwtSecret string) {
	pages := r.Group("/")
	pages.Use(middleware.PageAuthRequired(jwtSecret, reg, allowAdmin))
	pages.GET("/file-convert", func(c *gin.Context) {
		c.HTML(http.StatusOK, "file-convert.html", nil)
	})
	pages.GET("/png-to-pdf", func(c *gin.Context) {
		c.HTML(http.StatusOK, "png2pdf.html", nil)
	})
	pages.GET("/gif-generate", func(c *gin.Context) {
		c.HTML(http.StatusOK, "gif-generator.html", nil)
	})
}

func RegisterAuthRoutes(r *gin.Engine, reg *auth.Registry, authPassword, jwtSecret string) {
	r.POST("/api/login", handlers.LoginHandler(reg, authPassword, jwtSecret))
	r.POST("/api/logout", handlers.LogoutHandler)
}
