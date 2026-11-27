# w4 · m694 — A Blueprint's `numInstances` leaves autoscaling on, as Render's spec says

**Worker:** worker4 **Goal:** syncing a Blueprint that declares `numInstances` but no `scaling` block stores the manual count and leaves a service's autoscaling exactly as it was, as Render documents, instead of silently turning autoscaling off. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | `numInstances` without `scaling` stores the manual count and keeps autoscaling on | 45m | — |
| [t002](t002.md) | The disk refusal and ADR018 stop telling users that `numInstances` turns autoscaling off | 30m | t001 |
| [t003](t003.md) | Tests, and a dev-4 sync of an autoscaled service from a file that declares `numInstances` | 1h | t002 |
| [t004](t004.md) | Render parity | 25m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

Total: **7 tasks, about 3h35m**.

## Definition of done

- A Blueprint sync of a file that declares `numInstances` and no `scaling` on a service with autoscaling enabled keeps autoscaling enabled with its min, max and targets unchanged, stores the declared count as the manual instance count, and records no `autoscaling_config_changed` event. The same holds for validate's plan on REST, GraphQL, MCP and the dashboard.
- A file that declares `scaling` still sets autoscaling as before, and a service without autoscaling still takes `numInstances` as its running count.
- The disk refusal for an autoscaled service no longer advises declaring `numInstances`; it names a way that works (turning autoscaling off on the service).
- On dev-4, an autoscaled web service synced from such a file keeps autoscaling, and ADR018's services-lifecycle row describes the new rule.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: scaling and autoscaling), item 1. Found by a research agent and re-checked by hand at `origin/main` `20dfa991d4` before filing; no open, blocked or done item owns it.
- **Render:** [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec): "`numInstances` … _This value has no effect for services with autoscaling enabled._" and "If you add an existing service to a Blueprint, that service retains any existing autoscaling settings unless you add the `scaling` field in your Blueprint."
- **bex today:** `lego/backend/internal/apps/blueprint_plan.go:480-483` calls `turnOffAutoscaling(dst)` whenever the file applies `numInstances` without `scaling`, and the disk refusal at `:502-503` tells users to "declare numInstances in the file to turn autoscaling off". The rule came in without a Render citation; ADR018 (`docs/ADR018/services-lifecycle.md`, from w4/597) records it as plain behavior, with no user sign-off.
- **Not verified:** how Render lets a Blueprint turn autoscaling off (for example by removing a `scaling` block it used to declare); Render does not document it, so that path (`:467-469`) is left as it is.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a team that configured autoscaling in the dashboard can sync a Render-style Blueprint without losing it.
- **Why now:** the current rule silently changes a running service's scaling on every sync of a common file shape; w4/m695 changes the same Blueprint scaling code, so the two go together.
- **Render parity:** included: Blueprint validate and sync are served on REST, GraphQL, MCP and the dashboard.
