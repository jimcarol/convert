package video

import (
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxVideoSize = 500 << 20 // 500MB
	maxFaceSize  = 10 << 20  // 10MB
	jobTTL       = time.Hour
)

var validJobID = regexp.MustCompile(`^[0-9]+$`)

var videoExts = map[string]bool{".mp4": true, ".mov": true, ".webm": true, ".mkv": true}
var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}

// RegisterRoutes wires the video service endpoints onto r. tmpDir is the
// working directory for job artifacts (one subdirectory per job); the
// store is shared with StartCleaner.
func RegisterRoutes(r *gin.Engine, tmpDir string, store *Store) {

	r.GET("/voices", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"voices": voices})
	})

	r.POST("/jobs", func(c *gin.Context) {
		createJob(c, store, tmpDir)
	})

	r.GET("/jobs/:id", func(c *gin.Context) {
		job, ok := lookupJob(c, store)
		if !ok {
			return
		}
		resp := gin.H{
			"id":     job.ID,
			"stage":  job.Stage,
			"stages": []string{StageExtracting, StageTranscribing, StageRewriting, StageVoicing, StageSwapping, StageMuxing, StageDone},
		}
		if job.Err != "" {
			resp["error"] = job.Err
		}
		if job.Stage == StageDone {
			resp["download_url"] = "/jobs/" + job.ID + "/download"
		}
		c.JSON(http.StatusOK, resp)
	})

	r.GET("/jobs/:id/download", func(c *gin.Context) {
		job, ok := lookupJob(c, store)
		if !ok {
			return
		}
		if job.Stage != StageDone {
			c.JSON(http.StatusConflict, gin.H{"error": "job not finished", "stage": job.Stage})
			return
		}
		c.FileAttachment(job.FinalFile, "final.mp4")
	})
}

func createJob(c *gin.Context, store *Store, tmpDir string) {
	opts := JobOptions{
		Faceswap:      c.PostForm("faceswap") == "1" || c.PostForm("faceswap") == "true",
		Revoice:       c.PostForm("revoice") == "1" || c.PostForm("revoice") == "true",
		Voice:         c.PostForm("voice"),
		Language:      c.PostForm("language"),
		Instruction:   c.PostForm("instruction"),
		BurnSubtitles: c.PostForm("burn_subtitles") == "1" || c.PostForm("burn_subtitles") == "true",
	}

	if !opts.Faceswap && !opts.Revoice {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of faceswap/revoice must be enabled"})
		return
	}
	if opts.Voice != "" && !voiceSet[opts.Voice] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported voice"})
		return
	}
	if opts.Language != "" && !languageSet[opts.Language] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported language"})
		return
	}

	videoHeader, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video file is required"})
		return
	}
	if videoHeader.Size > maxVideoSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video exceeds 500MB"})
		return
	}
	if !videoExts[strings.ToLower(filepath.Ext(videoHeader.Filename))] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video must be mp4/mov/webm/mkv"})
		return
	}

	var face *multipart.FileHeader
	if opts.Faceswap {
		var faceErr error
		face, faceErr = c.FormFile("face")
		if faceErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "face image is required when faceswap=1"})
			return
		}
		if face.Size > maxFaceSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "face image exceeds 10MB"})
			return
		}
		if !imageExts[strings.ToLower(filepath.Ext(face.Filename))] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "face must be png/jpg/jpeg/webp"})
			return
		}
	}

	// build the job first so its ID doubles as the working dir name
	// (the cleaner relies on this 1:1 mapping); register it only after
	// all fields are populated
	job := store.New(opts)
	jobDir := filepath.Join(tmpDir, job.ID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job dir"})
		return
	}
	job.Dir = jobDir

	videoPath := filepath.Join(jobDir, "input"+strings.ToLower(filepath.Ext(videoHeader.Filename)))
	if err := c.SaveUploadedFile(videoHeader, videoPath); err != nil {
		os.RemoveAll(jobDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save video"})
		return
	}
	job.InputVideo = videoPath

	if opts.Faceswap {
		facePath := filepath.Join(jobDir, "face"+strings.ToLower(filepath.Ext(face.Filename)))
		if err := c.SaveUploadedFile(face, facePath); err != nil {
			os.RemoveAll(jobDir)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save face image"})
			return
		}
		job.InputFace = facePath
	}

	store.register(job)
	go store.run(job)

	c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "status_url": "/jobs/" + job.ID})
}

func lookupJob(c *gin.Context, store *Store) (*Job, bool) {
	id := c.Param("id")
	if !validJobID.MatchString(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return nil, false
	}
	job, ok := store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return nil, false
	}
	return job, true
}

// StartCleaner periodically removes job directories (and their registry
// entries) older than jobTTL. Video intermediates are far larger than the
// mp3s the TTS cleaner handles, hence the 1h TTL.
func StartCleaner(store *Store, tmpDir string) {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			path := filepath.Join(tmpDir, e.Name())
			info, err := os.Stat(path)
			if err != nil || time.Since(info.ModTime()) <= jobTTL {
				continue
			}
			if err := os.RemoveAll(path); err != nil {
				log.Printf("video cleaner: remove %s: %v", path, err)
				continue
			}
			store.remove(e.Name())
		}
	}
}
