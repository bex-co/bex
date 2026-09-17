-- w2/m163: record how many machine credentials a membership removal revoked.
--
-- Removing a member now revokes the API keys they created in that workspace
-- (that milestone's t001 decision: a key is a delegation of one subject's
-- authority, so ending the membership ends the delegation). That is a
-- deliberately destructive side effect — it can break a CI pipeline the
-- workspace still depends on — so the audit trail has to say it happened, not
-- just that a member was removed.
--
-- One typed column per value, matching maintenance_mode_to / instance_count_to
-- rather than a free-form metadata object (docs/ADR006-bex-api.md's audit
-- shape). NULL for every verb that does not revoke keys, and for a removal that
-- revoked none — 0 means "checked, none existed", NULL means "not applicable".
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS revoked_key_count integer;
