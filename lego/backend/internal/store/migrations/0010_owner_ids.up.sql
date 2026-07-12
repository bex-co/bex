-- w6/m7: opaque per-identity user ids ("own-<xid>"), the Render "own-" user id.
-- Maps a caller subject (Kratos identity id or Hydra client id — the same value
-- tenant_members.subject holds) to a stable opaque id, so the Render owners
-- members surface can report userId as an own- id instead of leaking the raw
-- subject. Minted lazily on first read (workspaces.Service), one id per subject.
CREATE TABLE owner_ids (
    subject    text PRIMARY KEY,
    own_id     text UNIQUE NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
