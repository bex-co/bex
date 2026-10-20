# CLI sweep 783 — blueprint connection uniqueness (live, 2026-10-19)

Nothing created; the pre-existing blueprint was not modified.

`POST /blueprints` for `bex-co/bex@main` with a different manifest path (`examples/static-site/render.yaml`) → 409 `BLUEPRINT_CONNECTION_CONFLICT`: `blueprint blp-… ("bex") already tracks https://github.com/bex-co/bex@main from "render.yaml"; update it with updateBlueprint to change its path, or retry with confirm="takeover blueprint blp-…" to replace it`.

Connection identity is repo+branch (path changes ride the existing blueprint), and the takeover path is phrase-guarded (ADR032 class). The takeover was **not** exercised — it would hijack a pre-existing blueprint.

No bug filed.
