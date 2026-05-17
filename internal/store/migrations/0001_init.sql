CREATE TABLE books (
  path          TEXT PRIMARY KEY,
  category      TEXT NOT NULL,
  title         TEXT NOT NULL,
  size_bytes    INTEGER NOT NULL,
  fingerprint   TEXT NOT NULL,
  added_at      DATETIME NOT NULL,
  removed_at    DATETIME
);

CREATE TABLE progress (
  book_path     TEXT PRIMARY KEY REFERENCES books(path) ON DELETE CASCADE,
  current_page  INTEGER NOT NULL DEFAULT 1,
  total_pages   INTEGER NOT NULL DEFAULT 0,
  last_read_at  DATETIME
);

CREATE TABLE bookmarks (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  book_path     TEXT NOT NULL REFERENCES books(path) ON DELETE CASCADE,
  page          INTEGER NOT NULL,
  label         TEXT,
  created_at    DATETIME NOT NULL
);
CREATE INDEX bookmarks_by_book ON bookmarks(book_path);

CREATE TABLE meta (
  key           TEXT PRIMARY KEY,
  value         TEXT NOT NULL
);
