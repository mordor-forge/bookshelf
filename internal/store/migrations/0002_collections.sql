CREATE TABLE collections (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  parent_id   INTEGER REFERENCES collections(id) ON DELETE CASCADE,
  source      TEXT NOT NULL CHECK (source IN ('scan', 'manual')),
  folder_path TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX collections_folder_path ON collections(folder_path) WHERE folder_path IS NOT NULL;
CREATE INDEX collections_parent ON collections(parent_id);

CREATE TABLE book_collections (
  book_path     TEXT NOT NULL REFERENCES books(path) ON DELETE CASCADE,
  collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  PRIMARY KEY (book_path, collection_id)
);
CREATE INDEX book_collections_by_collection ON book_collections(collection_id);
