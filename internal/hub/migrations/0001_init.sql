-- Hub (system of record) schema.
-- Conventions: timestamps are unix ms; money is integer minor units;
-- quantities are integer milli-units; booleans are 0/1.

CREATE TABLE settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

-- One row per entity holding its latest revision. Terminals pull
-- "everything with rev > X". AUTOINCREMENT guarantees revs never repeat
-- even though superseded rows are deleted.
CREATE TABLE changelog (
  rev       INTEGER PRIMARY KEY AUTOINCREMENT,
  entity    TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  UNIQUE (entity, entity_id)
);

CREATE TABLE locations (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  code       TEXT NOT NULL DEFAULT '',
  active     INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE users (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  username      TEXT NOT NULL COLLATE NOCASE UNIQUE,
  role          TEXT NOT NULL CHECK (role IN ('admin','manager','inventory','cashier')),
  password_hash TEXT NOT NULL DEFAULT '',
  pin_hash      TEXT NOT NULL DEFAULT '',
  active        INTEGER NOT NULL DEFAULT 1,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE terminals (
  id             TEXT PRIMARY KEY,
  name           TEXT NOT NULL,
  location_id    TEXT NOT NULL REFERENCES locations(id),
  receipt_prefix TEXT NOT NULL UNIQUE,
  token_hash     TEXT NOT NULL UNIQUE,
  active         INTEGER NOT NULL DEFAULT 1,
  paired_at      INTEGER NOT NULL,
  last_seen_at   INTEGER NOT NULL DEFAULT 0,
  last_ip        TEXT NOT NULL DEFAULT '',
  app_version    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE pairing_codes (
  code_hash     TEXT PRIMARY KEY,
  terminal_name TEXT NOT NULL DEFAULT '',
  location_id   TEXT NOT NULL REFERENCES locations(id),
  created_by    TEXT NOT NULL DEFAULT '',
  expires_at    INTEGER NOT NULL,
  used_at       INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE sessions (
  token_hash  TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL REFERENCES users(id),
  terminal_id TEXT NOT NULL REFERENCES terminals(id),
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);

CREATE TABLE categories (
  id         TEXT PRIMARY KEY,
  parent_id  TEXT NOT NULL DEFAULT '',
  name       TEXT NOT NULL,
  active     INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE products (
  id             TEXT PRIMARY KEY,
  name           TEXT NOT NULL,
  category_id    TEXT NOT NULL DEFAULT '',
  description    TEXT NOT NULL DEFAULT '',
  attributes     TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(attributes)),
  unit           TEXT NOT NULL DEFAULT 'pc',
  allow_fraction INTEGER NOT NULL DEFAULT 0,
  track_stock    INTEGER NOT NULL DEFAULT 1,
  tax_rate_bp    INTEGER NOT NULL DEFAULT 0,
  active         INTEGER NOT NULL DEFAULT 1,
  created_at     INTEGER NOT NULL,
  updated_at     INTEGER NOT NULL
);
CREATE INDEX products_name ON products(name COLLATE NOCASE);

CREATE TABLE variants (
  id          TEXT PRIMARY KEY,
  product_id  TEXT NOT NULL REFERENCES products(id),
  name        TEXT NOT NULL DEFAULT '',
  sku         TEXT NOT NULL,
  price_minor INTEGER NOT NULL CHECK (price_minor >= 0),
  cost_minor  INTEGER NOT NULL DEFAULT 0,
  active      INTEGER NOT NULL DEFAULT 1,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
CREATE UNIQUE INDEX variants_sku ON variants(sku COLLATE NOCASE);
CREATE INDEX variants_product ON variants(product_id);

CREATE TABLE barcodes (
  code           TEXT PRIMARY KEY,
  variant_id     TEXT NOT NULL REFERENCES variants(id),
  pack_qty_milli INTEGER NOT NULL DEFAULT 1000 CHECK (pack_qty_milli > 0),
  deleted        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX barcodes_variant ON barcodes(variant_id);

CREATE TABLE payment_methods (
  id                 TEXT PRIMARY KEY,
  name               TEXT NOT NULL,
  kind               TEXT NOT NULL CHECK (kind IN ('cash','card','transfer','mobile_money','credit','voucher','other')),
  requires_reference INTEGER NOT NULL DEFAULT 0,
  opens_drawer       INTEGER NOT NULL DEFAULT 0,
  sort_order         INTEGER NOT NULL DEFAULT 0,
  active             INTEGER NOT NULL DEFAULT 1,
  updated_at         INTEGER NOT NULL
);

-- Stock is an append-only ledger; stock_levels is its running total,
-- maintained in the same transaction as each movement.
CREATE TABLE stock_movements (
  id          TEXT PRIMARY KEY,
  variant_id  TEXT NOT NULL REFERENCES variants(id),
  location_id TEXT NOT NULL REFERENCES locations(id),
  qty_milli   INTEGER NOT NULL,
  reason      TEXT NOT NULL CHECK (reason IN ('opening','sale','return','receipt','adjustment','count','transfer_in','transfer_out')),
  ref_type    TEXT NOT NULL DEFAULT '',
  ref_id      TEXT NOT NULL DEFAULT '',
  user_id     TEXT NOT NULL DEFAULT '',
  terminal_id TEXT NOT NULL DEFAULT '',
  note        TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  received_at INTEGER NOT NULL
);
CREATE INDEX stock_movements_variant ON stock_movements(variant_id, location_id, created_at);
CREATE INDEX stock_movements_ref ON stock_movements(ref_type, ref_id);

CREATE TABLE stock_levels (
  variant_id  TEXT NOT NULL REFERENCES variants(id),
  location_id TEXT NOT NULL REFERENCES locations(id),
  qty_milli   INTEGER NOT NULL DEFAULT 0,
  updated_at  INTEGER NOT NULL,
  PRIMARY KEY (variant_id, location_id)
) WITHOUT ROWID;

CREATE TABLE sales (
  id             TEXT PRIMARY KEY,
  receipt_no     TEXT NOT NULL UNIQUE,
  terminal_id    TEXT NOT NULL REFERENCES terminals(id),
  location_id    TEXT NOT NULL REFERENCES locations(id),
  user_id        TEXT NOT NULL,
  user_name      TEXT NOT NULL DEFAULT '',
  customer_id    TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'completed' CHECK (status IN ('completed','voided')),
  subtotal_minor INTEGER NOT NULL,
  discount_minor INTEGER NOT NULL DEFAULT 0,
  tax_minor      INTEGER NOT NULL DEFAULT 0,
  total_minor    INTEGER NOT NULL,
  paid_minor     INTEGER NOT NULL,
  change_minor   INTEGER NOT NULL DEFAULT 0,
  note           TEXT NOT NULL DEFAULT '',
  created_at     INTEGER NOT NULL,
  received_at    INTEGER NOT NULL
);
CREATE INDEX sales_created ON sales(created_at);
CREATE INDEX sales_terminal ON sales(terminal_id, created_at);

CREATE TABLE sale_lines (
  id               TEXT PRIMARY KEY,
  sale_id          TEXT NOT NULL REFERENCES sales(id),
  line_no          INTEGER NOT NULL,
  variant_id       TEXT NOT NULL,
  product_name     TEXT NOT NULL,
  variant_name     TEXT NOT NULL DEFAULT '',
  sku              TEXT NOT NULL DEFAULT '',
  qty_milli        INTEGER NOT NULL,
  unit_price_minor INTEGER NOT NULL,
  discount_minor   INTEGER NOT NULL DEFAULT 0,
  tax_minor        INTEGER NOT NULL DEFAULT 0,
  total_minor      INTEGER NOT NULL
);
CREATE INDEX sale_lines_sale ON sale_lines(sale_id);
CREATE INDEX sale_lines_variant ON sale_lines(variant_id);

CREATE TABLE sale_payments (
  id           TEXT PRIMARY KEY,
  sale_id      TEXT NOT NULL REFERENCES sales(id),
  method_id    TEXT NOT NULL,
  method_name  TEXT NOT NULL,
  kind         TEXT NOT NULL,
  amount_minor INTEGER NOT NULL,
  reference    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX sale_payments_sale ON sale_payments(sale_id);
CREATE INDEX sale_payments_method ON sale_payments(method_id);

-- Idempotency record for every op pushed by a terminal.
CREATE TABLE inbox (
  op_id       TEXT PRIMARY KEY,
  terminal_id TEXT NOT NULL,
  kind        TEXT NOT NULL,
  status      TEXT NOT NULL,
  error       TEXT NOT NULL DEFAULT '',
  received_at INTEGER NOT NULL
);

CREATE TABLE audit_log (
  id          TEXT PRIMARY KEY,
  at          INTEGER NOT NULL,
  user_id     TEXT NOT NULL DEFAULT '',
  terminal_id TEXT NOT NULL DEFAULT '',
  action      TEXT NOT NULL,
  entity      TEXT NOT NULL DEFAULT '',
  entity_id   TEXT NOT NULL DEFAULT '',
  details     TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX audit_log_at ON audit_log(at);
