# S3 Index

A modern S3-compatible storage file browser. It features a standalone Golang backend with a responsive Svelte 5 + Tailwind CSS v4 frontend embedded directly into a single binary.

## Architecture

*   **Backend:** Go
*   **Frontend:** Svelte 5 + Vite + Tailwind CSS v4
*   **Deployment:** The frontend compiles into `frontend/dist/` which is then natively embedded via `//go:embed` into a single lightweight Go executable.

## API Routes

| Route | Method | Description |
|---|---|---|
| `/api/health` | `GET` | Health check |
| `/api/list?prefix=<prefix>` | `GET` | List folder contents |
| `/api/info?key=<key>` | `GET` | Get file metadata |
| `/api/search?q=<query>` | `GET` | Search all files and folders in bucket |
| `/api/object/{key...}` | `GET` | Generates S3 presigned URL for object |
| `/*` | `GET` | Serves embedded frontend static files |

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
