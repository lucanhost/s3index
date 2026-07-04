# Architecture Documentation

## Overview

S3 Index is a single-binary file browser for S3-compatible storage. It indexes bucket contents into an in-memory SQLite database for fast querying, while serving a modern web UI.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            s3index Binary                                │
│  ┌─────────────────┐  ┌─────────────────────────────────────────────┐   │
│  │   Frontend      │  │                 Backend                      │   │
│  │   (embedded)    │  │         (Go stdlib + SQLite)               │   │
│  │                 │  │                                            │   │
│  │  Svelte 5 SPA   │  │  ┌─────────────┐  ┌──────────────────────┐  │   │
│  │  - Vite build   │  │  │ HTTP Router  │  │ Background Sync      │  │   │
│  │  - Tailwind     │  │  │ (net/http)   │  │ (goroutine)        │  │   │
│  │  - Typescript   │  │  └──────┬──────┘  └───────┬──────────────┘  │   │
│  └─────────────────┘  │         │                  │                │   │
│                      │  │         │  Atomic Swap     │                │   │
│                      │  │         ▼                  ▼                │   │
│                      │  │  ┌─────────────┐  ┌──────────────────────┐  │   │
│                      │  │  │  Handlers   │  │ Store (SQLite)       │  │   │
│                      │  │  │ ─────────── │  │ ───────────────────── │  │   │
│                      │  │  │ - /api/health    │  │ - objects table       │  │   │
│                      │  │  │ - /api/list      │  │ - objects_fts (FTS5)  │  │   │
│                      │  │  │ - /api/info      │  │ - atomic pointer swap  │  │   │
│                      │  │  │ - /api/search    │  │ - sync goroutine      │  │   │
│                      │  │  │ - /api/sync      │  └──────────────────────┘  │   │
│                      │  │  │ - /api/object    │                            │   │
│                      │  │  └─────────────┘                              │   │
│                      │  └─────────────────────────────────────────────┘   │
│                      │                    │                               │
│                      │                    │ HTTP API                      │
│                      │                    ▼                               │
│                      │  ┌─────────────────────────────────────────────┐   │
│                      │  │         S3 / MinIO Client                 │   │
│                      │  │  - ListObjects (streaming)                │   │
│                      │  │  - PresignedGetObject URLs                │   │
│                      │  └─────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## Component Details

### 1. Main Entry Point (`cmd/s3index/main.go`)

Orchestrates startup sequence:
1. Load configuration from environment
2. Initialize S3 client
3. Create store with initial sync
4. Create HTTP server with routes
5. Start HTTP listener in goroutine
6. Wait for shutdown signal
7. Graceful shutdown (stop sync, close DB, shutdown HTTP)

### 2. Configuration (`internal/config/config.go`)

Uses `caarlos0/env` for environment variable parsing:
- Required: `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`
- Optional: `S3_REGION` (default from endpoint), `S3_ENDPOINT`, `S3_FORCE_PATH_STYLE`
- Derived: `PORT` (default 8080), `SYNC_INTERVAL` (default 5m)

### 3. S3 Client (`internal/s3client/s3.go`)

Thin wrapper around MinIO client:
- `ListObjects(ctx, recursive)` - streams objects via channel
- `GetPresignedUrl(ctx, key, expires)` - generates time-limited download URLs
- Supports AWS S3, Cloudflare R2, MinIO via endpoint configuration

### 4. Store (`internal/store/store.go`)

Core data management:
- **Atomic swap pattern**: New DB built, then pointer swapped, old DB closed
- **Background sync**: Goroutine with ticker + trigger channel
- **In-memory SQLite**: Fast queries, FTS5 for search
- **Graceful shutdown**: WaitGroup coordinates goroutine termination

#### Database Schema (`internal/db/schema.sql`)

```sql
CREATE TABLE objects (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    parent TEXT NOT NULL,     -- parent directory path
    is_dir BOOLEAN NOT NULL,  -- true for folders
    size INTEGER NOT NULL,
    last_modified TEXT NOT NULL,
    content_type TEXT NOT NULL,
    etag TEXT NOT NULL
);

CREATE INDEX objects_parent_idx ON objects(parent);

-- FTS5 for trigram fuzzy search
CREATE VIRTUAL TABLE objects_fts USING fts5(
    name, 
    key UNINDEXED, 
    tokenize='trigram'
);

CREATE TRIGGER objects_ai AFTER INSERT ON objects BEGIN
    INSERT INTO objects_fts(name, key) VALUES (new.name, new.key);
END;
```

### 5. API Layer (`internal/api/`)

#### Routes

| Route | Method | Description |
|-------|--------|-------------|
| `/api/health` | GET | Service health check |
| `/api/list?prefix=` | GET | List directory contents |
| `/api/info?key=` | GET | File metadata |
| `/api/search?q=` | GET | Trigram FTS5 search |
| `/api/sync` | POST | Trigger immediate resync |
| `/api/object/{key}` | GET | Presigned S3 download URL |
| `/` | GET | Serve SPA or API-only JSON |

#### Response Types (`internal/api/types.go`)

```go
type FileEntry struct {
    Name         string `json:"name"`
    Path         string `json:"path"`      // full key
    Size         int64  `json:"size"`
    LastModified string `json:"lastModified"`
}

type FolderEntry struct {
    Name string `json:"name"`
    Path string `json:"path"`
}

type DirectoryListing struct {
    Folders []FolderEntry `json:"folders"`
    Files   []FileEntry   `json:"files"`
}

type FileInfo struct {
    Size         int64  `json:"size"`
    ContentType  string `json:"contentType"`
    LastModified string `json:"lastModified"`
    ETag         string `json:"eTag"`
}
```

### 6. Frontend (`frontend/`)

Svelte 5 SPA with:
- **State management**: `$state`, `$derived` runes
- **Routing**: URL-based (no router library)
- **Features**: Breadcrumb, search, preview modal, README rendering
- **Build**: Vite → `frontend/dist/` → embedded via `//go:embed`

## Data Flow

### Initial Load
```
1. config.LoadConfig() → validates env vars
2. s3client.NewClient() → connects to S3
3. store.NewStore() → fetches all objects → builds DB → swaps state
4. api.NewServer() → registers routes → returns *http.ServeMux
5. http.Server.Serve() → listens for connections
```

### Request Cycle
```
GET /api/list?prefix=photos/
    ↓
handleList() → get queries from atomic pointer
    ↓
SELECT * FROM objects WHERE parent = 'photos/'
    ↓
rows → toEntries() → DirectoryListing{}
    ↓
writeJSON() → client
```

### Sync Cycle
```
Ticker OR POST /api/sync trigger
    ↓
createEmptyDB() → ListObjects() → insert all
    ↓
Atomic swap of DB state pointer
    ↓
Old DB.Close() → free memory
```

## Key Design Decisions

### Why SQLite (in-memory)?
- **Performance**: Millisecond queries vs network latency
- **FTS5**: Built-in trigram tokenizer for fuzzy search
- **No deps**: Single binary, no external database required
- **Atomic swap**: Easy to rebuild and swap without locks

### Why Atomic Swap vs WAL?
- Reads never block on writes
- Failed sync doesn't corrupt DB
- Old DB cleaned up after swap completes
- Trade-off: Full rebuild each sync (acceptable for read-heavy workloads)

### Why No Router Library?
- Standard library `http.ServeMux` supports method routing (Go 1.22+)
- Fewer dependencies for single-binary deployment
- Simpler middleware patterns

## File Structure

```
s3index/
├── cmd/s3index/main.go        # entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go        # HTTP handlers
│   │   ├── server.go          # router setup
│   │   └── types.go           # response types
│   ├── config/config.go       # env configuration
│   ├── db/
│   │   ├── schema.sql         # DB schema
│   │   ├── query.sql          # sqlc queries
│   │   └── *.go               # generated sqlc code
│   ├── s3client/s3.go         # S3 API client
│   └── store/store.go         # sync orchestration
├── frontend/
│   ├── src/                   # Svelte source
│   ├── dist/                  # built assets (embedded)
│   └── *.json                 # npm config
├── embed.go                   # //go:embed frontend/dist
├── go.mod                     # Go 1.25
└── Makefile                   # build targets
```