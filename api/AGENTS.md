<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-02 | Updated: 2026-04-02 -->

# api

## Purpose
HTTP layer of the image proxy service. Contains the Fiber route controller that registers all endpoints and the request/response model types. This package translates raw HTTP input into domain calls on `service.ImageService` and writes results back as streamed image responses.

## Key Files
| File | Description |
|------|-------------|
| `rest/image.go` | `ImageController`: registers five routes, handles request parsing, delegates to `ImageService`, streams image bytes back to the client |
| `model/image.go` | `ImageRequest` (entity, file, width, quality, type) and `ImageResponse` (body reader, content-length, content-disposition, type) |
| `model/serivce-type.go` | `ServiceName` value type for the four supported proxy services; maps service name strings to upstream CDN base URLs |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `rest/` | Fiber controller implementation |
| `model/` | Request/response structs and the `ServiceName` enum |

## For AI Agents

### Working In This Directory
- Route registration happens inside `NewImageController`; routes must have matching Swagger annotations.
- Run `make swag-gen` from the repo root after changing any `@Router`, `@Param`, `@Success`, or `@Tags` annotations.
- The `service_type` path parameter is validated by a Fiber regex constraint:
  `tmdb-images|kinopoisk-images|kinopoisk-ott-images|kinopoisk-st-images`
  Adding a new service requires updating this regex, `ServiceName` in `model/serivce-type.go`, and `service.ProxyImage`.
- Use `log.LoggerWithTrace(ctx, logger)` for all log calls inside handlers; never use the bare `logger` field directly.
- Each handler creates its own `context.WithTimeout`; do not change these without updating the service-layer timeouts.

### Testing Requirements
```bash
go test ./...
```
Controller tests require a mock or real `ImageService`. Test request parsing by constructing a `*fiber.Ctx` with `fiber_test` helpers.

### Common Patterns
- Params are parsed with `c.ParamsParser(&model.ImageRequest{})` which unmarshals path params into the struct.
- Responses are streamed with `c.SendStream(image.Body)` — never buffer the full body in the handler.
- Proxy handler forwards all upstream headers except `Server`.

## Dependencies

### Internal
| Package | Role |
|---------|------|
| `resizer/service` | Business logic called by every handler |
| `resizer/config` | `Config` passed to controller constructor |
| `resizer/converter/image` | `image.Type` used in `ImageRequest` |
| `resizer/shared/log` | `LoggerWithTrace` |

### External
| Package | Purpose |
|---------|---------|
| `github.com/gofiber/fiber/v2` | Route registration and context |
| `go.uber.org/zap` | Structured logging |

<!-- MANUAL: -->
