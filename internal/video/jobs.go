// Package video implements the standalone AI video rework service:
// upload a video, optionally face-swap it (FaceFusion) and/or re-voice it
// (whisper -> Kimi rewrite -> edge-tts), then download the result.
package video

import (
	"fmt"
	"sync"
	"time"
)

// Job stages, reported via GET /jobs/:id.
const (
	StagePending       = "pending"
	StageExtracting    = "extracting"   // ffmpeg: strip audio from source video
	StageTranscribing  = "transcribing" // faster-whisper: audio -> text
	StageRewriting     = "rewriting"    // Kimi: translate/rewrite the script
	StageVoicing       = "voicing"      // edge-tts: text -> new voiceover
	StageSwapping      = "swapping"     // FaceFusion: face swap (global concurrency: 1)
	StageMuxing        = "muxing"       // ffmpeg: combine new audio + video
	StageDone          = "done"
	StageFailed        = "failed"
)

// JobOptions selects which stages run. At least one of Faceswap/Revoice
// must be true. Lipsync is reserved for a future stage (LatentSync).
type JobOptions struct {
	Faceswap       bool   `json:"faceswap"`
	Revoice        bool   `json:"revoice"`
	Voice          string `json:"voice"`           // edge-tts voice id, used when Revoice
	Language       string `json:"language"`        // whisper language hint (zh/en/ja/ko), empty = auto
	Instruction    string `json:"instruction"`     // rewrite instruction for Kimi, optional
	BurnSubtitles  bool   `json:"burn_subtitles"`  // burn whisper/tts subtitles into the picture
	Lipsync        bool   `json:"lipsync"`         // not implemented yet
}

// Job tracks one video rework request through the pipeline.
type Job struct {
	ID        string     `json:"id"`
	Stage     string     `json:"stage"`
	Err       string     `json:"error,omitempty"`
	Options   JobOptions `json:"options"`
	CreatedAt time.Time  `json:"created_at"`

	Dir        string `json:"-"` // ./video-tmp/<id>
	InputVideo string `json:"-"` // absolute-ish path of uploaded video
	InputFace  string `json:"-"` // uploaded face image, empty when !Faceswap
	FinalFile  string `json:"-"` // set when Stage == StageDone
}

// Store is an in-memory job registry. State is lost on restart, which is
// acceptable for a local single-user service.
type Store struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func NewStore() *Store {
	return &Store{jobs: make(map[string]*Job)}
}

// New builds a job with a fresh ID but does NOT register it; fill in
// Dir/InputVideo/InputFace, then call register. This avoids handlers
// observing a half-initialized job.
func (s *Store) New(opts JobOptions) *Job {
	return &Job{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Stage:     StagePending,
		Options:   opts,
		CreatedAt: time.Now(),
	}
}

func (s *Store) register(j *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *Store) setStage(id, stage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Stage = stage
	}
}

func (s *Store) fail(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Stage = StageFailed
		j.Err = err.Error()
	}
}

func (s *Store) finish(id, finalFile string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Stage = StageDone
		j.FinalFile = finalFile
	}
}

// remove drops a job from the registry (used by the cleaner).
func (s *Store) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
}

// faceSwapSem caps concurrent FaceFusion runs at 1: a single run already
// saturates the machine (several GB RAM + full Neural Engine/GPU).
var faceSwapSem = make(chan struct{}, 1)
