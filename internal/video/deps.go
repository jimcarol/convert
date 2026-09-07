package video

import (
	"log"
	"os"
	"os/exec"
)

// CheckDeps logs a warning for every external tool the pipeline needs that
// is missing, so setup problems surface at startup instead of mid-job.
// edge-tts and facefusion can also run via `python3 -m`, which is probed
// lazily at job time; here we only check PATH.
func CheckDeps() {
	for _, tool := range []string{"ffmpeg", "python3", "edge-tts", "facefusion"} {
		if _, err := exec.LookPath(tool); err != nil {
			log.Printf("video deps WARNING: %q not found on PATH", tool)
		}
	}
	if os.Getenv("KIMI_API_KEY") == "" {
		log.Printf("video deps note: KIMI_API_KEY not set, rewrite stage will pass the transcript through unchanged")
	}
}
