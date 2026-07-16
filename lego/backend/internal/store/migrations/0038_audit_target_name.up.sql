-- Datastore webhook payloads require both the immutable resource id and its
-- human display name. Typed audit targets already carry the id; this separate,
-- non-secret scalar preserves the name even after a later rename or deletion.
ALTER TABLE audit_events ADD COLUMN target_name text NOT NULL DEFAULT '';
