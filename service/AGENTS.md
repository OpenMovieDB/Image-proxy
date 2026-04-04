<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-02 | Updated: 2026-04-02 -->

# service

## Purpose
Business logic layer. `ImageService` implements two primary operations: `Process` (fetch an image from S3, transform it with the converter, return the result) and `ProxyImage` (serve an image from external CDNs with S3 as a write-through cache). Also manages a `failed_urls.txt` file that records every upstream URL that returned a non-200 status, with admin endpoints to retrieve and clear it.

## Key Files
| File | Description |
|------|-------------|
| `image.go` | `ImageService` struct, constructor `NewImageService`, and all methods: `Process`, `ProxyImage`, `tryGetFromS3`, `fetchFromExternalService`, `cacheInS3`, `deleteFromS3`, `isHTMLContent`, `isValidImageResponse`, `isValidImageBySignature`, `logFailedURL`, `ClearFailedURLs`, `Close` |

## Subdirectories
None.

## For AI Agents

### Working In This Directory
- All public methods accept a `context.Context` as their first argument. Always propagate context; never use `context.Background()` inside request-scoped calls.
- `cacheInS3` and `deleteFromS3` are called with `go` (fire-and-forget). They must not mutate shared state except through the semaphore and the logger.
- The cache semaphore (`chan struct{}`, capacity 50) prevents S3 write storms. Do not increase this without profiling memory usage.
- `failed_urls.txt` is opened at startup and kept open for the lifetime of the service. Always use `failedURLsMutex` when reading or writing this file. Call `service.Close()` on graceful shutdown.
- HTML detection (`isHTMLContent`) guards against CDNs returning error pages that get cached in S3. If an HTML object is found in S3 it is deleted asynchronously and the request falls through to the external service.
- `ErrNotFound` is exported for callers that need to distinguish a missing S3 object from other errors.

### Testing Requirements
```bash
go test ./...
```
`ImageService` depends on `*s3.S3` (AWS SDK). Use an S3-compatible local server (e.g., MinIO) or mock the interface for unit tests. HTTP calls in `fetchFromExternalService` can be intercepted with `httptest.NewServer`.

### Common Patterns

**Process flow (S3 image with transformation):**
```
getFromS3 → check SVG bypass → NewCustomImage → Decode → Transform → Encode → ImageResponse
```

**ProxyImage flow:**
```
tryGetFromS3 (cache hit?) → fetchFromExternalService (on miss) → go cacheInS3 (async) → return ProxyResponse
```

**S3 key format for proxied images:**
```
proxy/<service_type>/<raw_path>
```

**S3 key format for processed images:**
```
<entity>/<file>
```

## Dependencies

### Internal
| Package | Role |
|---------|------|
| `resizer/api/model` | `ImageRequest`, `ImageResponse`, `ServiceName` |
| `resizer/config` | `Config` (S3 bucket, endpoint, TMDB proxy URL) |
| `resizer/converter/image` | `Strategy`, `NewCustomImage`, `WithWidth` |
| `resizer/shared/log` | `LoggerWithTrace` |

### External
| Package | Purpose |
|---------|---------|
| `github.com/aws/aws-sdk-go/service/s3` | S3 get/delete operations |
| `github.com/aws/aws-sdk-go/service/s3/s3manager` | Multipart upload for S3 cache writes |
| `go.uber.org/zap` | Structured logging |

<!-- MANUAL: -->
