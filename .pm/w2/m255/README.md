# w2 · m255 — Disk usage over 80% notifies, as on Render

**Worker:** worker2 **Goal:** when a service's persistent disk usage exceeds 80% of its allocated size, and when it drops back below, owners get exactly one notice each way. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Disk usage evaluator with hysteresis | 1.5h | — |
| t002 | Durable crossing facts in the service events feed | 1h | t001 |
| t003 | Email both directions plus push | 45m | t002, w2/m252/t005 |
| t004 | dev-2 and production walk | 30m | t003 |
| t005 | Render parity | 15m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- On dev-2 and production, filling a service disk past 80% produces one "above 80%" email and push per eligible member; freeing it below produces one "back below" email.
- Writing and deleting around the threshold does not produce repeated notices (hysteresis).
- The crossings appear in the service events feed; any Render events/webhook naming divergence is recorded in ADR018.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #5. render.com/docs/notifications: "A service's persistent disk usage exceeds 80% of the disk's allocated space" (Email ✓) and "… drops back below 80%" (Email ✓). bex has no such event; the signal exists (`kubelet_volume_stats_used_bytes`, `lego/backend/internal/metrics/source.go:712`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) for the ADR082 persistent disks shipped in w1/m83–m86.
- **Expected outcome:** a disk filling up warns its owner before the app breaks.
- **Why now:** disks are sold; a full disk silently breaks a tenant app; this is the last Render notification row w2/m252 does not cover.
- **Render parity included:** events feed, notifications and settings are tenant-facing.
