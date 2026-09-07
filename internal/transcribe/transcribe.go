// Package transcribe is the shared whisper wrapper used by both the tts
// service (mic re-voice) and the video service (re-voice line). It shells
// out to scripts/transcribe.py; keeping the invocation and output parsing
// here means the transcript JSON schema is owned in exactly one place.
package transcribe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoSpeech is returned when the audio contains no detectable speech
// (e.g. BGM-only input); transcribe.py exits with code 3 in that case.
var ErrNoSpeech = errors.New("no speech detected in audio")

// ToWav normalizes any audio/video file to 16kHz mono wav, the format
// whisper expects. Caller controls the timeout via ctx.
func ToWav(ctx context.Context, inPath, outPath string) error {
	return run(ctx, "ffmpeg", "-y", "-i", inPath,
		"-vn", "-acodec", "pcm_s16le", "-ar", "16000", "-ac", "1", outPath)
}

// Run transcribes wavPath (16k mono) and returns the full text.
// language is an optional ISO hint (zh/en/ja/ko); empty = auto-detect.
// jsonPath optionally keeps the segment-level transcript for debugging
// (the video service stores it in the job dir); empty = discard.
func Run(ctx context.Context, wavPath, language, jsonPath string) (string, error) {
	discard := jsonPath == ""
	if discard {
		jsonPath = filepath.Join(os.TempDir(), fmt.Sprintf("transcript-%d.json", time.Now().UnixNano()))
		defer os.Remove(jsonPath)
	}

	args := []string{filepath.Join("scripts", "transcribe.py"), wavPath, jsonPath, language}
	cmd := exec.CommandContext(ctx, "python3", args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("transcribe timed out")
	}
	if err != nil {
		if strings.Contains(string(out), "no speech detected") {
			return "", ErrNoSpeech
		}
		return "", fmt.Errorf("transcribe failed: %s", tail(string(out), err))
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("transcribe produced no output: %w", err)
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("bad transcribe output: %w", err)
	}
	text := strings.TrimSpace(result.Text)
	if text == "" {
		return "", ErrNoSpeech
	}
	return text, nil
}

func run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s timed out", name)
	}
	if err != nil {
		return fmt.Errorf("%s failed: %s", name, tail(string(out), err))
	}
	return nil
}

// tail keeps the underlying error plus the last 500 chars of output.
func tail(out string, err error) string {
	out = strings.TrimSpace(out)
	if len(out) > 500 {
		out = out[len(out)-500:]
	}
	if out == "" {
		return err.Error()
	}
	return out
}
