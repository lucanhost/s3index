-- name: InsertObject :exec
INSERT INTO objects (
    key, name, parent, is_dir, size, last_modified, content_type, etag
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: ListObjectsByParent :many
SELECT key, name, is_dir, size, last_modified
FROM objects
WHERE parent = ?;

-- name: GetObject :one
SELECT size, content_type, last_modified, etag
FROM objects
WHERE key = ?;

-- name: SearchObjects :many
SELECT key, name, is_dir, size, last_modified
FROM objects
WHERE name LIKE ? LIMIT 600;
