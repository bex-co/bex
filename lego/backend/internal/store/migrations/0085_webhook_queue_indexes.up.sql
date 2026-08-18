-- Admission counts only the affected workspaces' open logical notifications.
-- Keep that working set directly addressable through their endpoint ids rather
-- than scanning every tenant's bounded queue on each dispatcher page.
CREATE INDEX webhook_deliveries_endpoint_open_idx
    ON webhook_deliveries (endpoint_id)
    WHERE delivered_at IS NULL AND failed_at IS NULL;

-- Fair claims group pending reservations by the workspace owning each
-- endpoint, then preserve due/creation/id order within it. The endpoint prefix
-- lets that join stay on the partial pending working set.
CREATE INDEX webhook_delivery_attempts_endpoint_due_idx
    ON webhook_delivery_attempts (endpoint_id, available_at, created_at, id)
    WHERE status = 'pending';
