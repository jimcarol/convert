package main

import (
	"file-converter/handlers"
	"file-converter/internal/server"
	"file-converter/middleware"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable must be set")
	}

	r := server.NewRouter()
	r.LoadHTMLGlob("templates/*")

	r.GET("/tts", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tts.html", nil)
	})

	api := r.Group("/")
	api.Use(middleware.AuthRequired(jwtSecret))
	api.POST("/tts", handlers.TTSHandler)
	api.GET("/tts/voices", handlers.GetTTSVoices)
	api.GET("/tts/download/:filename", handlers.TTSDownloadHandler)

	_ = os.MkdirAll("./tmp", os.ModePerm)
	go handlers.StartTTSCleaner()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
