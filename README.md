# S3 Index

A modern, high-performance S3-compatible storage file browser. It features a standalone Golang backend with an ultra-fast `kelindar/column` metadata store, serving a beautifully responsive Svelte 5 + Tailwind CSS v4 frontend embedded directly into a single binary.

## Architecture

*   **Backend:** Go (1.25) modular API utilizing standard package layout (`cmd/s3index`, `internal/api`, `internal/store`, `internal/s3client`, `internal/config`).
*   **Store:** `kelindar/column` providing zero-allocation, SIMD-accelerated, thread-safe memory columnar indexing of your S3 bucket metadata.
*   **Frontend:** Svelte 5 + Vite + Tailwind CSS v4 for a seamless Single Page Application (SPA) experience.
*   **Deployment:** The frontend compiles into `frontend/dist/` which is then natively embedded via `//go:embed` into a single lightweight Go executable.
*   **Caching Strategy:** Static assets (JS/CSS) are aggressively cached by the browser (`Cache-Control: public, max-age=31536000, immutable`), while the HTML entrypoint is never cached to ensure instant updates. API routes fetch in-memory from the fast column store.

## Go API Routes

| Route | Method | Description |
|---|---|---|
| `/api/health` | `GET` | Health check |
| `/api/list?prefix=<prefix>` | `GET` | List folder contents (Queries columnar memory store) |
| `/api/info?key=<key>` | `GET` | Get file metadata (Queries columnar memory store) |
| `/api/search?q=<query>` | `GET` | Search all files and folders in bucket |
| `/api/object/{key...}` | `GET` | Generates S3 presigned URL and redirects (302) client |
| `/*` | `GET` | Serves embedded frontend static files (SPA fallback) |

## Development & Operations

1. **Build frontend assets**:
   ```bash
   make build-frontend
   ```

2. **Build standalone binary**:
   ```bash
   make build
   ```
   This compiles the Svelte assets to `frontend/dist/` and then builds the `s3index` Go binary with embedded assets.

3. **Development Mode (Hot-Reloading)**:
   ```bash
   make dev
   ```
   Launches the Go Air hot-rebuild backend (on `http://localhost:8080`) and Svelte Vite dev server (on `http://localhost:5173`) concurrently.

4. **Clean up build artifacts**:
   ```bash
   make clean
   ```

5. **Docker Container**:
   Build and run the application in a background container:
   ```bash
   docker compose up --build -d
   ```
