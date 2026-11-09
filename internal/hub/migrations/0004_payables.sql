-- Phase 3.5: two-step transfers and money owed to suppliers.

-- Transfers: goods leave the source when sent and arrive when received.
-- Transfers made before this change were one-step, so they count as received.
ALTER TABLE stock_transfers ADD COLUMN status TEXT NOT NULL DEFAULT 'received'
  CHECK (status IN ('in_transit','received','cancelled'));
ALTER TABLE stock_transfers ADD COLUMN received_by TEXT NOT NULL DEFAULT '';
ALTER TABLE stock_transfers ADD COLUMN received_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stock_transfers ADD COLUMN receive_note TEXT NOT NULL DEFAULT '';
UPDATE stock_transfers SET received_by = created_by, received_at = created_at;
ALTER TABLE stock_transfer_lines ADD COLUMN received_qty_milli INTEGER NOT NULL DEFAULT 0;
UPDATE stock_transfer_lines SET received_qty_milli = qty_milli;
CREATE INDEX stock_transfers_status ON stock_transfers(status, created_at);

-- Suppliers: payment terms and what we owe them.
ALTER TABLE suppliers ADD COLUMN terms_days INTEGER NOT NULL DEFAULT 0;
ALTER TABLE suppliers ADD COLUMN balance_minor INTEGER NOT NULL DEFAULT 0;

-- A supplier's invoice. paid_minor is filled by payments, oldest bill first.
CREATE TABLE supplier_bills (
  id           TEXT PRIMARY KEY,
  bill_no      TEXT NOT NULL UNIQUE,
  supplier_id  TEXT NOT NULL REFERENCES suppliers(id),
  supplier_ref TEXT NOT NULL DEFAULT '',
  grn_id       TEXT NOT NULL DEFAULT '',
  bill_date    INTEGER NOT NULL,
  due_date     INTEGER NOT NULL,
  amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
  paid_minor   INTEGER NOT NULL DEFAULT 0,
  status       TEXT NOT NULL CHECK (status IN ('open','paid','void')),
  note         TEXT NOT NULL DEFAULT '',
  created_by   TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL
);
CREATE INDEX supplier_bills_supplier ON supplier_bills(supplier_id, bill_date);
CREATE INDEX supplier_bills_open ON supplier_bills(status, due_date);
CREATE UNIQUE INDEX supplier_bills_grn ON supplier_bills(grn_id) WHERE grn_id <> '' AND status <> 'void';

CREATE TABLE supplier_payments (
  id           TEXT PRIMARY KEY,
  payment_no   TEXT NOT NULL UNIQUE,
  supplier_id  TEXT NOT NULL REFERENCES suppliers(id),
  method       TEXT NOT NULL,
  reference    TEXT NOT NULL DEFAULT '',
  amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
  paid_at      INTEGER NOT NULL,
  note         TEXT NOT NULL DEFAULT '',
  created_by   TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL
);
CREATE INDEX supplier_payments_supplier ON supplier_payments(supplier_id, paid_at);

-- How much of each payment went to which bill.
CREATE TABLE supplier_allocations (
  payment_id   TEXT NOT NULL REFERENCES supplier_payments(id),
  bill_id      TEXT NOT NULL REFERENCES supplier_bills(id),
  amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
  PRIMARY KEY (payment_id, bill_id)
) WITHOUT ROWID;
CREATE INDEX supplier_allocations_bill ON supplier_allocations(bill_id);

-- Every change to what we owe a supplier (positive = we owe more).
CREATE TABLE supplier_ledger (
  id           TEXT PRIMARY KEY,
  supplier_id  TEXT NOT NULL REFERENCES suppliers(id),
  at           INTEGER NOT NULL,
  kind         TEXT NOT NULL CHECK (kind IN ('opening','bill','payment','void','adjustment')),
  ref_id       TEXT NOT NULL DEFAULT '',
  ref_no       TEXT NOT NULL DEFAULT '',
  amount_minor INTEGER NOT NULL,
  note         TEXT NOT NULL DEFAULT '',
  user_name    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX supplier_ledger_supplier ON supplier_ledger(supplier_id, at);
