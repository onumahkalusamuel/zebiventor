-- Phase 3: back office (purchasing, receiving, transfers, counts, customer
-- accounts, reorder levels, cost of goods sold).

ALTER TABLE variants ADD COLUMN reorder_level_milli INTEGER NOT NULL DEFAULT 0;
-- Unit cost when the hub recorded the sale (weighted average at that time).
ALTER TABLE sale_lines ADD COLUMN cost_minor INTEGER NOT NULL DEFAULT 0;
UPDATE sale_lines SET cost_minor = COALESCE((SELECT cost_minor FROM variants v WHERE v.id = sale_lines.variant_id), 0);
CREATE INDEX returns_customer ON returns(customer_id) WHERE customer_id <> '';
CREATE INDEX shifts_opened ON shifts(opened_at);

CREATE TABLE suppliers (
  id           TEXT PRIMARY KEY,
  name         TEXT NOT NULL,
  contact_name TEXT NOT NULL DEFAULT '',
  phone        TEXT NOT NULL DEFAULT '',
  email        TEXT NOT NULL DEFAULT '',
  address      TEXT NOT NULL DEFAULT '',
  notes        TEXT NOT NULL DEFAULT '',
  active       INTEGER NOT NULL DEFAULT 1,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);

CREATE TABLE purchase_orders (
  id          TEXT PRIMARY KEY,
  po_no       TEXT NOT NULL UNIQUE,
  supplier_id TEXT NOT NULL REFERENCES suppliers(id),
  location_id TEXT NOT NULL REFERENCES locations(id),
  status      TEXT NOT NULL CHECK (status IN ('draft','ordered','partial','received','cancelled')),
  expected_at INTEGER NOT NULL DEFAULT 0,
  note        TEXT NOT NULL DEFAULT '',
  created_by  TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  ordered_at  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX purchase_orders_supplier ON purchase_orders(supplier_id, created_at);
CREATE INDEX purchase_orders_status ON purchase_orders(status, created_at);

CREATE TABLE purchase_order_lines (
  id                 TEXT PRIMARY KEY,
  po_id              TEXT NOT NULL REFERENCES purchase_orders(id),
  line_no            INTEGER NOT NULL,
  variant_id         TEXT NOT NULL REFERENCES variants(id),
  qty_ordered_milli  INTEGER NOT NULL CHECK (qty_ordered_milli > 0),
  qty_received_milli INTEGER NOT NULL DEFAULT 0,
  unit_cost_minor    INTEGER NOT NULL DEFAULT 0 CHECK (unit_cost_minor >= 0)
);
CREATE INDEX purchase_order_lines_po ON purchase_order_lines(po_id);

CREATE TABLE goods_receipts (
  id           TEXT PRIMARY KEY,
  grn_no       TEXT NOT NULL UNIQUE,
  po_id        TEXT NOT NULL DEFAULT '',
  supplier_id  TEXT NOT NULL DEFAULT '',
  location_id  TEXT NOT NULL REFERENCES locations(id),
  supplier_ref TEXT NOT NULL DEFAULT '',
  note         TEXT NOT NULL DEFAULT '',
  received_by  TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  total_minor  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX goods_receipts_po ON goods_receipts(po_id);
CREATE INDEX goods_receipts_supplier ON goods_receipts(supplier_id, created_at);

CREATE TABLE goods_receipt_lines (
  id              TEXT PRIMARY KEY,
  grn_id          TEXT NOT NULL REFERENCES goods_receipts(id),
  po_line_id      TEXT NOT NULL DEFAULT '',
  variant_id      TEXT NOT NULL REFERENCES variants(id),
  qty_milli       INTEGER NOT NULL CHECK (qty_milli > 0),
  unit_cost_minor INTEGER NOT NULL DEFAULT 0,
  total_minor     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX goods_receipt_lines_grn ON goods_receipt_lines(grn_id);

CREATE TABLE stock_transfers (
  id               TEXT PRIMARY KEY,
  transfer_no      TEXT NOT NULL UNIQUE,
  from_location_id TEXT NOT NULL REFERENCES locations(id),
  to_location_id   TEXT NOT NULL REFERENCES locations(id),
  note             TEXT NOT NULL DEFAULT '',
  created_by       TEXT NOT NULL DEFAULT '',
  created_at       INTEGER NOT NULL
);

CREATE TABLE stock_transfer_lines (
  id          TEXT PRIMARY KEY,
  transfer_id TEXT NOT NULL REFERENCES stock_transfers(id),
  variant_id  TEXT NOT NULL REFERENCES variants(id),
  qty_milli   INTEGER NOT NULL CHECK (qty_milli > 0)
);
CREATE INDEX stock_transfer_lines_transfer ON stock_transfer_lines(transfer_id);

CREATE TABLE stock_counts (
  id          TEXT PRIMARY KEY,
  count_no    TEXT NOT NULL UNIQUE,
  name        TEXT NOT NULL DEFAULT '',
  location_id TEXT NOT NULL REFERENCES locations(id),
  category_id TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL CHECK (status IN ('open','posted','cancelled')),
  created_by  TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  posted_by   TEXT NOT NULL DEFAULT '',
  posted_at   INTEGER NOT NULL DEFAULT 0,
  note        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE stock_count_lines (
  count_id       TEXT NOT NULL REFERENCES stock_counts(id),
  variant_id     TEXT NOT NULL REFERENCES variants(id),
  counted_milli  INTEGER NOT NULL DEFAULT 0,
  counted_at     INTEGER NOT NULL,
  counted_by     TEXT NOT NULL DEFAULT '',
  expected_milli INTEGER NOT NULL DEFAULT 0, -- frozen when posted
  cost_minor     INTEGER NOT NULL DEFAULT 0, -- frozen when posted
  uncounted      INTEGER NOT NULL DEFAULT 0, -- added at posting with zero_uncounted
  PRIMARY KEY (count_id, variant_id)
) WITHOUT ROWID;

CREATE TABLE account_payments (
  id           TEXT PRIMARY KEY,
  payment_no   TEXT NOT NULL UNIQUE,
  customer_id  TEXT NOT NULL,
  terminal_id  TEXT NOT NULL,
  shift_id     TEXT NOT NULL DEFAULT '',
  method_id    TEXT NOT NULL,
  method_name  TEXT NOT NULL,
  kind         TEXT NOT NULL,
  amount_minor INTEGER NOT NULL,
  reference    TEXT NOT NULL DEFAULT '',
  note         TEXT NOT NULL DEFAULT '',
  user_id      TEXT NOT NULL,
  user_name    TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  received_at  INTEGER NOT NULL
);
CREATE INDEX account_payments_customer ON account_payments(customer_id, created_at);
CREATE INDEX account_payments_created ON account_payments(created_at);

-- Every change to a customer's balance, so statements can be printed.
CREATE TABLE customer_ledger (
  id           TEXT PRIMARY KEY,
  customer_id  TEXT NOT NULL,
  at           INTEGER NOT NULL,
  kind         TEXT NOT NULL CHECK (kind IN ('opening','sale','return','payment','adjustment')),
  ref_id       TEXT NOT NULL DEFAULT '',
  ref_no       TEXT NOT NULL DEFAULT '',
  amount_minor INTEGER NOT NULL,
  note         TEXT NOT NULL DEFAULT '',
  user_name    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX customer_ledger_customer ON customer_ledger(customer_id, at);

-- Balances that existed before the ledger.
INSERT INTO customer_ledger(id, customer_id, at, kind, amount_minor, note)
  SELECT 'opening-' || id, id, updated_at, 'opening', balance_minor, 'Balance before statements were kept'
  FROM customers WHERE balance_minor <> 0;

-- Manager approval rules (1 = required for cashiers).
INSERT OR IGNORE INTO settings(key, value, updated_at) VALUES
  ('approval_discount', '1', 0), ('approval_price_override', '1', 0), ('approval_refund', '1', 0),
  ('approval_paid_out', '1', 0), ('approval_no_sale', '1', 0);
INSERT OR IGNORE INTO changelog(entity, entity_id) VALUES
  ('setting', 'approval_discount'), ('setting', 'approval_price_override'), ('setting', 'approval_refund'),
  ('setting', 'approval_paid_out'), ('setting', 'approval_no_sale');
