package s3index

import "embed"

//go:embed all:frontend/dist
var EmbedFS embed.FS
