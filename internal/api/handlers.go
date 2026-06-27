package api

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

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
	db := s.store.GetDB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Store not initialized"})
	}

	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	rows, err := db.Query("SELECT key, name, is_dir, size, last_modified FROM objects WHERE parent = ?", prefix)
	if err != nil {
		log.Printf("Query error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	for rows.Next() {
		var key, name, lastMod string
		var isDir bool
		var size int64

		if err := rows.Scan(&key, &name, &isDir, &size, &lastMod); err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		if isDir {
			folders = append(folders, FolderEntry{Name: name, Path: key})
		} else {
			files = append(files, FileEntry{
				Name:         name,
				Path:         key,
				Size:         size,
				LastModified: lastMod,
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

	db := s.store.GetDB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Store not initialized"})
	}

	var info FileInfo
	err := db.QueryRow("SELECT size, content_type, last_modified, etag FROM objects WHERE key = ?", key).Scan(&info.Size, &info.ContentType, &info.LastModified, &info.ETag)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Not found"})
	}

	return c.JSON(info)
}

func (s *Server) handleSearch(c *fiber.Ctx) error {
	if s.config.S3Bucket == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "S3_BUCKET not configured"})
	}

	query := c.Query("q")
	db := s.store.GetDB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Store not initialized"})
	}

	lowerQ := strings.ToLower(query)
	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	// In SQLite, LIKE is case-insensitive by default for ASCII.
	rows, err := db.Query("SELECT key, name, is_dir, size, last_modified FROM objects WHERE name LIKE ? LIMIT 600", "%"+lowerQ+"%")
	if err != nil {
		log.Printf("Query error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	for rows.Next() {
		var key, name, lastMod string
		var isDir bool
		var size int64

		if err := rows.Scan(&key, &name, &isDir, &size, &lastMod); err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		if isDir {
			if len(folders) < 100 {
				folders = append(folders, FolderEntry{Name: name, Path: key})
			}
		} else {
			if len(files) < 500 {
				files = append(files, FileEntry{
					Name:         name,
					Path:         key,
					Size:         size,
					LastModified: lastMod,
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

	presignedUrl, err := s.s3client.GetPresignedUrl(c.Context(), key, time.Hour)
	if err != nil {
		log.Printf("GetPresignedUrl error: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.Redirect(presignedUrl, fiber.StatusFound)
}
