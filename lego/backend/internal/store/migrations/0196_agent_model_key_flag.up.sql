-- ADR091 D7: a workspace's own model provider key (BYO) wins over the platform
-- model route. The key itself lives only in OpenBao at
-- agent-sessions/<workspace>/model-key; this column records that the
-- workspace admin set one (NULL = no BYO key), so route decisions (create,
-- dispatch, the per-request credential mint, the trust-tier session count)
-- never need an OpenBao round trip. Written only by the agent environment's
-- write-only modelApiKey / clearModelApiKey update.
ALTER TABLE agent_environments ADD COLUMN model_key_set_at timestamptz;
