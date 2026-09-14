#!/usr/bin/env bash
# 构建 heavy 镜像;base 镜像(converter-heavy-base:v1,预装 LibreOffice 等大依赖)
# 不存在时先自动构建。base 只在依赖变更时才需要重建(bump tag)。
set -euo pipefail
cd "$(dirname "$0")/.."

BASE_IMAGE="converter-heavy-base:v1"

if ! docker image inspect "$BASE_IMAGE" >/dev/null 2>&1; then
  echo "base image $BASE_IMAGE not found, building it first..."
  docker compose --profile base build heavy-base
fi

docker compose build heavy "$@"
