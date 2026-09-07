package video

import (
	"fmt"
	"path/filepath"
	"sync"
)

// run executes the full pipeline for a job in the background:
//
//	goroutine A (revoice):  extract -> transcribe -> rewrite -> edge-tts
//	goroutine B (faceswap): facefusion (global concurrency 1)
//	join:                   mux -> final.mp4
//
// The two lines run in parallel; mux waits for both.
func (s *Store) run(job *Job) {
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		firstErr    error
		newAudio    string // voiceover mp3, empty when !Revoice
		vttFile     string
		swappedFile string // face-swapped video, empty when !Faceswap
	)

	fail := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	if job.Options.Revoice {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dir := job.Dir

			s.setStage(job.ID, StageExtracting)
			wav := filepath.Join(dir, "audio.wav")
			if err := extractAudio(job.InputVideo, wav); err != nil {
				fail(fmt.Errorf("extract audio: %w", err))
				return
			}

			s.setStage(job.ID, StageTranscribing)
			transcriptJSON := filepath.Join(dir, "transcript.json")
			text, err := transcribeAudio(wav, transcriptJSON, job.Options.Language)
			if err != nil {
				fail(fmt.Errorf("transcribe: %w", err))
				return
			}
			if text == "" {
				fail(fmt.Errorf("transcribe: no speech detected"))
				return
			}

			s.setStage(job.ID, StageRewriting)
			text, err = rewriteScript(text, job.Options.Instruction)
			if err != nil {
				fail(fmt.Errorf("rewrite: %w", err))
				return
			}

			s.setStage(job.ID, StageVoicing)
			voice := job.Options.Voice
			if voice == "" {
				voice = defaultVoice()
			}
			mp3 := filepath.Join(dir, "voice.mp3")
			vtt := filepath.Join(dir, "voice.vtt")
			if err := synthesizeVoice(text, voice, mp3, vtt); err != nil {
				fail(fmt.Errorf("edge-tts: %w", err))
				return
			}

			mu.Lock()
			newAudio, vttFile = mp3, vtt
			mu.Unlock()
		}()
	}

	if job.Options.Faceswap {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.setStage(job.ID, StageSwapping)
			out := filepath.Join(job.Dir, "swapped.mp4")
			if err := faceSwap(job.InputFace, job.InputVideo, out); err != nil {
				fail(fmt.Errorf("facefusion: %w", err))
				return
			}
			mu.Lock()
			swappedFile = out
			mu.Unlock()
		}()
	}

	wg.Wait()
	if firstErr != nil {
		s.fail(job.ID, firstErr)
		return
	}

	s.setStage(job.ID, StageMuxing)
	videoTrack := job.InputVideo
	if swappedFile != "" {
		videoTrack = swappedFile
	}
	final := filepath.Join(job.Dir, "final.mp4")
	burn := job.Options.BurnSubtitles && vttFile != ""
	if err := mux(videoTrack, newAudio, vttFile, burn, final); err != nil {
		s.fail(job.ID, fmt.Errorf("mux: %w", err))
		return
	}

	s.finish(job.ID, final)
}
