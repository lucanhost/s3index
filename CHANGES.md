# S3 Index — Code Review & Fixes Summary

## Date: 2026-06-27

## Project Overview

S3 Index is a Go backend + Svelte 5 frontend for browsing S3-compatible storage (AWS S3, Cloudflare R2, MinIO). Single binary deployment with embedded frontend. Uses in-memory SQLite to index S3 objects for fast listing/searching with background sync.

## Issues Found & Fixed

### HIGH — Security / Bugs

| # | File | Issue | Fix |
|---|------|-------|-----|
| 1 | `internal/api/handlers.go:150` | **Error message leak** — `err.Error()` sent directly to client, exposing internal S3 errors | Replaced with generic `"Failed to generate presigned URL"` |
| 2 | `internal/api/handlers.go:106` | **LIKE wildcard injection** — `%`, `_`, `\` in user input not escaped | Added `escapeLike()` helper that escapes all LIKE special chars |
| 3 | `internal/config/config.go:29-31` | **Silent config failure** — `env.Parse` errors only logged as warning, missing required fields not caught | Changed to `log.Fatalf` on parse error; added validation for `S3Bucket`, `S3AccessKeyID`, `S3SecretAccessKey` |
| 4 | `internal/store/store.go:28-43` | **Duplicate schema** — SQL schema hardcoded in `store.go` AND in `schema.sql`, will drift apart | Removed hardcoded copy, embedded `schema.sql` via `//go:embed` |

### MEDIUM — Reliability / Correctness

| # | File | Issue | Fix |
|---|------|-------|-----|
| 5 | `internal/s3client/s3.go:22` | **`S3_REGION` loaded but never used** — minio client created without region | Added `Region: cfg.S3Region` to `minio.Options` |
| 6 | `internal/api/handlers.go` | **No request-scoped timeouts** — DB queries have no deadline | Added `context.WithTimeout` (5s) to all DB query handlers |
| 7 | `internal/store/store.go:63` | **No sync worker shutdown coordination** — goroutine cancelled but no WaitGroup | Added `sync.WaitGroup`, new `Shutdown()` method called before server stops |
| 8 | `internal/api/server.go:87` | **`f.Close()` error ignored** | Changed to `defer func()` that logs the close error |

## Files Changed

| File | Action | Summary |
|------|--------|---------|
| `internal/config/config.go` | Modified | Fatal on missing required env vars |
| `internal/s3client/s3.go` | Modified | Pass `Region` to minio client |
| `internal/api/handlers.go` | Modified | Fix error leak, escape LIKE, add request timeouts, add `escapeLike()` |
| `internal/api/server.go` | Modified | Handle `f.Close()` error |
| `internal/store/store.go` | Modified | Embed schema, add `sync.WaitGroup`, add `Shutdown()` |
| `internal/store/schema.sql` | **New** | Embedded copy of schema for runtime DB init |
| `cmd/s3index/main.go` | Modified | Call `colStore.Shutdown()` before Fiber shutdown |

## Test Results

All endpoints verified working:

| Endpoint | Status | Notes |
|----------|--------|-------|
| `GET /api/health` | 200 | Returns `{"status":"ok","ts":...}` |
| `GET /api/list?prefix=` | 200 | Lists root folder correctly |
| `GET /api/list?prefix=linux/` | 200 | Subfolder listing works |
| `GET /api/info?key=README.md` | 200 | Returns metadata with content type |
| `GET /api/search?q=img` | 200 | LIKE escape works, finds matching files |
| `GET /api/object/README.md` | 302 | Presigned URL generated correctly |
| `GET /` | 200 | Embedded Svelte frontend served |
| Missing `S3_BUCKET` | fatal | Config validation catches missing vars |

- `go vet ./...` — passed
- `go build ./cmd/s3index` — passed
- Background sync — completed successfully
- Graceful shutdown — clean exit
