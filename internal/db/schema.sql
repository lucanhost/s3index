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
CREATE INDEX objects_name_idx ON objects(name);
CREATE INDEX objects_is_dir_idx ON objects(is_dir);
