#!/bin/bash
# Download faster-whisper models from ModelScope (国内可达) into a local
# cache dir, since huggingface.co is unreachable from some networks.
#
# Usage: scripts/download_whisper_model.sh [tiny|small|...]
# Default: small. Override cache location with WHISPER_CACHE_DIR.

set -euo pipefail

SIZE="${1:-small}"
CACHE_DIR="${WHISPER_CACHE_DIR:-$HOME/.cache/faster-whisper}"
DEST="$CACHE_DIR/$SIZE"
REPO="pengzhendong/faster-whisper-$SIZE"

if [ -f "$DEST/model.bin" ]; then
  echo "already downloaded: $DEST"
  exit 0
fi

mkdir -p "$DEST"
for f in config.json model.bin tokenizer.json vocabulary.txt; do
  echo "downloading $REPO/$f ..."
  curl -fSL --retry 3 -o "$DEST/$f" \
    "https://modelscope.cn/models/$REPO/resolve/master/$f"
done

echo "done: $DEST"
