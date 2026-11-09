-- Phase 7: product pictures (see images.go).
ALTER TABLE products ADD COLUMN image_hash TEXT NOT NULL DEFAULT '';
CREATE TABLE images (
  hash       TEXT PRIMARY KEY,
  data       BLOB NOT NULL,
  created_at INTEGER NOT NULL
);
