-- w6/m101 t001: spec.displayName lived only on the App CR, so the event feed —
-- which joins apps at dispatch time — reported the immutable creation-time name
-- in every webhook and push notification for a renamed service. This column is
-- a read projection of that label; empty means never renamed and readers fall
-- back to apps.name.
ALTER TABLE apps ADD COLUMN display_name text NOT NULL DEFAULT '';
