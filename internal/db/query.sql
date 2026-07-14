-- name: InsertObject :exec
INSERT INTO objects (
    key, name, parent, is_dir, size, last_modified, content_type, etag
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: UpsertObject :exec
INSERT OR REPLACE INTO objects (
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
SELECT o.key, o.name, o.is_dir, o.size, o.last_modified
FROM objects o
JOIN objects_fts f ON o.key = f.key
WHERE f.name MATCH ? LIMIT 600;
