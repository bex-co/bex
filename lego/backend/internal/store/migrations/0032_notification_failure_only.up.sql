-- Make deploy email quiet by default: failures are actionable, while starts
-- and successes are routine. Move legacy rows that still exactly match the
-- former all-enabled default; preserve every customized combination.
ALTER TABLE notification_settings
    ALTER COLUMN deploy_started SET DEFAULT false,
    ALTER COLUMN deploy_succeeded SET DEFAULT false,
    ALTER COLUMN deploy_failed SET DEFAULT true;

UPDATE notification_settings
SET deploy_started = false,
    deploy_succeeded = false,
    updated_at = now()
WHERE deploy_started = true
  AND deploy_succeeded = true
  AND deploy_failed = true;
