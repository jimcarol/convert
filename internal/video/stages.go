package video

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"file-converter/internal/transcribe"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// VoiceOption mirrors the TTS service's voice list (copied rather than
// imported, to keep this service decoupled from handlers/).
type VoiceOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var voices = []VoiceOption{
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

var voiceSet = func() map[string]bool {
	m := make(map[string]bool, len(voices))
	for _, v := range voices {
		m[v.ID] = true
	}
	return m
}()

// languageSet whitelists the whisper language hints accepted from the form;
// empty means auto-detect.
var languageSet = map[string]bool{"zh": true, "en": true, "ja": true, "ko": true}

func defaultVoice() string { return voices[0].ID }

// per-stage timeouts
const (
	extractTimeout    = 2 * time.Minute
	transcribeTimeout = 15 * time.Minute
	rewriteTimeout    = 2 * time.Minute
	voiceTimeout      = 2 * time.Minute
	faceSwapTimeout   = 60 * time.Minute
	muxTimeout        = 5 * time.Minute
)

// runCmd executes a command and returns a trimmed stderr/stdout tail on
// failure, same pattern as the TTS service. The underlying error is always
// included: when the binary is missing entirely ("executable file not
// found") there is no output to show.
func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s timed out", name)
	}
	if err != nil {
		tail := strings.TrimSpace(string(out))
		if len(tail) > 500 {
			tail = tail[len(tail)-500:]
		}
		if tail == "" {
			return fmt.Errorf("%s failed: %v", name, err)
		}
		return fmt.Errorf("%s failed: %v: %s", name, err, tail)
	}
	return nil
}

// extractAudio pulls the audio track out of the source video as 16kHz wav
// (what whisper wants).
func extractAudio(videoPath, wavPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), extractTimeout)
	defer cancel()
	return transcribe.ToWav(ctx, videoPath, wavPath)
}

// transcribeAudio transcribes wavPath via the shared whisper wrapper and
// keeps the segment-level JSON in the job dir for debugging.
func transcribeAudio(wavPath, jsonPath, language string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), transcribeTimeout)
	defer cancel()
	text, err := transcribe.Run(ctx, wavPath, language, jsonPath)
	if errors.Is(err, transcribe.ErrNoSpeech) {
		return "", fmt.Errorf("视频中未检测到清晰人声（可能是纯音乐/背景音视频）")
	}
	return text, err
}

// rewriteScript sends the transcript to Kimi (OpenAI-compatible API) for
// translation / spoken-style rewriting. When KIMI_API_KEY is unset it
// returns the original text unchanged, so the pipeline still works for
// pure face-swap + re-voice jobs.
func rewriteScript(text, instruction string) (string, error) {
	apiKey := os.Getenv("KIMI_API_KEY")
	if apiKey == "" {
		return text, nil
	}
	if instruction == "" {
		instruction = "改写成自然流畅、适合朗读的中文口播稿，长度与原文接近，只输出改写后的正文"
	}

	base := os.Getenv("KIMI_BASE_URL")
	if base == "" {
		base = "https://api.moonshot.cn/v1"
	}
	model := os.Getenv("KIMI_MODEL")
	if model == "" {
		model = "kimi-k2.6"
	}

	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是视频口播文案编辑。" + instruction},
			{"role": "user", "content": text},
		},
		"temperature": 0.6,
	})

	ctx, cancel := context.WithTimeout(context.Background(), rewriteTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("kimi request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kimi returned %s: %s", resp.Status, truncate(string(respBody), 300))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("bad kimi response: %s", truncate(string(respBody), 300))
	}
	rewritten := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if rewritten == "" {
		return text, nil
	}
	return rewritten, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// synthesizeVoice runs edge-tts, producing an mp3 plus a .vtt subtitle file
// (used for optional subtitle burning at mux time). Falls back to
// `python3 -m edge_tts` when the CLI is not on PATH.
func synthesizeVoice(text, voiceID, mp3Path, vttPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), voiceTimeout)
	defer cancel()

	txtPath := mp3Path + ".txt"
	if err := os.WriteFile(txtPath, []byte(text), 0o600); err != nil {
		return err
	}
	defer os.Remove(txtPath)

	args := []string{"--voice", voiceID, "--file", txtPath,
		"--write-media", mp3Path, "--write-subtitles", vttPath}
	if path, err := exec.LookPath("edge-tts"); err == nil {
		return runCmd(ctx, path, args...)
	}
	args = append([]string{"-m", "edge_tts"}, args...)
	return runCmd(ctx, "python3", args...)
}

// faceSwap runs FaceFusion in headless mode. Concurrent runs are capped by
// faceSwapSem; callers may wait in the queue for a while.
//
// FaceFusion CLI flags vary slightly between versions — if your installed
// version differs, this is the single place to adjust.
func faceSwap(facePath, videoPath, outPath string) error {
	faceSwapSem <- struct{}{}
	defer func() { <-faceSwapSem }()

	ctx, cancel := context.WithTimeout(context.Background(), faceSwapTimeout)
	defer cancel()

	args := []string{"headless-run",
		"--source", facePath,
		"--target", videoPath,
		"--output", outPath,
		"--execution-providers", "coreml",
	}
	if path, err := exec.LookPath("facefusion"); err == nil {
		return runCmd(ctx, path, args...)
	}
	args = append([]string{"-m", "facefusion"}, args...)
	return runCmd(ctx, "python3", args...)
}

// mux combines the (possibly face-swapped) video with the new voiceover.
// newAudio may be empty (face-swap-only job): then the original audio is
// kept. vttPath may be empty; when set and burnSubtitles is true the
// subtitles are burned in, which forces a video re-encode.
func mux(videoPath, newAudio, vttPath string, burnSubtitles bool, outPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), muxTimeout)
	defer cancel()

	if newAudio == "" && !burnSubtitles {
		// nothing to combine; the video already carries its own audio
		return os.Rename(videoPath, outPath)
	}

	args := []string{"-y", "-i", videoPath}
	if newAudio != "" {
		args = append(args, "-i", newAudio)
	}

	// map video + audio sources
	args = append(args, "-map", "0:v:0")
	if newAudio != "" {
		args = append(args, "-map", "1:a:0")
	} else {
		args = append(args, "-map", "0:a:0?")
	}

	// burning subtitles forces a video re-encode; otherwise stream-copy.
	// job dirs only contain digits, so the vtt path needs no filter
	// escaping (no ':' or quotes)
	if burnSubtitles && vttPath != "" {
		args = append(args, "-vf", "subtitles="+vttPath,
			"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p")
	} else {
		args = append(args, "-c:v", "copy")
	}
	args = append(args, "-c:a", "aac")
	if newAudio != "" {
		args = append(args, "-shortest")
	}

	args = append(args, outPath)
	return runCmd(ctx, "ffmpeg", args...)
}
