-- 0195 rekeyed alert streams to the service's srv- id, the key the worker
-- writes. Restoring the CR-name key would split every digest again, so the
-- down migration leaves the rows as they are.
SELECT 1;
