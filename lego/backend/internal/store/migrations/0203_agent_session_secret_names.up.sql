-- ADR098: the names of the secrets attached to an agent session (workspace
-- agent secret references plus inline session secrets). Values live only in
-- OpenBao; this column never holds a value or a hash of one.
ALTER TABLE agent_sessions ADD COLUMN secret_names text[] NOT NULL DEFAULT '{}';
