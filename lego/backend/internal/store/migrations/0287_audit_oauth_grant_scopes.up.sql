-- w4/m454: GrantOAuthAccessEvent records the scopes a member granted an OAuth
-- client (an MCP agent), as Hydra holds them, identity scopes such as openid and
-- offline_access included. oauth_scopes cannot hold them: it is the CALLER's own
-- token provenance, checked against the closed capability vocabulary. NULL on
-- every other verb and on rows recorded before this.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS oauth_grant_scopes text[];

ALTER TABLE audit_events
  ADD CONSTRAINT audit_events_oauth_grant_scopes_check
    CHECK (
      oauth_grant_scopes IS NULL
      OR (
        cardinality(oauth_grant_scopes) BETWEEN 1 AND 32
        AND char_length(array_to_string(oauth_grant_scopes, ' ')) <= 1024
      )
    );
