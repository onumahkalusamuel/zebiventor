-- Phase 5: stock work at a branch while the hub is unreachable.
-- Transfers sent from this till wait here until the hub numbers them.
ALTER TABLE transfers ADD COLUMN sent_locally INTEGER NOT NULL DEFAULT 0;

-- Purchase orders still expected at this till's location.
CREATE TABLE purchase_orders (
  id               TEXT PRIMARY KEY,
  po_no            TEXT NOT NULL,
  location_id      TEXT NOT NULL,
  status           TEXT NOT NULL,
  data             TEXT NOT NULL,             -- protocol.PurchaseOrder as JSON
  received_locally INTEGER NOT NULL DEFAULT 0, -- a delivery booked here is waiting to reach the hub
  updated_at       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX purchase_orders_location ON purchase_orders(location_id, status);
