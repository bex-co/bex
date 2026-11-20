-- w10/m527: which platform catalog models a workspace's members may choose
-- (ADR047 D7, ADR090 § Model pin amendments 2026-11-20), as Devin's admins
-- choose the agents and models on offer. Stored as the opt-out, like
-- disabled_agents: every existing row and a workspace with no row offer the
-- whole catalog, and a model added to the catalog later starts on. The
-- catalog's default model is never turned off (bex-api refuses a write that
-- leaves it off, and ignores it here if the catalog's default changes).
ALTER TABLE agent_environments
    ADD COLUMN disabled_models text[] NOT NULL DEFAULT '{}' CHECK (cardinality(disabled_models) <= 64);
