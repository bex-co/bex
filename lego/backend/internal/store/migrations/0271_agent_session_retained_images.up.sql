-- w22/m413, ADR047 D12: a turn a person stopped keeps its disk. Its sandbox is
-- paused, the committed root filesystem is copied to
-- snapshots/<ws>-sandbox/images/<id> (the place a repository environment
-- build's image lives), and the session's next turn boots a fresh sandbox from
-- it. The row is that image's handle, so it outlives the sandbox and is
-- deleted only once the registry no longer holds the image.
--
-- state: saving (the Completer is pausing and copying; the next turn waits for
-- it until save_deadline), kept (usable until retained_until), in_use (a
-- dispatch claimed it and is booting from it, until claimed_until) or released
-- (the image is to be deleted from the registry; release_reason says why).
-- Every transition is a compare-and-set on state, so an expiry, a cancel and a
-- continuation racing each other leave exactly one winner. One live row per
-- session. size_bytes is reserved against the workspace's sandbox storage cap
-- before the image is copied and counts until the row is gone. base_image is
-- the agent template image the disk ran on, so a continuation never boots into
-- an older driver than the template's.
CREATE TABLE agent_session_retained_images (
    id             text        PRIMARY KEY,
    session_id     text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    workspace_id   text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    turn           integer     NOT NULL CHECK (turn > 0),
    state          text        NOT NULL DEFAULT 'saving' CHECK (state IN ('saving', 'kept', 'in_use', 'released')),
    base_image     text        NOT NULL DEFAULT '',
    image_digest   text,
    size_bytes     bigint      CHECK (size_bytes >= 0),
    created_at     timestamptz NOT NULL DEFAULT now(),
    save_deadline  timestamptz NOT NULL,
    retained_until timestamptz,
    claimed_until  timestamptz,
    released_at    timestamptz,
    release_reason text,
    purge_after    timestamptz,
    purge_attempts integer     NOT NULL DEFAULT 0,
    CHECK (state NOT IN ('kept', 'in_use') OR (image_digest IS NOT NULL AND retained_until IS NOT NULL)),
    CHECK (state <> 'in_use' OR claimed_until IS NOT NULL),
    CHECK (state <> 'released' OR released_at IS NOT NULL)
);

CREATE UNIQUE INDEX agent_session_retained_images_live ON agent_session_retained_images (session_id) WHERE state <> 'released';
CREATE INDEX agent_session_retained_images_workspace ON agent_session_retained_images (workspace_id);
CREATE INDEX agent_session_retained_images_expiry ON agent_session_retained_images (retained_until) WHERE state = 'kept';
CREATE INDEX agent_session_retained_images_saving ON agent_session_retained_images (save_deadline) WHERE state = 'saving';
CREATE INDEX agent_session_retained_images_in_use ON agent_session_retained_images (claimed_until) WHERE state = 'in_use';
CREATE INDEX agent_session_retained_images_purge ON agent_session_retained_images (purge_after NULLS FIRST, released_at) WHERE state = 'released';

-- What became of a stopped turn's disk, for the turn's own record: '' (nothing
-- was kept: the turn was not stopped, or retention is off), kept (until
-- retained_until), continued (the next turn started from it) or lost (it could
-- not be kept, or was released before use). start_note is a sentence the turn
-- that started after it adds to its delivery note, when it had to start from a
-- fresh sandbox although an image was kept.
--
-- The check is added NOT VALID: every existing row holds the default, and
-- skipping the validation scan keeps the lock brief. New and updated rows are
-- still checked.
ALTER TABLE agent_session_turns
    ADD COLUMN retention text NOT NULL DEFAULT '',
    ADD COLUMN retained_until timestamptz,
    ADD COLUMN start_note text NOT NULL DEFAULT '';

ALTER TABLE agent_session_turns
    ADD CONSTRAINT agent_session_turns_retention_check
        CHECK (retention IN ('', 'kept', 'continued', 'lost')) NOT VALID;
