.PHONY: dev build-frontend build clean

dev:
	~/go/bin/air & npm run dev --prefix frontend

build-frontend:
	npm run build --prefix frontend

build: build-frontend
	go build -ldflags="-s -w" -o s3index ./cmd/s3index

clean:
	rm -rf s3index tmp frontend/dist
