package main

import (
	"file-converter/internal/server"
	"file-converter/internal/video"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// Standalone AI video rework service (face swap + re-voice).
// Local-only by default: binds 127.0.0.1 unless BIND says otherwise.
func main() {
	tmpDir := os.Getenv("VIDEO_TMP_DIR")
	if tmpDir == "" {
		tmpDir = "./video-tmp"
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		log.Fatal(err)
	}

	r := server.NewRouter()
	r.LoadHTMLFiles("templates/video.html")
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "video.html", nil)
	})

	store := video.NewStore()
	video.RegisterRoutes(r, tmpDir, store)

	go video.StartCleaner(store, tmpDir)

	bind := os.Getenv("BIND")
	if bind == "" {
		bind = "127.0.0.1"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("video service listening on %s:%s (tmp: %s)", bind, port, tmpDir)
	video.CheckDeps()
	if err := r.Run(bind + ":" + port); err != nil {
		log.Fatal(err)
	}
}
