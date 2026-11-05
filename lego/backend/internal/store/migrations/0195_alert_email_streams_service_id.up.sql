-- w2/m252: feed-driven failure alerts (server_failed, cron_failed,
-- image_pull_failed) were keyed by the feed's App CR name
-- ("<tenant>-<name>") instead of the service's srv- id. The worker now keys
-- them by srv- id; this rekeys the streams already written so a digest window
-- that is open across the deploy continues instead of mailing the next
-- failure at once under a fresh key, and a pending digest links its service
-- page. A stream whose service is gone keeps its old key and ages out with
-- the retention sweep; one that would collide with a srv- keyed stream is
-- left as it is.
UPDATE alert_email_streams s
SET resource_id = a.id
FROM apps a
WHERE s.tenant_id = a.tenant_id
  AND s.resource_id = a.tenant_id || '-' || a.name
  AND s.event IN ('server_failed', 'cron_failed', 'image_pull_failed')
  AND NOT EXISTS (
      SELECT 1 FROM alert_email_streams o
      WHERE o.tenant_id = s.tenant_id AND o.subject = s.subject
        AND o.resource_id = a.id AND o.event = s.event);
