# CLI sweep 583 — `services update --name`: rename is a display-label edit, not a resource rename

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Two owned free fixtures (`qa583-web`, `qa583-other`), both deleted with REST 404 and zero `qa583`/`QA583` names remaining. The initially surprising acceptance of an invalid resource name was traced to the documented name contract before any filing.

- **What the wire does:** the pinned CLI sends top-level `name` in `PATCH /v1/services/{id}`; bex coalesces it onto `displayName` ([rest.go](/lego/backend/internal/apps/rest.go:913)), because the App's resource name is immutable Kubernetes identity. This is the conscious divergence recorded in [ADR018](/docs/ADR018-render-parity.md) (w8/034, ADR006 settled name contract).
- **Rename semantics verified live:** renaming leaves `immutableName`, `slug`, and the public URL (`qa583-web.onbex.co`) unchanged; `name` and `displayName` in reads follow the new label. Renaming to another live service's label is refused `409 (CONFLICT) … already in use`; renaming to the same label is a no-op success.
- **Display-name grammar enforced:** `QA583_Bad` and `My QA583 Service` (uppercase, underscore, spaces) are accepted as labels — correctly outside the create-side `RESOURCE_NAME_INVALID` rule. Refusals: ID-shaped label → `400 (NAME_RESOURCE_ID_RESERVED)`; 300-char label → `400 (DISPLAY_NAME_TOO_LONG): display name must be at most 100 Unicode code points` (46 chars accepted). Empty `--name ''` fails client-side (`at least one update flag must be provided`) — the documented "CLI cannot express an empty clear" limit.
- **Resolution:** the old label stops resolving by name immediately (`No service named 'qa583-web'`), the new label resolves. One resolver error string still says `render workspace set …` — the pinned upstream client's own wording, a documented residual, not filed.
- **Neighbor dialects:** `--plan banana` via update → the same named plan 400 as create; `--num-instances` is not an update flag at this pin (client: `not supported for update (use the dashboard …)`).

## Unrun

- Display-name control characters / Unicode line separators through the CLI (pinned client normalizes edge whitespace; the raw API contract is covered by w8/034's own tests).
- Rename of Postgres/Key Value instances through their CLI update verbs.
