-- Phase 5: daily sales summaries, so reports over months and years stay
-- fast (see summary.go). Filled from existing sales on first start.
CREATE TABLE sales_daily (
  day TEXT NOT NULL, hour INTEGER NOT NULL, user_id TEXT NOT NULL, terminal_id TEXT NOT NULL,
  location_id TEXT NOT NULL, customer_id TEXT NOT NULL, user_name TEXT NOT NULL DEFAULT '',
  docs INTEGER NOT NULL DEFAULT 0, change_minor INTEGER NOT NULL DEFAULT 0,
  qty INTEGER NOT NULL DEFAULT 0, gross INTEGER NOT NULL DEFAULT 0, disc INTEGER NOT NULL DEFAULT 0, tax INTEGER NOT NULL DEFAULT 0,
  ret INTEGER NOT NULL DEFAULT 0, net INTEGER NOT NULL DEFAULT 0, cost INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (day, hour, user_id, terminal_id, location_id, customer_id)
) WITHOUT ROWID;

CREATE TABLE sales_daily_items (
  variant_id TEXT NOT NULL, location_id TEXT NOT NULL, day TEXT NOT NULL,
  product_name TEXT NOT NULL DEFAULT '', sub TEXT NOT NULL DEFAULT '', docs INTEGER NOT NULL DEFAULT 0,
  qty INTEGER NOT NULL DEFAULT 0, gross INTEGER NOT NULL DEFAULT 0, disc INTEGER NOT NULL DEFAULT 0, tax INTEGER NOT NULL DEFAULT 0,
  ret INTEGER NOT NULL DEFAULT 0, net INTEGER NOT NULL DEFAULT 0, cost INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (variant_id, location_id, day)
) WITHOUT ROWID;
CREATE INDEX sales_daily_items_day ON sales_daily_items(day);

CREATE TABLE sales_daily_cats (
  category_id TEXT NOT NULL, location_id TEXT NOT NULL, day TEXT NOT NULL, docs INTEGER NOT NULL DEFAULT 0,
  qty INTEGER NOT NULL DEFAULT 0, gross INTEGER NOT NULL DEFAULT 0, disc INTEGER NOT NULL DEFAULT 0, tax INTEGER NOT NULL DEFAULT 0,
  ret INTEGER NOT NULL DEFAULT 0, net INTEGER NOT NULL DEFAULT 0, cost INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (day, category_id, location_id)
) WITHOUT ROWID;

CREATE TABLE payments_daily (
  day TEXT NOT NULL, method_name TEXT NOT NULL, kind TEXT NOT NULL, amount_minor INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (day, method_name, kind)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS returns_created ON returns(created_at);
CREATE INDEX IF NOT EXISTS account_payments_created ON account_payments(created_at);
