# w11 · m3 — Read-only mobile supervision

**Worker:** worker11 **Goal:** let a tenant identify resource health and recent change from a phone, then inspect events, deploys, metrics, and live logs through unreliable mobile networks without exposing configuration or destructive controls. **Status:** todo

## Gating

Start after `w11/m2/t009`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Build the project-grouped resource status home | 60m | w11/m2/t009 |
| t002 | Add service detail, deploy history, and events timeline | 60m | t001 |
| t003 | Adapt retained charts into metrics snapshots and sparklines | 45m | t002 |
| t004 | Add read-only app and deploy live-log tails | 60m | t002 |
| t005 | Harden mobile reconnect, refresh, stale, and offline behavior | 45m | t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006 |
| t009 | Closeout | 10m | t008 |

## Definition of done

From a phone, an authorized tenant can find service/Postgres/Key Value resources, see current state and latest deploy, inspect the unified events/deploy timeline, view CPU/memory/network snapshots, and tail app/deploy logs. Background/foreground transitions and an interrupted connection recover without duplicate lines or hidden staleness. Empty, unavailable, and partial-data states are honest, and no creation, settings, secret, billing-admin, or destructive route is reachable.

## Source + Goal linkage

- **Source:** ADR048 D1/D2, M1 supervision tier, and dashboard surface inventory.
- **Goal linkage:** ADR008 agent-readable state made equally useful to humans away from a desktop.
- **Expected outcome:** alert-to-evidence triage works from a phone using existing bex primitives.
- **Why now:** read-only evidence is independently useful and de-risks mobile networking before mutations and push depend on it.
- **Render parity:** included for existing service/deploy/event/log/metrics shapes and failure semantics; the mobile presentation may be compact but the contract remains identical.
