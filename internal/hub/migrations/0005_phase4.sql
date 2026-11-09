-- Tills now keep transfers coming to their location so they can receive
-- them offline: queue the ones already on the road.
INSERT INTO changelog(entity, entity_id)
  SELECT 'transfer', id FROM stock_transfers WHERE status = 'in_transit';
