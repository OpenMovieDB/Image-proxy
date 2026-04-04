<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-02 | Updated: 2026-04-02 -->

# Image-proxy

## Purpose
Image proxy and resize service for the PoisKino platform. Fetches images from S3 storage or external CDNs (TMDB, Kinopoisk/Yandex), transforms them using libvips via the bimg library, and serves the result over HTTP. S3 acts as a write-through cache for all proxied images. The service runs as a single Go binary with a Fiber v2 HTTP server.

## Key Files
| File | Description |
|------|-------------|
| `main.go` | Entry point: wires Fiber app, AWS S3 client, rate limiter, ETag, compression, OTel tracing, Zap logger, and Swagger UI |
| `go.mod` | Module `resizer`; declares all direct and indirect dependencies |
| `Makefile` | Build, lint (`errcheck`, `goconst`, `gocyclo`), and Swagger gen targets |
| `Dockerfile` | Container image build (requires libvips at runtime) |
| `docker-compose.yaml` | Local development compose file |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `api/` | HTTP layer: Fiber route registration and request/response models (see `api/AGENTS.md`) |
| `service/` | Business logic: S3 fetch, external proxy, async S3 caching, failed-URL tracking (see `service/AGENTS.md`) |
| `converter/` | Image transformation engine: format encoders and resize transforms (see `converter/AGENTS.md`) |
| `config/` | Environment-based configuration struct loaded at startup (see `config/AGENTS.md`) |
| `shared/` | Cross-cutting utilities: Zap logger init and OTel trace helpers (see `shared/AGENTS.md`) |
| `docs/` | Generated Swagger JSON (`make swag-gen`); do not edit manually |
| `infra/` | Kubernetes manifests (`deployment.yaml`, `service.yaml`, `tunnel.yaml`); infrastructure-only |

## For AI Agents

### Working In This Directory
- The module name is `resizer`. All internal imports use `resizer/<package>`.
- libvips must be installed on the build host for `bimg` to compile. In Docker this is handled by the `Dockerfile`.
- Always run `make swag-gen` after changing Swagger annotations before committing.
- Do not edit files under `docs/` directly; they are generated.
- `app` in the root is a compiled binary artifact, not a directory.

### Testing Requirements
```bash
go test ./...
```
Integration tests require a reachable S3 endpoint. Unit tests for converters and models run without external services.

### Common Patterns
- Dependency injection via constructor functions (`NewImageService`, `NewImageController`).
- Context with timeout wraps every external call (S3, HTTP). Timeouts: 10s for S3 reads, 5 min for proxy requests.
- All log calls use `log.LoggerWithTrace(ctx, logger)` to attach OTel trace/span IDs.
- Async S3 cache writes use a semaphore (`chan struct{}`, capacity 50) to bound concurrency.

## Dependencies

### Internal
All packages are internal to the `resizer` module. `main.go` composes them:
`config` → `shared/{log,trace}` → `converter/image` → `service` → `api/rest`

### External
| Package | Purpose |
|---------|---------|
| `github.com/gofiber/fiber/v2` | HTTP server and middleware |
| `github.com/h2non/bimg` | Image processing (wraps libvips) |
| `github.com/aws/aws-sdk-go` | S3 client |
| `github.com/caarlos0/env/v8` | Environment variable parsing |
| `go.uber.org/zap` | Structured logging |
| `go.opentelemetry.io/otel` | Distributed tracing |
| `github.com/hyperdxio/otel-config-go` | HyperDX OTel bootstrap |

<!-- MANUAL: -->
