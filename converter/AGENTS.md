<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-02 | Updated: 2026-04-02 -->

# converter

## Purpose
Image transformation engine. Provides the `CustomImage` pipeline type, a `Strategy` that maps output format types to concrete `Encoder` implementations, and individual format encoders (WebP, AVIF, JPEG, PNG) built on `bimg`/libvips. Also contains a `Transform` function type and the `WithWidth` resize helper.

## Key Files
| File | Description |
|------|-------------|
| `image/image.go` | `CustomImage`: wraps a `*bimg.Image`, exposes `Decode`, `Transform`, and `Encode` steps |
| `image/strategy.go` | `Strategy`: singleton map from `Type` to `Encoder`; constructed with `MustStrategy(logger)` |
| `image/type.go` | `Type` value type for the five image formats (webp, avif, jpeg, png, svg); implements `encoding.TextUnmarshaler` |
| `image/transform.go` | `Transform` function type; `WithWidth(int)` resizes proportionally using `bimg.EnlargeAndCrop` |
| `image/format/webp.go` | `Webp` encoder: calls `bimg.Process` with `bimg.WEBP` option |
| `image/format/jpeg.go` | `Jpeg` encoder |
| `image/format/png.go` | `Png` encoder |
| `image/format/avif.go` | `Avif` encoder |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `image/` | Core pipeline, strategy, types, and transforms |
| `image/format/` | One file per output format, each implementing the `Encoder` interface |
| `transform/` | Empty placeholder package (`package transform`); reserved for future transform types |

## For AI Agents

### Working In This Directory
- `bimg` wraps libvips. libvips must be present on the host. In Docker, the `Dockerfile` installs it.
- `Strategy` is a singleton (double-checked lock in `MustStrategy`). Do not call `MustStrategy` more than once per process; `main.go` calls it once and passes the result via DI.
- Adding a new format requires: a new `Type` constant in `type.go`, a new encoder file in `format/`, registration in `MustStrategy`, and a regex update in the `api` route if the format is exposed as a proxy output.
- `WithWidth` skips resize when `width == 0` or when the image is already the requested width.
- SVG images bypass the converter entirely (handled in `service/image.go` before `NewCustomImage` is called).

### Testing Requirements
```bash
go test ./...
```
Encoder tests require test image fixtures. Use small JPEG/PNG files. Tests must not depend on network or S3.

### Common Patterns
```go
// Typical usage from service layer
customImage := image.NewCustomImage(strategy.Apply(params.Type))
customImage.Decode(reader)
customImage.Transform(image.WithWidth(params.Width))
reader, contentLength, err := customImage.Encode(ctx, params.Quality)
```
Each encoder calls `img.Process(bimg.Options{Type: bimg.<FORMAT>, Quality: int(quality)})` and returns a `bytes.Buffer` reader.

## Dependencies

### Internal
| Package | Role |
|---------|------|
| `resizer/shared/log` | `LoggerWithTrace` inside format encoders |

### External
| Package | Purpose |
|---------|---------|
| `github.com/h2non/bimg` | libvips bindings for image processing |
| `go.uber.org/zap` | Structured logging in encoders and transforms |

<!-- MANUAL: -->
