package server

import (
	"file-converter/handlers"
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
	r.GET("/file-convert", func(c *gin.Context) {
		c.HTML(http.StatusOK, "file-convert.html", nil)
	})
	r.GET("/png-to-pdf", func(c *gin.Context) {
		c.HTML(http.StatusOK, "png2pdf.html", nil)
	})
	r.GET("/gif-generate", func(c *gin.Context) {
		c.HTML(http.StatusOK, "gif-generator.html", nil)
	})
	r.GET("/online-note", func(c *gin.Context) {
		c.HTML(http.StatusOK, "notes.html", nil)
	})
	r.GET("/password-x", func(c *gin.Context) {
		c.File("./static/vault.html")
	})
	r.GET("/vault", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/password-x")
	})
}

func RegisterAuthRoutes(r *gin.Engine, authPassword, jwtSecret string) {
	r.POST("/api/login", handlers.LoginHandler(authPassword, jwtSecret))
	r.POST("/api/logout", handlers.LogoutHandler)
}
