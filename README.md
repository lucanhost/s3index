# S3 Index

A modern, high-performance, standalone S3-compatible storage file browser. The project compiles into a single binary containing both the Go backend API and the embedded Svelte 5 + Tailwind CSS v4 frontend.

## Features

- **Isolated & Embeddable Design**: Serve the complete frontend directly from a single compiled binary without external dependencies.
- **High-Performance Caching**: Native, thread-safe, and expirable LRU cache (backed by `TwiN/gocache/v2`) protecting S3 endpoints by caching results for List, Info, and Search queries.
- **Memory-Safe Search**: Recursively queries buckets with safety cancel hooks to prevent goroutine leaks.
- **Fast, Minimal UI**: A beautifully optimized single-page application (SPA) with folders-first list view, immediate search (⌘K), and instant file previews.
- **Direct Downloads**: Generates S3 presigned URLs automatically to redirect clients for high-speed direct downloads.
- **Hot-Reloading Environment**: Seamless developer orchestration using Go Air and Vite.

## Tech Stack

- **Backend**: Go (`net/http`), MinIO S3 SDK.
- **Frontend**: Svelte 5, TypeScript, Tailwind CSS v4, Vite.
- **Orchestration**: Makefile, Go Air (`air`).

---

## Configuration

Duplicate `.env.example` to create a `.env` file at the root:

```ini
S3_BUCKET=your-bucket-name
S3_REGION=us-east-1
S3_ENDPOINT=https://s3.yourprovider.com
S3_ACCESS_KEY_ID=your-access-key-id
S3_SECRET_ACCESS_KEY=your-secret-access-key
S3_FORCE_PATH_STYLE=false

# Cache Configurations (Optional)
API_CACHE_TTL=1m
API_CACHE_SIZE=1000
API_CACHE_MAX_MEMORY=50MB
```

---

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 18+ & npm
- [Air](https://github.com/air-verse/air) (for Go live reloading)

### Installation

Install frontend dependencies:

```bash
npm install --prefix frontend
```

### Local Development (Hot Reloading)

Launch the backend Air server and Svelte dev server concurrently:

```bash
make dev
```

The frontend will run at `http://localhost:5173` (with HMR) proxying S3 calls to the Go backend on `http://localhost:8080`.

### Production Build

Build the production-ready frontend bundle and compile the standalone optimized Go executable:

```bash
make build
```

This compiles Vite assets to `frontend/dist/` and compiles the Go app with embedded assets using `-ldflags="-s -w"` to strip debug symbols (shrinking the binary size to ~7.9MB).

Run the standalone executable:

```bash
PORT=8080 ./s3index
```

### Docker (Containerization)

Build and start the application inside a lightweight Alpine container:

```bash
docker compose up --build -d
```

The server will be available at `http://localhost:8080` mapping variables automatically from your `.env` file.

To stop the container:

```bash
docker compose down
```

### Clean Up

Remove compiled binaries and temporary build assets:

```bash
make clean
```

