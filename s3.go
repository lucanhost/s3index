package main

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

func initS3Client() error {
	host, secure := parseEndpoint(globalConfig.S3Endpoint)

	var err error
	minioClient, err = minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(globalConfig.S3AccessKeyID, globalConfig.S3SecretAccessKey, ""),
		Secure: secure,
	})
	if err != nil {
		return err
	}
	return nil
}

func parseEndpoint(endpoint string) (string, bool) {
	if endpoint == "" {
		return "s3.amazonaws.com", true
	}
	secure := true
	host := endpoint
	if strings.HasPrefix(endpoint, "https://") {
		host = strings.TrimPrefix(endpoint, "https://")
		secure = true
	} else if strings.HasPrefix(endpoint, "http://") {
		host = strings.TrimPrefix(endpoint, "http://")
		secure = false
	}
	return host, secure
}

// ListDirectory lists files and folders for a given S3 prefix
func ListDirectory(ctx context.Context, prefix string) (DirectoryListing, error) {
	queryPrefix := prefix
	if queryPrefix != "" && !strings.HasSuffix(queryPrefix, "/") {
		queryPrefix = queryPrefix + "/"
	}

	folders := make([]FolderEntry, 0, 32)
	files := make([]FileEntry, 0, 64)

	objectCh := minioClient.ListObjects(ctx, globalConfig.S3Bucket, minio.ListObjectsOptions{
		Prefix:    queryPrefix,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return DirectoryListing{}, obj.Err
		}

		if strings.HasSuffix(obj.Key, "/") {
			// It is a folder
			name := strings.TrimPrefix(obj.Key, queryPrefix)
			name = strings.TrimSuffix(name, "/")
			if name == "" {
				continue
			}
			folders = append(folders, FolderEntry{Name: name, Path: obj.Key})
		} else {
			// It is a file
			name := strings.TrimPrefix(obj.Key, queryPrefix)
			if name == "" {
				continue
			}
			files = append(files, FileEntry{
				Name:         name,
				Path:         obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified.Format(time.RFC3339),
			})
		}
	}

	return DirectoryListing{Folders: folders, Files: files}, nil
}

// GetObjectInfo retrieves metadata (size, contentType, ETag, lastModified) for an object
func GetObjectInfo(ctx context.Context, key string) (FileInfo, error) {
	objInfo, err := minioClient.StatObject(ctx, globalConfig.S3Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		Size:         objInfo.Size,
		ContentType:  objInfo.ContentType,
		LastModified: objInfo.LastModified.Format(time.RFC3339),
		ETag:         objInfo.ETag,
	}, nil
}

// GetPresignedUrl generates a temporary presigned URL for downloading
func GetPresignedUrl(ctx context.Context, key string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := minioClient.PresignedGetObject(ctx, globalConfig.S3Bucket, key, expires, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// SearchBucket performs a recursive, bucket-wide search matching query
func SearchBucket(ctx context.Context, query string) (SearchResults, error) {
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lowerQ := strings.ToLower(query)
	files := make([]FileEntry, 0, 128)
	folderSet := make(map[string]string) // path -> name
	maxKeys := 500

	objectCh := minioClient.ListObjects(searchCtx, globalConfig.S3Bucket, minio.ListObjectsOptions{
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return SearchResults{}, obj.Err
		}

		key := obj.Key
		segments := strings.Split(key, "/")

		// Check folder segments in path
		for i := 0; i < len(segments)-1; i++ {
			seg := segments[i]
			if seg != "" && strings.Contains(strings.ToLower(seg), lowerQ) {
				folderPath := strings.Join(segments[:i+1], "/") + "/"
				if _, ok := folderSet[folderPath]; !ok {
					folderSet[folderPath] = seg
				}
			}
		}

		// Check filename
		if len(segments) > 0 {
			name := segments[len(segments)-1]
			if name != "" && strings.Contains(strings.ToLower(name), lowerQ) {
				files = append(files, FileEntry{
					Name:         name,
					Path:         key,
					Size:         obj.Size,
					LastModified: obj.LastModified.Format(time.RFC3339),
				})
			}
		}

		if len(files) >= maxKeys {
			break
		}
	}

	folders := make([]FolderEntry, 0, len(folderSet))
	for path, name := range folderSet {
		folders = append(folders, FolderEntry{Name: name, Path: path})
	}

	return SearchResults{Files: files, Folders: folders}, nil
}
