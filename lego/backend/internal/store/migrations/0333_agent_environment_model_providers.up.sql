-- w10/m573: a workspace admin may allow the platform catalog's models by
-- provider instead of one by one, Devin's other filter type ("Filter by
-- specific models or by provider (one filter type at a time)"; ADR047 D7 and
-- ADR090 § Model pin amendments 2026-11-22). A model's provider is its catalog
-- id's prefix before '/'.
--
-- enabled_providers NULL filters by model (0330's two modes). Set, the
-- workspace offers the catalog default plus every catalog model of a listed
-- provider, one added to the catalog later included.
--
-- A provider write also stores the catalog it saw as known_models and, as
-- disabled_models, every catalog model but the default whose provider is not
-- listed, so a previous-release bex-api offers the same set under the same
-- catalog. The filter is in force only while known_models is set and
-- disabled_models is exactly what a provider write derives from known_models
-- and enabled_providers: a previous-release enabledModels write changes
-- disabled_models and leaves this column in place, and the row then reads by
-- model, as that Pod reads it. An enabledModels write stores NULL here.
ALTER TABLE agent_environments
    ADD COLUMN enabled_providers text[] CHECK (cardinality(enabled_providers) <= 64);
