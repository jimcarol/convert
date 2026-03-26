# Image Conversion Service

This is an image conversion service built using Go and the Gin framework. The service supports converting uploaded images to WebP format, with additional features for downloading the converted files and automatic cleaning of temporary files.

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
```shell
docker buildx build --platform linux/amd64 --no-cache -f Dockerfile.lite -t converter-lite:${tag_name}-amd64 .
docker buildx build --platform linux/amd64 --no-cache -f Dockerfile.heavy -t converter-heavy:${tag_name}-amd64 .
```

### Tag + Push
```shell
docker tag converter-lite:${tag_name}-amd64 jimhsx/convert-lite:${tag_name}-amd64
docker tag converter-heavy:${tag_name}-amd64 jimhsx/convert-heavy:${tag_name}-amd64

docker push jimhsx/convert-lite:${tag_name}-amd64
docker push jimhsx/convert-heavy:${tag_name}-amd64
```

### Caddy Reverse Proxy (keep existing URL paths)
Use [Caddyfile.example](./Caddyfile.example), or copy:

```caddyfile
:80 {
    @heavy_convert path /concat /convert /upload-gif
    @heavy_download path /download/*

    reverse_proxy @heavy_convert heavy:8080
    reverse_proxy @heavy_download heavy:8080

    reverse_proxy lite:8080
}
```

### Runtime ENV
- `lite` service: `AUTH_PASSWORD`, `JWT_SECRET`, `PORT` (optional, default `8080`)
- `heavy` service: `JWT_SECRET`, `PORT` (optional, default `8080`)

Set the same `JWT_SECRET` for both services so one login token works across both upstreams.

### Local Development with docker-compose
Start split services locally:

```shell
AUTH_PASSWORD='your-password' JWT_SECRET='your-jwt-secret' docker compose up --build
```

Then open:
- `http://localhost:8080`

Compose services:
- `lite` for auth/notes/passwords/pages
- `heavy` for `/concat`, `/convert`, `/upload-gif`, `/download/*`
- `caddy` for path-based reverse proxy

Stop and remove containers:

```shell
docker compose down
```
