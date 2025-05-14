# Use the official Golang image as the build stage
FROM golang:1.24.2-alpine AS builder

# 安装 gcc 和 libwebp-dev
RUN apk add --no-cache gcc g++ libc-dev libwebp-dev

ENV CGO_ENABLED=1
# Set working directory inside container
WORKDIR /app

# Copy go.mod and go.sum, then download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the Go app
RUN go build -o main .

# Use a minimal image for the final container
FROM alpine:latest

# Install certificates if needed (for HTTPS, etc.)
RUN apk --no-cache add ca-certificates libreoffice openjdk11-jre font-noto-cjk font-roboto font-dejavu
ENV FONT_PATH="/usr/share/fonts/dejavu/DejaVuSansCondensed-Bold.ttf"

WORKDIR /root/

# Copy the compiled binary from builder
COPY --from=builder /app/main .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Expose the port your Gin app runs on
EXPOSE 8080

# Command to run the binary
CMD ["./main"]
