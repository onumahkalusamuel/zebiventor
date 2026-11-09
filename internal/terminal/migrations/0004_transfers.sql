-- Phase 4: transfers coming to this till's location, so they can be
-- received here even while the hub is unreachable.
CREATE TABLE transfers (
  id               TEXT PRIMARY KEY,
  transfer_no      TEXT NOT NULL,
  from_location_id TEXT NOT NULL,
  to_location_id   TEXT NOT NULL,
  status           TEXT NOT NULL,
  data             TEXT NOT NULL,             -- protocol.StockTransfer as JSON
  received_locally INTEGER NOT NULL DEFAULT 0, -- received here, waiting to reach the hub
  local_receipt    TEXT NOT NULL DEFAULT '',
  updated_at       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX transfers_to ON transfers(to_location_id, status);
