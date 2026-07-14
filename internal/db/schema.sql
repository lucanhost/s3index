CREATE TABLE objects (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    parent TEXT NOT NULL,
    is_dir BOOLEAN NOT NULL,
    size INTEGER NOT NULL,
    last_modified TEXT NOT NULL,
    content_type TEXT NOT NULL,
    etag TEXT NOT NULL
);

CREATE INDEX objects_parent_idx ON objects(parent);
CREATE INDEX objects_is_dir_idx ON objects(is_dir);

-- FTS5 virtual table for high-performance substring searching
CREATE VIRTUAL TABLE objects_fts USING fts5(name, key UNINDEXED, tokenize='trigram');

-- Triggers to keep FTS index in sync
CREATE TRIGGER objects_ai AFTER INSERT ON objects BEGIN
    INSERT INTO objects_fts(name, key) VALUES (new.name, new.key);
END;

CREATE TRIGGER objects_ad AFTER DELETE ON objects BEGIN
    DELETE FROM objects_fts WHERE key = old.key;
END;
