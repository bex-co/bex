# w4 · m695 — A Blueprint `scaling` block works on a background worker, and export round-trips it

**Worker:** worker4 **Goal:** a Blueprint can declare autoscaling for a background worker, as Render's spec allows, and exporting an autoscaled worker writes its `scaling` block instead of flattening it to a fixed count. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Validate and sync accept `scaling` on a background worker | 45m | — |
| [t002](t002.md) | Export writes an autoscaled worker's `scaling` block | 30m | t001 |
| [t003](t003.md) | dev-4: a worker Blueprint with `scaling` validates, syncs and exports unchanged | 30m | t002 |
| [t004](t004.md) | Render parity | 25m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

Total: **7 tasks, about 3h05m**.

## Definition of done

- Blueprint validate and sync accept a `scaling` block on a `type: worker` service on every surface and turn on its autoscaling with the declared range and targets.
- Generating a Blueprint from an autoscaled worker writes `scaling` (min, max, targets), not `numInstances: <max>`, and the generated file validates.
- On dev-4, a worker Blueprint with `scaling` syncs, and its export matches the file.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: scaling and autoscaling), item 2. Re-checked by hand at `origin/main` `20dfa991d4` before filing; no open, blocked or done item owns it.
- **Render:** [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec) § Scaling: "These fields apply scaling settings for the following service types: Web services, Private services, Background workers." The pinned schema (`lego/backend/internal/apps/schema/render.yaml.json`) puts `scaling` on `serverService`, whose types include `worker`.
- **bex today:** validation refuses it (`lego/backend/internal/apps/deploy.go:3460-3461`, "scaling is not available on background workers"), and export writes an autoscaled worker as `numInstances: <max>` (`blueprint_generate.go:467-485`, pinned by `blueprint_generate_test.go:248-256`). The API already supports worker autoscaling (w4/m101).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a Render Blueprint with an autoscaled worker deploys on bex, and an export can be synced back without losing the worker's autoscaling.
- **Why now:** a Render-style file fails validation with 400 today; it shares the Blueprint scaling code with w4/m694.
- **Render parity:** included: Blueprint validate, sync and export are served on REST, GraphQL, MCP and the dashboard.
