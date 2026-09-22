package handlers

import (
	"context"
	"errors"
	"file-converter/internal/auth"
	ttsqueue "file-converter/internal/tts"
	"file-converter/middleware"
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
	ttsWorkers  = 2 // 同时合成的任务数上限
)

// ttsQueue 由 cmd/tts 启动时注入（见 InitTTSQueue）。
var ttsQueue *ttsqueue.Queue

// InitTTSQueue 注入任务队列，并返回队列实例供调用方启动 worker。
func InitTTSQueue() *ttsqueue.Queue {
	ttsQueue = ttsqueue.NewQueue(ttsWorkers, RunEdgeTTSJob)
	return ttsQueue
}

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

// TTSHandler 校验参数后把合成任务入队，立即返回 202 + job_id；
// 前端轮询 GET /tts/jobs/:id 拿结果。admin 任务在队列中插队。
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

	username := middleware.Username(c)
	job, position, err := ttsQueue.Enqueue(username, username == auth.AdminUsername, text, req.Voice, req.Rate, req.Pitch)
	if errors.Is(err, ttsqueue.ErrQueueFull) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "队列已满，请稍后再试"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "enqueue failed"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":     job.ID,
		"status_url": "/tts/jobs/" + job.ID,
		"position":   position,
	})
}

// RunEdgeTTSJob 是注入队列的执行函数：写临时 txt、调 edge-tts、校验产物，
// 返回 mp3 文件名。ctx 由队列 worker 带 120s 超时（不绑 HTTP 请求）。
func RunEdgeTTSJob(ctx context.Context, j *ttsqueue.Job) (string, error) {
	txtPath := filepath.Join("./tmp", j.ID+".txt")
	mp3Name := j.ID + ".mp3"
	mp3Path := filepath.Join("./tmp", mp3Name)

	if err := os.WriteFile(txtPath, []byte(j.Text), 0o600); err != nil {
		return "", errors.New("failed to write temp file")
	}
	defer os.Remove(txtPath)

	out, err := runEdgeTTS(ctx, j.Voice, txtPath, mp3Path, j.Rate, j.Pitch)
	if ctx.Err() == context.DeadlineExceeded {
		return "", errors.New("TTS timeout")
	}
	if err != nil {
		errOut := string(out)
		if len(errOut) > 500 {
			errOut = errOut[:500]
		}
		return "", fmt.Errorf("TTS failed: %s", errOut)
	}

	if _, err := os.Stat(mp3Path); err != nil {
		return "", errors.New("TTS failed: no output file")
	}
	return mp3Name, nil
}

// ttsJobVisible 校验当前用户能否看到该任务：owner 或 admin。
// 对他人任务返回 404，不暴露任务存在性（文本内容有隐私性）。
func ttsJobVisible(c *gin.Context) (*ttsqueue.Job, int, bool) {
	job, position, ok := ttsQueue.Get(c.Param("id"))
	username := middleware.Username(c)
	if !ok || (job.Owner != username && username != auth.AdminUsername) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return nil, -1, false
	}
	return job, position, true
}

// TTSJobHandler 返回任务状态；queued 时带 position，done 时带 download_url。
func TTSJobHandler(c *gin.Context) {
	job, position, ok := ttsJobVisible(c)
	if !ok {
		return
	}
	resp := gin.H{"id": job.ID, "status": job.Status}
	switch job.Status {
	case ttsqueue.StatusQueued:
		resp["position"] = position
	case ttsqueue.StatusDone:
		resp["download_url"] = "/tts/download/" + job.MP3Name
	case ttsqueue.StatusFailed:
		resp["error"] = job.Err
	}
	c.JSON(http.StatusOK, resp)
}

// TTSJobsHandler 返回当前用户的任务列表（倒序），供「我的任务」历史区块
// 和刷新页面后恢复进行中任务使用。
func TTSJobsHandler(c *gin.Context) {
	const excerptRunes = 40
	list := ttsQueue.List(middleware.Username(c))
	jobs := make([]gin.H, 0, len(list))
	for _, item := range list {
		j := item.Job
		entry := gin.H{
			"id":         j.ID,
			"status":     j.Status,
			"created_at": j.CreatedAt,
		}
		runes := []rune(j.Text)
		if len(runes) > excerptRunes {
			entry["text_excerpt"] = string(runes[:excerptRunes]) + "…"
		} else {
			entry["text_excerpt"] = j.Text
		}
		switch j.Status {
		case ttsqueue.StatusQueued:
			entry["position"] = item.Position
		case ttsqueue.StatusDone:
			entry["download_url"] = "/tts/download/" + j.MP3Name
		case ttsqueue.StatusFailed:
			entry["error"] = j.Err
		}
		jobs = append(jobs, entry)
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// TTSCancelHandler 取消排队中的任务（running 不可打断）。
func TTSCancelHandler(c *gin.Context) {
	job, _, ok := ttsJobVisible(c)
	if !ok {
		return
	}
	if !ttsQueue.Cancel(job.ID) {
		c.JSON(http.StatusConflict, gin.H{"error": "任务已开始执行，无法取消"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "cancelled"})
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

// StartTTSCleaner periodically removes generated .mp3/.txt files older than 15 minutes.
// 15 分钟与队列的 jobTTL 对齐：历史列表可见期内下载链接保持有效。
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
			if err == nil && now.Sub(info.ModTime()) > 15*time.Minute {
				_ = os.Remove(path)
			}
		}
	}
}
