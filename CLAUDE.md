# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an image proxy service built in Go that processes and serves images from S3 storage. It provides image transformation capabilities (resizing, format conversion, quality adjustment) and supports proxying images from external services like TMDB and Kinopoisk.

## Architecture

### Core Components

- **main.go**: Entry point that sets up Fiber web server, AWS S3 client, health checks, OpenTelemetry tracing, and dependency injection
- **api/rest/image.go**: REST endpoints for image processing and proxying
- **service/image.go**: Business logic for image processing and external service proxying
- **converter/image/**: Image transformation engine using libvips (bimg library)
- **config/**: Environment-based configuration management
- **shared/**: Common utilities for logging and tracing

### Key Dependencies

- **Fiber v2**: Web framework for HTTP handling with built-in healthcheck middleware
- **bimg**: Image processing library (requires libvips)
- **AWS SDK**: S3 storage integration
- **OpenTelemetry**: Distributed tracing and observability
- **Zap**: Structured logging

## Development Commands

### Build and Run

```bash
# Build the application
go build -o app

# Run locally (requires environment variables)
go run main.go

# Build and run with Docker
docker build -t image-proxy .
docker run -p 8080:8080 image-proxy

# Run with Docker Compose
docker-compose up
```

### Code Quality Tools

```bash
# Run all quality checks
make all-check

# Individual checks
make errcheck    # Check for unhandled errors
make goconst     # Find repeated string constants
make gocyclo     # Check cyclomatic complexity
```

### API Documentation

```bash
# Generate Swagger documentation
make swag-gen

# View docs at http://localhost:8080/docs after starting server
```

## Environment Configuration

Required environment variables:
- `S3_BUCKET`: S3 bucket name
- `S3_ACCESS_KEY`: S3 access key
- `S3_SECRET_KEY`: S3 secret key
- `S3_ENDPOINT`: S3 endpoint URL
- `S3_REGION`: S3 region (default: ru-1)

Optional configuration:
- `PORT`: Server port (default: 8080)
- `RATE_LIMIT_MAX_REQUESTS`: Rate limit per duration (default: 100)
- `CACHE_TTL`: In-memory cache TTL (default: 10m)

## API Endpoints

### Image Processing
`GET /images/{entity}/{file}/{width}/{quality}/{type}`
- Processes images from S3 with specified transformations
- Supports JPEG, PNG, WebP, AVIF formats
- Implements caching and rate limiting

### Image Proxying
`GET /{service_type}/{path}`
- Proxies images from external services
- Supported services: tmdb-images, kinopoisk-images, kinopoisk-ott-images, kinopoisk-st-images
- Adds cache headers for browser caching

### Health Check Endpoints
`GET /livez`
- Liveness probe for Kubernetes
- Always returns 200 OK if service is running

`GET /readyz`
- Readiness probe for Kubernetes
- Returns 200 OK if service is ready and S3 is accessible
- Returns 503 Service Unavailable if S3 connection fails

## Key Implementation Details

### Image Processing Pipeline
1. Request validation and parsing (api/rest/image.go:46-73)
2. S3 image retrieval with caching (service/image.go)
3. Image transformation using converter strategy pattern
4. Response streaming with appropriate headers

### Caching Strategy
- S3-based caching for proxied images
- Configurable TTL for in-memory cache
- Cache keys based on transformation parameters

### Error Handling
- Structured logging with OpenTelemetry trace correlation
- Proper HTTP status codes and error responses
- Fallback to external services when S3 cache misses

## Testing

The project uses standard Go testing practices. Run tests with:
```bash
go test ./...
```

## Deployment

Kubernetes manifests are provided in `infra/`:
- `deployment.yaml`: Main application deployment
- `service.yaml`: Service configuration
- `tunnel.yaml`: Ingress/tunnel configuration