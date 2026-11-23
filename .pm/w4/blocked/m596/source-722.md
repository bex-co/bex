# 722 — Render's OpenAPI drift round of 2026-11: `POST /sandboxes/{sandboxId}/suspend` and 30 changed operations

Why: the weekly Render drift workflow has been red on its `render-schema` job since the 2026-11-16 schedule, and no board item owns it, so every later API change Render makes is hidden behind a red that is already expected.

**Severity:** major. **Estimate:** unknown until the diff is read; promote to a milestone at triage if it is over an hour. **Source:** `/loopx w4` orchestrator, 2026-11-22, from the w4/m553 worker's note on run [45095344162](https://github.com/bex-co/bex/actions/runs/45095344162) (`render-changelog`, `render-mcp` and `render-examples` green; `render-schema` red). The scheduled run [43902485998](https://github.com/bex-co/bex/actions/runs/43902485998) (2026-11-16) was red too; 2026-11-09 and 2026-11-10 were green.

## Evidence

`scripts/render-schema-drift.sh` against `https://api-docs.render.com/openapi/render-public-api-1.json` and the pin `renderOpenAPISHA256` (`lego/backend/internal/api/render_openapi.go:45`):

- **Added upstream:** `POST /sandboxes/{sandboxId}/suspend`. Removed: none.
- **Changed inside their schemas, among operations bex serves:**
  - Logs: `GET /logs`, `GET /logs/subscribe`, `GET /logs/values`.
  - Metrics: 18 `GET /metrics/*` operations: `active-connections`, `bandwidth`, `bandwidth-sources`, `cpu`, `cpu-limit`, `cpu-target`, `disk-capacity`, `disk-usage`, `filters/application`, `filters/http`, `filters/path`, `http-latency`, `http-requests`, `instance-count`, `memory`, `memory-limit`, `memory-target` and `replication-lag`.
  - Others: `GET /notification-settings/overrides`, `GET /postgres/{postgresId}`, `PATCH /postgres/{postgresId}`, `POST /postgres`, `POST /postgres/{postgresId}/recovery`, `POST /environments/{environmentId}/resources`, `GET /sandboxes`, `POST /sandboxes` and `POST /sandbox-groups`.
- **The same job's CLI check:** `render-oss/cli v2.38.0 is newer than bex's pin v2.36.0` (`lego/cli/UPSTREAM_RENDER_CLI.md`). [w22/m444](../../../w22/done/m444/README.md) owns the move to v2.37.0 only.

## Fix

1. Run a drift round as ADR018's round paragraphs describe. Diff each changed operation against the pinned document. Decide how bex answers every added or changed parameter, body field and response field: serve it, refuse it by name, or document it with an owner.
2. `POST /sandboxes/{sandboxId}/suspend` is either mounted with Render's semantics (bex already suspends idle sandboxes, w2/m278) or listed in `renderOperationOmissions` with an owning board item.
3. Re-pin `renderOpenAPISHA256` and record the round in ADR018.
4. CLI v2.38.0: hand it to w22/m444, which can widen to v2.38.0 or file the step after it, rather than bumping the pin here.

## Acceptance criteria

- [ ] `scripts/render-schema-drift.sh` reports no OpenAPI drift against Render's live document.
- [ ] A `render-schema-drift.yml` dispatch on `main` is green on `render-schema`, or red only on the CLI pin with w22/m444 named as its owner.
- [ ] ADR018 records the round with every changed field's disposition.
