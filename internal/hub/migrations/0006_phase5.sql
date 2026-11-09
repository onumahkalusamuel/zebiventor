-- Phase 5: whether each till's last connection was encrypted (TLS).
ALTER TABLE terminals ADD COLUMN encrypted INTEGER NOT NULL DEFAULT 0;

-- Tills keep the purchase orders still expected, to receive deliveries
-- offline: queue the open ones.
INSERT INTO changelog(entity, entity_id)
  SELECT 'purchase_order', id FROM purchase_orders WHERE status IN ('ordered','partial');
