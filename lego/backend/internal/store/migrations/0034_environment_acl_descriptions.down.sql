-- w4/m24 migration 0034 rollback: jsonb entries back to text[] CIDRs.
-- Descriptions are dropped (the pre-m24 column cannot hold them). ALTER TYPE's
-- USING clause cannot run a subquery, so the conversion goes through a
-- temporary column.
ALTER TABLE environments ADD COLUMN ip_allow_list_cidrs text[] NOT NULL DEFAULT '{}';

UPDATE environments
SET ip_allow_list_cidrs = COALESCE(
    (SELECT array_agg(
         CASE WHEN jsonb_typeof(entry) = 'string' THEN entry #>> '{}'
              ELSE entry ->> 'cidrBlock' END
         ORDER BY idx)
     FROM jsonb_array_elements(ip_allow_list) WITH ORDINALITY AS t(entry, idx)),
    '{}');

ALTER TABLE environments DROP COLUMN ip_allow_list;
ALTER TABLE environments RENAME COLUMN ip_allow_list_cidrs TO ip_allow_list;
