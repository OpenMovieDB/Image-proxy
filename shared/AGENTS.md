<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-02 | Updated: 2026-04-02 -->

# shared

## Purpose
Cross-cutting utilities used by every other package. Provides two thin wrappers: a Zap logger initializer that fans log output to both the console and an OTLP exporter (HyperDX), and an OpenTelemetry tracer provider initializer that exports traces to stdout and registers the global OTel propagator.

## Key Files
| File | Description |
|------|-------------|
| `log/log.go` | `InitLogger(ctx)` creates a tee-core Zap logger (OTLP + console). `LoggerWithTrace(ctx, logger)` enriches a logger with `trace_id` and `span_id` from the active span. |
| `trace/trace.go` | `InitTrace()` creates an SDK `TracerProvider` with stdout exporter and `AlwaysSample`, registers it as the global OTel provider, and sets W3C TraceContext + Baggage propagators. |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `log/` | Logger initialization and trace-enriched logger helper |
| `trace/` | OTel tracer provider initialization |

## For AI Agents

### Working In This Directory
- `InitLogger` and `InitTrace` are called once in `main.go`. Do not call them more than once per process.
- Always use `LoggerWithTrace(ctx, logger)` inside any function that receives a `context.Context`. This is the only way trace/span IDs appear in log output.
- `InitTrace` uses `stdout.New` as a secondary exporter for local debugging. In production, the HyperDX OTLP exporter configured by `otelconfig.ConfigureOpenTelemetry()` in `main.go` is the primary sink.
- Do not add business logic to this package. It is infrastructure-only.

### Testing Requirements
```bash
go test ./...
```
Both files have no complex logic and are typically covered by integration-level tests that verify log output contains trace fields. Unit tests can call `InitLogger` and `InitTrace` directly with no external dependencies.

### Common Patterns
```go
// In any handler or service method:
logger := log.LoggerWithTrace(ctx, i.logger)
logger.Info("message", zap.String("key", "value"))
```

## Dependencies

### Internal
None. `shared` has no imports from other `resizer/` packages.

### External
| Package | Purpose |
|---------|---------|
| `go.uber.org/zap` | Core logger type |
| `github.com/hyperdxio/opentelemetry-go/otelzap` | OTel Zap core for OTLP log export |
| `github.com/hyperdxio/opentelemetry-logs-go` | OTLP log exporter and SDK |
| `go.opentelemetry.io/otel` | Global tracer provider and propagator registration |
| `go.opentelemetry.io/otel/sdk` | `TracerProvider`, `Resource`, sampler |
| `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` | Stdout trace exporter for local debugging |

<!-- MANUAL: -->
