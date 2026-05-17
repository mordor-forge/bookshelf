CREATE TABLE notes (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  book_path   TEXT NOT NULL REFERENCES books(path) ON DELETE CASCADE,
  page        INTEGER NOT NULL,
  body        TEXT NOT NULL,
  created_at  DATETIME NOT NULL,
  updated_at  DATETIME NOT NULL
);
CREATE INDEX notes_by_book ON notes(book_path);
