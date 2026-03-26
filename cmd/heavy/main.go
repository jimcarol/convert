package main

import (
	"file-converter/internal/heavy"
	"file-converter/internal/server"
	"log"
	"os"
)

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable must be set")
	}

	r := server.NewRouter()
	heavy.RegisterRoutes(r, jwtSecret)

	_ = os.MkdirAll("./tmp", os.ModePerm)
	go heavy.AutoCleanTmp()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
