-- w22/m413 t004, ADR047 D12: why a stopped turn's disk was not kept. The
-- retained image's row, which carries release_reason, is deleted once the
-- registry no longer holds the image, so the reason is copied onto the turn,
-- where the API reads it: '' (kept, continued, or no reason known) or the
-- release reason of a lost disk (expired, storage_limit, save_failed, ...), or
-- editor_open for a stop that kept nothing because an editor was open on the
-- sandbox.
ALTER TABLE agent_session_turns
    ADD COLUMN retention_reason text NOT NULL DEFAULT '';
