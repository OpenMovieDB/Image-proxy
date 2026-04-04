<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-02 | Updated: 2026-04-02 -->

# config

## Purpose
Environment-based configuration for the image proxy service. Defines a single `Config` struct whose fields are populated from environment variables at startup using `caarlos0/env`. The service panics on startup if any required variable is missing.

## Key Files
| File | Description |
|------|-------------|
| `config.go` | `Config` struct with all service settings; `New()` parses env vars and returns a populated `*Config` |

## Subdirectories
None.

## For AI Agents

### Working In This Directory
- `New()` panics if a field tagged `required` is absent. Required fields are: `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_ENDPOINT`.
- `S3_REGION` has no default; if omitted the AWS SDK will use its own default region logic.
- `TMDB_IMAGE_PROXY` has no default and is optional. When empty, `ServiceName.ToProxyURL` for `TmdbImages` returns a URL prefixed with an empty string — ensure this is set in all proxy environments.
- To add a new configuration field: add it to the `Config` struct with an `env:"VAR_NAME"` tag (add `envDefault:"..."` or `required` as appropriate), then update this file and the root `CLAUDE.md` environment section.
- Do not add logic to this package beyond parsing.

### Testing Requirements
```bash
go test ./...
```
Test `New()` by setting environment variables before the call. Use `t.Setenv` in Go test files to avoid polluting the global environment.

### Common Patterns
```go
// Struct tag examples
Port    string        `env:"PORT" envDefault:"8080"`
S3Bucket string       `env:"S3_BUCKET,required"`
CacheTTL time.Duration `env:"CACHE_TTL" envDefault:"10m"`
```
All `time.Duration` fields accept standard Go duration strings (e.g., `"10m"`, `"1s"`).

## Dependencies

### Internal
None.

### External
| Package | Purpose |
|---------|---------|
| `github.com/caarlos0/env/v8` | Struct tag-based environment variable parsing |

<!-- MANUAL: -->
