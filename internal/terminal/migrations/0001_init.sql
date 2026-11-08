-- Terminal (till) local database: a read-mostly mirror of hub master data,
-- the till's own sales, and a durable outbox of ops waiting to reach the hub.

CREATE TABLE kv (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- ---------- mirror of hub master data ----------
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);

CREATE TABLE locations (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, code TEXT NOT NULL DEFAULT '', active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE users (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, username TEXT NOT NULL COLLATE NOCASE, role TEXT NOT NULL,
  password_hash TEXT NOT NULL DEFAULT '', pin_hash TEXT NOT NULL DEFAULT '', active INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX users_username ON users(username);

CREATE TABLE categories (
  id TEXT PRIMARY KEY, parent_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE products (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, category_id TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
  attributes TEXT NOT NULL DEFAULT '{}', unit TEXT NOT NULL DEFAULT 'pc', allow_fraction INTEGER NOT NULL DEFAULT 0,
  track_stock INTEGER NOT NULL DEFAULT 1, tax_rate_bp INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 1,
  updated_at INTEGER NOT NULL DEFAULT 0,
  -- denormalised lowercase haystack for fast search (name + attribute values)
  search TEXT NOT NULL DEFAULT ''
);
CREATE INDEX products_name ON products(name COLLATE NOCASE);

CREATE TABLE variants (
  id TEXT PRIMARY KEY, product_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', sku TEXT NOT NULL,
  price_minor INTEGER NOT NULL, cost_minor INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX variants_product ON variants(product_id);
CREATE INDEX variants_sku ON variants(sku COLLATE NOCASE);

CREATE TABLE barcodes (
  code TEXT PRIMARY KEY, variant_id TEXT NOT NULL, pack_qty_milli INTEGER NOT NULL DEFAULT 1000, deleted INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX barcodes_variant ON barcodes(variant_id);

CREATE TABLE payment_methods (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, requires_reference INTEGER NOT NULL DEFAULT 0,
  opens_drawer INTEGER NOT NULL DEFAULT 0, sort_order INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE stock_levels (
  variant_id TEXT NOT NULL, location_id TEXT NOT NULL, qty_milli INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (variant_id, location_id)
) WITHOUT ROWID;

-- ---------- this till's transactions ----------
CREATE TABLE sales (
  id TEXT PRIMARY KEY, receipt_no TEXT NOT NULL UNIQUE, location_id TEXT NOT NULL, user_id TEXT NOT NULL,
  user_name TEXT NOT NULL DEFAULT '', customer_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'completed',
  subtotal_minor INTEGER NOT NULL, discount_minor INTEGER NOT NULL DEFAULT 0, tax_minor INTEGER NOT NULL DEFAULT 0,
  total_minor INTEGER NOT NULL, paid_minor INTEGER NOT NULL, change_minor INTEGER NOT NULL DEFAULT 0,
  note TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL,
  synced_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX sales_created ON sales(created_at);
CREATE INDEX sales_unsynced ON sales(synced_at) WHERE synced_at = 0;

CREATE TABLE sale_lines (
  id TEXT PRIMARY KEY, sale_id TEXT NOT NULL REFERENCES sales(id), line_no INTEGER NOT NULL, variant_id TEXT NOT NULL,
  product_name TEXT NOT NULL, variant_name TEXT NOT NULL DEFAULT '', sku TEXT NOT NULL DEFAULT '',
  qty_milli INTEGER NOT NULL, unit_price_minor INTEGER NOT NULL, discount_minor INTEGER NOT NULL DEFAULT 0,
  tax_minor INTEGER NOT NULL DEFAULT 0, total_minor INTEGER NOT NULL, track_stock INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX sale_lines_sale ON sale_lines(sale_id);

CREATE TABLE sale_payments (
  id TEXT PRIMARY KEY, sale_id TEXT NOT NULL REFERENCES sales(id), method_id TEXT NOT NULL, method_name TEXT NOT NULL,
  kind TEXT NOT NULL, amount_minor INTEGER NOT NULL, reference TEXT NOT NULL DEFAULT ''
);
CREATE INDEX sale_payments_sale ON sale_payments(sale_id);

-- ---------- durable outbox ----------
CREATE TABLE outbox (
  id         TEXT PRIMARY KEY,           -- ULID, also the idempotency key at the hub
  kind       TEXT NOT NULL,
  ref_id     TEXT NOT NULL DEFAULT '',   -- e.g. sale id
  payload    TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','rejected')),
  attempts   INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  sent_at    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX outbox_pending ON outbox(status, id);
