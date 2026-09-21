-- w4/m125: what a sync run did that its state cannot say.
--
-- Creating a blueprint for a repo+branch another blueprint already tracks is
-- now refused unless the caller confirms a takeover (BLUEPRINT_CONNECTION_
-- CONFLICT). A confirmed takeover reuses the existing row — same id, new name,
-- path and manifest — so the blueprint that was there before leaves no trace
-- in `blueprints` at all: the row simply reads as the new one. Sync history is
-- the only blueprint-scoped trail, and its columns are state, timestamps, a
-- commit and an error message, none of which can say "this run replaced a
-- blueprint named X".
--
-- note is that sentence. Empty string, not NULL, matching the deploys.
-- stall_reason convention added one migration earlier: it is an optional
-- annotation with a natural "nothing to say" value, and every read path treats
-- "" as absent.
ALTER TABLE blueprint_syncs
  ADD COLUMN IF NOT EXISTS note text NOT NULL DEFAULT '';
