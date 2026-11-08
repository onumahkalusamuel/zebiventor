-- Phase 2: shifts, cash movements, returns, customers, tax, approvals.

ALTER TABLE sales ADD COLUMN shift_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN approved_by TEXT NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN tax_mode TEXT NOT NULL DEFAULT 'none';
ALTER TABLE sale_lines ADD COLUMN original_price_minor INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sale_lines ADD COLUMN tax_rate_bp INTEGER NOT NULL DEFAULT 0;
CREATE INDEX sales_shift ON sales(shift_id);
CREATE INDEX sales_customer ON sales(customer_id) WHERE customer_id <> '';

CREATE TABLE customers (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  phone         TEXT NOT NULL DEFAULT '',
  email         TEXT NOT NULL DEFAULT '',
  notes         TEXT NOT NULL DEFAULT '',
  active        INTEGER NOT NULL DEFAULT 1,
  balance_minor INTEGER NOT NULL DEFAULT 0, -- owed on account; hub-maintained
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);
CREATE INDEX customers_phone ON customers(phone) WHERE phone <> '';

CREATE TABLE returns (
  id              TEXT PRIMARY KEY,
  return_no       TEXT NOT NULL UNIQUE,
  sale_id         TEXT NOT NULL DEFAULT '',
  sale_receipt_no TEXT NOT NULL DEFAULT '',
  terminal_id     TEXT NOT NULL REFERENCES terminals(id),
  location_id     TEXT NOT NULL REFERENCES locations(id),
  user_id         TEXT NOT NULL,
  user_name       TEXT NOT NULL DEFAULT '',
  approved_by     TEXT NOT NULL DEFAULT '',
  shift_id        TEXT NOT NULL DEFAULT '',
  customer_id     TEXT NOT NULL DEFAULT '',
  reason          TEXT NOT NULL DEFAULT '',
  tax_minor       INTEGER NOT NULL DEFAULT 0,
  total_minor     INTEGER NOT NULL,
  created_at      INTEGER NOT NULL,
  received_at     INTEGER NOT NULL
);
CREATE INDEX returns_sale ON returns(sale_id);
CREATE INDEX returns_created ON returns(created_at);

CREATE TABLE return_lines (
  id           TEXT PRIMARY KEY,
  return_id    TEXT NOT NULL REFERENCES returns(id),
  sale_line_id TEXT NOT NULL DEFAULT '',
  variant_id   TEXT NOT NULL,
  product_name TEXT NOT NULL,
  variant_name TEXT NOT NULL DEFAULT '',
  sku          TEXT NOT NULL DEFAULT '',
  qty_milli    INTEGER NOT NULL,
  tax_minor    INTEGER NOT NULL DEFAULT 0,
  total_minor  INTEGER NOT NULL,
  restock      INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX return_lines_return ON return_lines(return_id);
CREATE INDEX return_lines_sale_line ON return_lines(sale_line_id);

CREATE TABLE return_refunds (
  id           TEXT PRIMARY KEY,
  return_id    TEXT NOT NULL REFERENCES returns(id),
  method_id    TEXT NOT NULL,
  method_name  TEXT NOT NULL,
  kind         TEXT NOT NULL,
  amount_minor INTEGER NOT NULL,
  reference    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX return_refunds_return ON return_refunds(return_id);

CREATE TABLE shifts (
  id                  TEXT PRIMARY KEY,
  terminal_id         TEXT NOT NULL REFERENCES terminals(id),
  location_id         TEXT NOT NULL,
  status              TEXT NOT NULL CHECK (status IN ('open','closed')),
  opened_by           TEXT NOT NULL,
  opened_by_name      TEXT NOT NULL DEFAULT '',
  opened_at           INTEGER NOT NULL,
  opening_float_minor INTEGER NOT NULL DEFAULT 0,
  closed_by           TEXT NOT NULL DEFAULT '',
  closed_by_name      TEXT NOT NULL DEFAULT '',
  closed_at           INTEGER NOT NULL DEFAULT 0,
  counted_cash_minor  INTEGER NOT NULL DEFAULT 0,
  expected_cash_minor INTEGER NOT NULL DEFAULT 0,
  variance_minor      INTEGER NOT NULL DEFAULT 0,
  note                TEXT NOT NULL DEFAULT '',
  summary             TEXT NOT NULL DEFAULT '{}',
  received_at         INTEGER NOT NULL
);
CREATE INDEX shifts_terminal ON shifts(terminal_id, opened_at);

CREATE TABLE cash_movements (
  id           TEXT PRIMARY KEY,
  shift_id     TEXT NOT NULL,
  terminal_id  TEXT NOT NULL,
  kind         TEXT NOT NULL CHECK (kind IN ('paid_in','paid_out')),
  amount_minor INTEGER NOT NULL,
  reason       TEXT NOT NULL DEFAULT '',
  user_id      TEXT NOT NULL,
  user_name    TEXT NOT NULL DEFAULT '',
  approved_by  TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  received_at  INTEGER NOT NULL
);
CREATE INDEX cash_movements_shift ON cash_movements(shift_id);

INSERT OR IGNORE INTO settings(key, value, updated_at) VALUES ('tax_mode', 'none', 0), ('tax_name', 'VAT', 0);
INSERT OR IGNORE INTO changelog(entity, entity_id) VALUES ('setting', 'tax_mode'), ('setting', 'tax_name');
