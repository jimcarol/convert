package main

import (
	"file-converter/handlers"
	"file-converter/internal/server"
	"file-converter/middleware"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	authPassword := os.Getenv("AUTH_PASSWORD")
	jwtSecret := os.Getenv("JWT_SECRET")
	if authPassword == "" || jwtSecret == "" {
		log.Fatal("AUTH_PASSWORD and JWT_SECRET environment variables must be set")
	}

	r := server.NewRouter()
	server.RegisterPublicWebRoutes(r)
	server.RegisterAuthRoutes(r, authPassword, jwtSecret)
	registerProtectedRoutes(r, jwtSecret)

	handlers.LoadNotes()
	handlers.LoadPasswords()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func registerProtectedRoutes(r *gin.Engine, jwtSecret string) {
	api := r.Group("/")
	api.Use(middleware.AuthRequired(jwtSecret))
	{
		api.GET("/notes", handlers.GetNotes)
		api.POST("/notes", handlers.CreateNote)
		api.PUT("/notes/:id", handlers.UpdateNote)
		api.DELETE("/notes/:id", handlers.DeleteNote)

		api.GET("/passwords", handlers.GetPasswords)
		api.POST("/passwords", handlers.CreatePassword)
		api.PUT("/passwords/:id", handlers.UpdatePassword)
		api.DELETE("/passwords/:id", handlers.DeletePassword)
	}
}
