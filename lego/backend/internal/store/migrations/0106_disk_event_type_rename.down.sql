-- Reverse of 0106: restore the pre-w8/m34 bex spellings on live filters.
UPDATE webhook_endpoints
SET event_types = ARRAY(
    SELECT CASE
        WHEN t = 'disk_created' THEN 'disk_attached'
        WHEN t = 'disk_deleted' THEN 'disk_detached'
        ELSE t
    END
    FROM unnest(event_types) AS t
)
WHERE event_types && ARRAY['disk_created', 'disk_deleted']::text[];
