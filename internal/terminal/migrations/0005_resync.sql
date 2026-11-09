-- Phase 5: after the hub's data is put back (restore or an older copy), a
-- full resync marks catalogue rows stale first; whatever the hub doesn't
-- send again is retired.
ALTER TABLE products ADD COLUMN stale INTEGER NOT NULL DEFAULT 0;
ALTER TABLE variants ADD COLUMN stale INTEGER NOT NULL DEFAULT 0;
ALTER TABLE barcodes ADD COLUMN stale INTEGER NOT NULL DEFAULT 0;
ALTER TABLE categories ADD COLUMN stale INTEGER NOT NULL DEFAULT 0;
ALTER TABLE payment_methods ADD COLUMN stale INTEGER NOT NULL DEFAULT 0;
