-- w32/m285 t009: a capture the substrate could not start is requested again.
-- The OpenSandbox server fails a capture with `snapshot_runtime_create_failed`
-- when its own create of the SandboxSnapshot object fails, before anything
-- froze or committed the sandbox. On production on 2026-11-17 three captures
-- ended that way, each because the server's Kubernetes client lost its API
-- connection under the create. bex-api now requests such a capture again, and
-- capture_attempts counts the requests made for one snapshot, which bounds
-- them.
ALTER TABLE sandbox_snapshots
    ADD COLUMN capture_attempts smallint NOT NULL DEFAULT 1 CHECK (capture_attempts >= 1);
