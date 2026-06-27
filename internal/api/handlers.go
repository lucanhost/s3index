package api

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const requestTimeout = 5 * time.Second

func (s *Server) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"ts":     time.Now().UnixMilli(),
	})
}

func (s *Server) handleList(c *fiber.Ctx) error {
	if s.config.S3Bucket == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "S3_BUCKET not configured"})
	}

	prefix := c.Query("prefix")
	q := s.store.GetQueries()
	if q == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Store not initialized"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	objects, err := q.ListObjectsByParent(ctx, prefix)
	if err != nil {
		log.Printf("ListObjectsByParent error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	for _, obj := range objects {
		if obj.IsDir {
			folders = append(folders, FolderEntry{Name: obj.Name, Path: obj.Key})
		} else {
			files = append(files, FileEntry{
				Name:         obj.Name,
				Path:         obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified,
			})
		}
	}

	listing := &DirectoryListing{
		Folders: folders,
		Files:   files,
	}

	return c.JSON(listing)
}

func (s *Server) handleInfo(c *fiber.Ctx) error {
	if s.config.S3Bucket == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "S3_BUCKET not configured"})
	}

	key := c.Query("key")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing key parameter"})
	}

	q := s.store.GetQueries()
	if q == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Store not initialized"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	obj, err := q.GetObject(ctx, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Not found"})
	}

	info := FileInfo{
		Size:         obj.Size,
		ContentType:  obj.ContentType,
		LastModified: obj.LastModified,
		ETag:         obj.Etag,
	}

	return c.JSON(info)
}

func (s *Server) handleSearch(c *fiber.Ctx) error {
	if s.config.S3Bucket == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "S3_BUCKET not configured"})
	}

	query := c.Query("q")
	q := s.store.GetQueries()
	if q == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Store not initialized"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	lowerQ := strings.ToLower(query)
	// Escape double quotes and wrap in quotes for FTS MATCH
	escapedQ := "\"" + strings.ReplaceAll(lowerQ, "\"", "\"\"") + "\""
	
	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	objects, err := q.SearchObjects(ctx, escapedQ)
	if err != nil {
		log.Printf("SearchObjects error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	for _, obj := range objects {
		if obj.IsDir {
			if len(folders) < 100 {
				folders = append(folders, FolderEntry{Name: obj.Name, Path: obj.Key})
			}
		} else {
			if len(files) < 500 {
				files = append(files, FileEntry{
					Name:         obj.Name,
					Path:         obj.Key,
					Size:         obj.Size,
					LastModified: obj.LastModified,
				})
			}
		}
	}

	return c.JSON(SearchResults{Files: files, Folders: folders})
}

func (s *Server) handleObjectRedirect(c *fiber.Ctx) error {
	if s.config.S3Bucket == "" {
		return c.Status(fiber.StatusInternalServerError).SendString("S3_BUCKET not configured")
	}

	key := c.Params("*")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing key")
	}

	decodedKey, err := url.PathUnescape(key)
	if err != nil {
		decodedKey = key
	}

	presignedUrl, err := s.s3client.GetPresignedUrl(c.Context(), decodedKey, time.Hour)
	if err != nil {
		log.Printf("GetPresignedUrl error: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate presigned URL")
	}

	return c.Redirect(presignedUrl, fiber.StatusFound)
}
