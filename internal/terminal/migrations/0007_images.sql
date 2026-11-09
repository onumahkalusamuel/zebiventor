-- Phase 7: product pictures (fetched from the hub by hash and cached).
ALTER TABLE products ADD COLUMN image_hash TEXT NOT NULL DEFAULT '';
