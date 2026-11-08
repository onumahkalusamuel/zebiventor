-- Phase 3: payments on account, reorder levels.

ALTER TABLE variants ADD COLUMN reorder_level_milli INTEGER NOT NULL DEFAULT 0;

CREATE TABLE account_payments (
  id TEXT PRIMARY KEY, payment_no TEXT NOT NULL UNIQUE, customer_id TEXT NOT NULL, customer_name TEXT NOT NULL DEFAULT '',
  shift_id TEXT NOT NULL DEFAULT '', method_id TEXT NOT NULL, method_name TEXT NOT NULL, kind TEXT NOT NULL,
  amount_minor INTEGER NOT NULL, reference TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL, user_name TEXT NOT NULL DEFAULT '', balance_before_minor INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, synced_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX account_payments_shift ON account_payments(shift_id);
CREATE INDEX account_payments_customer ON account_payments(customer_id);
