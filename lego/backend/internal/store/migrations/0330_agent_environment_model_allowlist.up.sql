-- w10/m568: the workspace model list becomes Devin's two modes, "Allow all
-- models" and "Allowlist specific models" (ADR047 D7, ADR090 § Model pin
-- amendments 2026-11-22). A model is allowed when the allowlist names it, so
-- a model added to the platform catalog after an admin narrowed the list is
-- off there until an admin turns it on.
--
-- known_models is the platform catalog an admin's narrowing write saw, and
-- the allowlist is known_models less disabled_models. NULL allows every
-- catalog model, a later one included: a workspace with no row, a row that
-- turns no model off, and a write that turns every model back on. A catalog
-- model outside known_models is new in the catalog and off.
--
-- disabled_models stays exact (the catalog at the write less the allowlist),
-- so a previous-release bex-api reads the row the same way, and a row whose
-- known_models is NULL or whose disabled_models is empty reads as the opt-out
-- 0315 stored: the catalog less disabled_models. bex-api's startup converts a
-- row narrowed under 0315 (known_models NULL, disabled_models not empty) by
-- recording the catalog it starts with as known_models, which keeps its offer.
ALTER TABLE agent_environments
    ADD COLUMN known_models text[] CHECK (cardinality(known_models) <= 64);
