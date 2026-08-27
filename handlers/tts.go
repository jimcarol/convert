package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	ttsMaxRunes = 3000
	ttsTimeout  = 120 * time.Second
)

type VoiceOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var ttsVoices = []VoiceOption{
	{"zh-CN-XiaoxiaoNeural", "大陆 · 晓晓（女声）"},
	{"zh-CN-YunxiNeural", "大陆 · 云希（男声）"},
	{"zh-CN-YunjianNeural", "大陆 · 云健（男声）"},
	{"zh-CN-XiaoyiNeural", "大陆 · 晓伊（女声）"},
	{"zh-TW-HsiaoChenNeural", "台湾 · 晓晨（女声）"},
	{"zh-TW-HsiaoYuNeural", "台湾 · 晓雨（女声）"},
	{"zh-TW-YunJheNeural", "台湾 · 云哲（男声）"},
	{"zh-HK-HiuMaanNeural", "香港 · 晓曼（女声）"},
	{"zh-HK-WanLungNeural", "香港 · 云龙（男声）"},
}

var ttsVoiceSet = func() map[string]bool {
	m := make(map[string]bool, len(ttsVoices))
	for _, v := range ttsVoices {
		m[v.ID] = true
	}
	return m
}()

type TTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
	Rate  int    `json:"rate"`  // -50..100, percent; 0 means edge-tts default
	Pitch int    `json:"pitch"` // -50..50, Hz; 0 means edge-tts default
}

// GetTTSVoices returns the selectable voice list for the TTS page.
func GetTTSVoices(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"voices": ttsVoices})
}

// TTSHandler converts text to speech via the edge-tts CLI and returns a download URL.
func TTSHandler(c *gin.Context) {
	var req TTSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}
	if utf8.RuneCountInString(text) > ttsMaxRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("text too long (max %d chars)", ttsMaxRunes)})
		return
	}
	if !ttsVoiceSet[req.Voice] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported voice"})
		return
	}
	if req.Rate < -50 || req.Rate > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rate must be between -50 and 100"})
		return
	}
	if req.Pitch < -50 || req.Pitch > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pitch must be between -50 and 50"})
		return
	}

	base := fmt.Sprintf("%d", time.Now().UnixNano())
	txtPath := filepath.Join("./tmp", base+".txt")
	mp3Name := base + ".mp3"
	mp3Path := filepath.Join("./tmp", mp3Name)

	if err := os.WriteFile(txtPath, []byte(text), 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write temp file"})
		return
	}
	defer os.Remove(txtPath)

	ctx, cancel := context.WithTimeout(c.Request.Context(), ttsTimeout)
	defer cancel()

	out, err := runEdgeTTS(ctx, req.Voice, txtPath, mp3Path, req.Rate, req.Pitch)
	if ctx.Err() == context.DeadlineExceeded {
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "TTS timeout"})
		return
	}
	if err != nil {
		errOut := string(out)
		if len(errOut) > 500 {
			errOut = errOut[:500]
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS failed: " + errOut})
		return
	}

	if _, err := os.Stat(mp3Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS failed: no output file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"download_url": "/tts/download/" + mp3Name})
}

// TTSDownloadHandler serves generated mp3 files.
func TTSDownloadHandler(c *gin.Context) {
	filename := filepath.Base(c.Param("filename"))
	c.FileAttachment(filepath.Join("./tmp", filename), filename)
}

// runEdgeTTS invokes edge-tts; falls back to `python3 -m edge_tts` when the
// CLI is not on PATH (e.g. pip --user installs on macOS).
func runEdgeTTS(ctx context.Context, voice, txtPath, mp3Path string, rate, pitch int) ([]byte, error) {
	args := []string{"--voice", voice, "--file", txtPath, "--write-media", mp3Path}
	if rate != 0 {
		args = append(args, fmt.Sprintf("--rate=%+d%%", rate))
	}
	if pitch != 0 {
		args = append(args, fmt.Sprintf("--pitch=%+dHz", pitch))
	}
	if path, err := exec.LookPath("edge-tts"); err == nil {
		return exec.CommandContext(ctx, path, args...).CombinedOutput()
	}
	args = append([]string{"-m", "edge_tts"}, args...)
	return exec.CommandContext(ctx, "python3", args...).CombinedOutput()
}

// StartTTSCleaner periodically removes generated .mp3/.txt files older than 10 minutes.
func StartTTSCleaner() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		files, err := os.ReadDir("./tmp")
		if err != nil {
			continue
		}
		now := time.Now()
		for _, f := range files {
			ext := filepath.Ext(f.Name())
			if ext != ".mp3" && ext != ".txt" {
				continue
			}
			path := filepath.Join("./tmp", f.Name())
			info, err := os.Stat(path)
			if err == nil && now.Sub(info.ModTime()) > 10*time.Minute {
				_ = os.Remove(path)
			}
		}
	}
}
