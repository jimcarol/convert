# Image Conversion Service

This is an image conversion service built using Go and the Gin framework. The service supports converting uploaded images to WebP format, with additional features for downloading the converted files and automatic cleaning of temporary files. It also includes a text-to-speech (TTS) feature powered by edge-tts (Microsoft Edge TTS voices, including Taiwan/HK Mandarin).

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Setup](#setup)
4. [Usage](#usage)
5. [API Endpoints](#api-endpoints)
6. [Auto Cleanup](#auto-cleanup)
7. [License](#license)

---

## Prerequisites

Before you start, ensure that the following software is installed:

- **Go 1.18+**: [Install Go](https://golang.org/dl/)
- **Git**: [Install Git](https://git-scm.com/)
- **Go modules**: Make sure Go modules are enabled (`GO_MODULE=on` by default in Go 1.16+)

Additionally, ensure you have the following libraries:

- `github.com/gin-gonic/gin` for the web server.
- `github.com/chai2010/webp` for WebP image encoding.

You can install them using:

```bash
go get github.com/gin-gonic/gin
go get github.com/chai2010/webp
go get github.com/signintech/gopdf
```

## Installation
1. Clone the repository
```bash
git clone https://github.com/yourusername/image-conversion-service.git
cd image-conversion-service
```

2. Install dependencies
In your project directory, install the necessary Go dependencies:

```bash
go mod tidy
```
This will download the required libraries and set up Go modules for the project.

## Setup
1. Directory Structure
Ensure the following directory structure:

```
/image-conversion-service
├── /templates
│   └── index.html  (HTML file for the frontend)
├── /static
│   └── (Static files such as CSS, JS, etc.)
├── /tmp
│   └── (Temporary files for uploaded images and conversions)
├── main.go         (Go server code)
└── README.md       (This file)
```

Make sure the templates and static folders exist, and the tmp directory will be created automatically when the service starts.

2. Create Temporary Directories
If not already created, the tmp directory will be created automatically when the server starts. This folder will store the uploaded and converted images temporarily.

3. Running the Service
To start the service, run the following command in your terminal:

```bash
go run main.go
```

The service will start running on http://localhost:8080.

## Usage
1. Uploading Images for Conversion
Endpoint: POST /convert

Request: This endpoint accepts an image file upload and a target format (currently only "webp" is supported).

file: The image file you want to convert (JPEG or PNG only).

target: The desired output format (currently only webp is supported).

Example request using curl:

```bash

curl -X POST -F "file=@/path/to/image.png" -F "target=webp" http://localhost:8080/convert
```

2. Downloading Converted Image
Once the image is successfully converted, you will receive a download URL in the response.

Example response:

```json
{
  "download_url": "/download/1637079733000000000.webp"
}
```
You can download the converted image by visiting the URL:

```bash
http://localhost:8080/download/1637079733000000000.webp
```
## API Endpoints

POST /convert
Converts an uploaded image to the target format (currently only webp).

Request Parameters:

file: The image file to convert (JPEG or PNG).

target: The desired format (currently only webp).

Response:

download_url: The URL to download the converted file.

GET /download/:filename
Downloads a converted image by its filename.

Request Parameters:

filename: The name of the file you want to download (e.g., 1637079733000000000.webp).

GET /
Renders the index.html page (you can use this to create a frontend for uploading images).

## Text to Speech (TTS)

Page: `/tts` — enter text, pick a voice, generate an mp3 you can play and download. Uses the edge-tts CLI (requires Python 3 + `pip install edge-tts` on the host; already baked into Dockerfile.tts).

GET /tts/voices
Returns the selectable voice list (`{"voices": [{"id": "zh-TW-HsiaoChenNeural", "label": "..."}, ...]}`).

POST /tts
Converts text to speech.

Request (JSON):

```json
{
  "text": "要转换的文字（最多 3000 字）",
  "voice": "zh-TW-HsiaoChenNeural",
  "pitch": 45,
  "rate": -10
}
```

`pitch`（-50 ~ 50，单位 Hz）和 `rate`（-50 ~ 100，单位 %）可选，默认 0（声音原始音调/语速）。调高 pitch 会明显变「嗲」。

Response:

```json
{
  "download_url": "/tts/download/1787567779622397000.mp3"
}
```

GET /tts/download/:filename
Downloads a generated mp3. Files live in `./tmp` and are auto-cleaned (older than 10 minutes).

All TTS endpoints except the page itself require login (JWT cookie). Voice names are validated against a server-side whitelist in `handlers/tts.go`.

## Auto Cleanup
The service automatically cleans up the tmp directory every 5 minutes. Any files older than 10 minutes will be deleted. This helps keep the server disk clean and free of unnecessary files.

## Notes for New Developers
1. Add New Conversion Formats
If you want to add support for additional image formats:

Implement a function similar to ConvertToWebP for the new format (e.g., ConvertToJpeg, ConvertToPng).

Add a corresponding check in the ConvertHandler function to handle the new format.

2. Frontend Development
The index.html file in the templates folder is used as the frontend. You can edit this file to create a more user-friendly interface for image uploads and conversion.

3. Logging and Debugging
The service uses gin's built-in logging, so you'll see logs in the terminal as the server runs.

If you encounter any issues, check the logs for more details.

# Docker
```shell
docker buildx build --platform linux/amd64 --no-cache -t  converter:${tag_name}-amd64 .
docker tag converter:${tag_name}-amd64 jimhsx/convert:${tag_name}-amd64 
docker push jimhsx/convert:${tag_name}-amd64
```

## Docker (Split lite/heavy)

### Build

The heavy image's runtime deps (LibreOffice + JRE + CJK fonts) live in a separate
base image (`Dockerfile.heavy-base`, tagged `converter-heavy-base:v1`). Build it
**once** per machine — rebuild only when those deps change:

```shell
docker buildx build --platform linux/amd64 -f Dockerfile.heavy-base \
  -t converter-heavy-base:v1 --load .
```

`Dockerfile.heavy` does `FROM converter-heavy-base:v1`, so the heavy build below
fails if the base image is missing locally. To bump base deps: tag a new version
(`v2`) and update the `FROM` in `Dockerfile.heavy` plus the `heavy-base` service
image in `docker-compose.yml`. (Pushing the base to a registry is unnecessary in the
usual build-here / pull-on-server flow — only useful if you ever build heavy on a
second machine.)

Each build stamps the image with labels (version, git commit, build date, feature list)
via build args, so you can later inspect what a given image contains:

```shell
export VERSION="${tag_name}"
export GIT_COMMIT="$(git rev-parse --short HEAD)"
export BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export FEATURES="tts"   # edit per release

docker buildx build --platform linux/amd64 --no-cache -f Dockerfile.lite \
  --build-arg VERSION --build-arg GIT_COMMIT --build-arg BUILD_DATE --build-arg FEATURES \
  -t converter-lite:${tag_name}-amd64 .
docker buildx build --platform linux/amd64 --no-cache -f Dockerfile.heavy \
  --build-arg VERSION --build-arg GIT_COMMIT --build-arg BUILD_DATE --build-arg FEATURES \
  -t converter-heavy:${tag_name}-amd64 .
docker buildx build --platform linux/amd64 --no-cache -f Dockerfile.tts \
  --build-arg VERSION --build-arg GIT_COMMIT --build-arg BUILD_DATE \
  -t converter-tts:${tag_name}-amd64 .
```

### Inspect image labels
```shell
docker inspect -f '{{json .Config.Labels}}' jimhsx/convert-lite:${tag_name}-amd64 | jq
docker inspect -f '{{json .Config.Labels}}' jimhsx/convert-tts:${tag_name}-amd64 | jq
docker inspect -f '{{ index .Config.Labels "app.features" }}' jimhsx/convert-lite:${tag_name}-amd64
```

### Tag + Push
```shell
docker tag converter-lite:${tag_name}-amd64 jimhsx/convert-lite:${tag_name}-amd64
docker tag converter-tts:${tag_name}-amd64 jimhsx/convert-tts:${tag_name}-amd64
docker tag converter-heavy:${tag_name}-amd64 jimhsx/convert-heavy:${tag_name}-amd64

docker push jimhsx/convert-lite:${tag_name}-amd64
docker push jimhsx/convert-tts:${tag_name}-amd64
docker push jimhsx/convert-heavy:${tag_name}-amd64
```

### Caddy Reverse Proxy (keep existing URL paths)
Use [Caddyfile.example](./Caddyfile.example), or copy:

```caddyfile
:80 {
    @heavy_convert path /concat /convert /upload-gif
    @heavy_download path /download/*
    @tts path /tts /tts/*

    reverse_proxy @heavy_convert heavy:8080
    reverse_proxy @heavy_download heavy:8080
    reverse_proxy @tts tts:8080

    reverse_proxy lite:8080
}
```

### Runtime ENV
- `lite` service: `JWT_SECRET`, `INVITE_CODES` (multi-user invite codes) and/or `AUTH_PASSWORD` (legacy admin login), `DATA_DIR` (optional, default `./data`), `PORT` (optional, default `8080`)
- `heavy` service: `JWT_SECRET`, `PORT` (optional, default `8080`)
- `tts` service: `JWT_SECRET`, `PORT` (optional, default `8080`)

Set the same `JWT_SECRET` for all services so one login token works across all upstreams.

### Multi-user login (invite codes)

`lite` supports multi-user login via admin-configured invite codes:

```shell
INVITE_CODES='alice=code-a1b2c3,bob=code-x9y8z7'
```

- Format: comma-separated `username=code` pairs. Usernames: `[a-zA-Z0-9_-]` only (used in data filenames); codes shorter than 8 chars trigger a startup warning.
- Users log in with the invite code on any login box (the request field is still `password` for backward compatibility with the bundled password-x frontend).
- `AUTH_PASSWORD` remains as an `admin` fallback channel. At least one of `INVITE_CODES` / `AUTH_PASSWORD` must be set; production should prefer invite codes only.
- **Data isolation**: each user's notes/passwords live in separate files — `data/notes-<user>.json`, `data/passwords-<user>.json` (directory configurable via `DATA_DIR`). Users can only see their own data.
- **Migration**: on first startup, legacy `notes.json` / `passwords.json` in the working directory are renamed to the first invited user's files (or `admin` when only `AUTH_PASSWORD` is set).
- **Revocation**: remove a user from `INVITE_CODES` and restart — their existing cookies immediately return 401 on `lite` routes. Note: `heavy`/`tts` only verify the JWT signature (they hold no user data), so a revoked user's token stays usable there until it expires (7 days).
- **Login-gated pages**: `/file-convert`, `/png-to-pdf` and `/gif-generate` require a valid login — unauthenticated (or revoked) visits get a 302 redirect to `/`, where the login dialog lives. `/online-note` and `/password-x` stay public because they have their own built-in login UI.

### Local Development with docker-compose
Start split services locally:

```shell
# first time only: heavy's base image (LibreOffice etc., large download, one-time)
docker compose --profile base build heavy-base
# `docker compose up --build` builds the heavy service, which requires this base.
# Note: heavy + heavy-base are pinned to `platform: linux/amd64` (matches the server);
# on Apple Silicon they build/run emulated — slower, but only one base arch ever exists.
# Use docker-compose.dev.yml for fast native daily development instead.

AUTH_PASSWORD='your-password' JWT_SECRET='your-jwt-secret' docker compose up --build
# or with invite codes:
INVITE_CODES='alice=code-a1b2c3,bob=code-x9y8z7' JWT_SECRET='your-jwt-secret' docker compose up --build
```

Then open:
- `http://localhost:8080`

Compose services:
- `lite` for auth/notes/passwords/pages
- `heavy` for `/concat`, `/convert`, `/upload-gif`, `/download/*`
- `tts` for `/tts` page, `POST /tts`, `/tts/voices`, `/tts/download/*` (includes Python + edge-tts)
- `caddy` for path-based reverse proxy

Stop and remove containers:

```shell
docker compose down
```

---

## Video Service (`cmd/video`, standalone)

AI video rework pipeline: upload a video, optionally **face-swap** it (FaceFusion) and/or **re-voice** it (faster-whisper → Kimi rewrite → edge-tts), then download the result. Runs as an independent process (default `127.0.0.1:8090`), separate from `lite`/`heavy`/`tts`.

### Pipeline

```
video.mp4 + face.png
├── revoice line:  ffmpeg extract -> faster-whisper transcribe -> Kimi rewrite -> edge-tts
├── faceswap line: facefusion headless-run (--execution-providers coreml, concurrency = 1)
└── join:          ffmpeg mux -> final.mp4
```

### Prerequisites (local, Apple Silicon)

```bash
brew install ffmpeg
pip3 install faster-whisper edge-tts
scripts/download_whisper_model.sh small   # whisper 模型走 ModelScope（huggingface.co 国内不可达）
# FaceFusion: see https://docs.facefusion.io — needs its own venv + onnxruntime
```

- `KIMI_API_KEY` (optional): enables script rewriting via Kimi. Without it the original transcript is voiced as-is.
- `KIMI_BASE_URL` / `KIMI_MODEL` (optional): default `https://api.moonshot.cn/v1` / `kimi-k2.6`.
- `WHISPER_MODEL` (optional): default `small`; `WHISPER_CACHE_DIR` (default `~/.cache/faster-whisper`).

### Run

```bash
go run ./cmd/video          # or: go build -o bin/video ./cmd/video
# env: BIND (default 127.0.0.1), PORT (default 8090), VIDEO_TMP_DIR (default ./video-tmp)
```

### API

| Endpoint | Description |
|---|---|
| `POST /jobs` | multipart: `video` (file), `face` (file, required when `faceswap=1`), fields: `faceswap`, `revoice`, `voice`, `instruction`, `burn_subtitles` → `{job_id}` |
| `GET /jobs/:id` | poll job stage: `extracting → transcribing → rewriting → voicing → swapping → muxing → done` |
| `GET /jobs/:id/download` | download `final.mp4` when done |
| `GET /voices` | list available edge-tts voices |

Example:

```bash
curl -F "video=@in.mp4" -F "face=@me.png" -F "faceswap=1" -F "revoice=1" \
     -F "voice=zh-CN-YunxiNeural" http://127.0.0.1:8090/jobs
curl http://127.0.0.1:8090/jobs/<job_id>
```

Job artifacts live in `./video-tmp/<job_id>/` and are cleaned up after 1 hour.
