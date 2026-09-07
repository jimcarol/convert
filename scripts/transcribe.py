#!/usr/bin/env python3
"""faster-whisper CLI wrapper for the video service.

Usage: python3 scripts/transcribe.py <audio.wav> <output.json> [language]

Writes {"text": ..., "segments": [{start, end, text}, ...]} to output.json.
Model size is controlled via WHISPER_MODEL (default "small"); on an M-series
Mac "small" with int8 is a good speed/accuracy balance (~1-2 min per minute
of audio). Requires: pip install faster-whisper

language: optional ISO code (zh, en, ja, ...); auto-detected when omitted.
Exits with code 3 when no speech is detected (e.g. BGM-only videos), so
callers can fail the job with a clear message instead of passing whisper's
hallucinated text downstream.
"""

import json
import os
import sys


def resolve_model(model_size: str) -> str:
    """Prefer a locally cached model dir (see scripts/download_whisper_model.sh);
    fall back to downloading from HuggingFace Hub."""
    if os.path.isdir(model_size):
        return model_size
    cache = os.environ.get(
        "WHISPER_CACHE_DIR", os.path.expanduser("~/.cache/faster-whisper")
    )
    local = os.path.join(cache, model_size)
    if os.path.isfile(os.path.join(local, "model.bin")):
        return local
    return model_size  # faster-whisper will try HF Hub


def main() -> int:
    if len(sys.argv) not in (3, 4):
        print("usage: transcribe.py <audio.wav> <output.json> [language]", file=sys.stderr)
        return 2

    wav_path, out_path = sys.argv[1], sys.argv[2]
    language = sys.argv[3] if len(sys.argv) == 4 and sys.argv[3] else None

    try:
        from faster_whisper import WhisperModel
    except ImportError:
        print("faster-whisper not installed: pip install faster-whisper", file=sys.stderr)
        return 1

    model_size = os.environ.get("WHISPER_MODEL", "small")
    model = WhisperModel(resolve_model(model_size), device="cpu", compute_type="int8")

    segments_iter, info = model.transcribe(
        wav_path, beam_size=5, language=language, vad_filter=True
    )
    segments = [
        {
            "start": round(s.start, 3),
            "end": round(s.end, 3),
            "text": s.text.strip(),
            "avg_logprob": round(s.avg_logprob, 3),
        }
        for s in segments_iter
    ]
    segments = [s for s in segments if s["text"]]

    # Hallucination guard: on non-speech audio (BGM-only videos) whisper
    # still "transcribes" something, but segment confidence collapses
    # (real speech scores ≈ -0.2~-0.8, hallucinations < -2). Drop
    # low-confidence segments; if nothing survives, report no speech.
    MIN_AVG_LOGPROB = -1.0
    segments = [s for s in segments if s["avg_logprob"] >= MIN_AVG_LOGPROB]

    if not segments:
        print("no speech detected in audio", file=sys.stderr)
        return 3

    result = {
        "text": "".join(s["text"] for s in segments).strip(),
        "language": info.language,
        "segments": segments,
    }
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(result, f, ensure_ascii=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
