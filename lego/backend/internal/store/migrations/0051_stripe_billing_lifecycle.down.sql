ALTER TABLE audit_events
    DROP COLUMN IF EXISTS billing_status_to,
    DROP COLUMN IF EXISTS billing_status_from,
    DROP COLUMN IF EXISTS billing_override_reason;
ALTER TABLE tenants DROP COLUMN IF EXISTS billing_comped;
DROP TABLE IF EXISTS billing_notifications;
DROP TABLE IF EXISTS billing_enforcements;
DROP TABLE IF EXISTS stripe_billing_events;
DROP TABLE IF EXISTS billing_lifecycles;
DROP TABLE IF EXISTS billing_provider_mappings;
