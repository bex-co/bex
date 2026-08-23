# w1 · m86 — Persistent disks 4/4: Blueprint + dashboard + records closeout

**Worker:** worker1 **Goal:** a Render user's `render.yaml` with a `disk` block deploys on bex; the dashboard grows Render's Disk tab with the live-captured form contract; every record that said "disks are a non-goal" is closed out — ADR082 D6 (dashboard), D7, and § Record changes. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est  | depends_on       |
| ---- | ----------------------------------------------------------- | ---- | ---------------- |
| t001 | Blueprint: flip the four `disk` capability entries          | 1h   | —                |
| t002 | Blueprint estimated-pricing "Disks" group                   | 30m  | t001             |
| t003 | Dashboard Disk tab (live-captured contract)                 | 1.5h | —                |
| t004 | Service-disk metrics wiring for the usage graph             | 30m  | t003             |
| t005 | Records closeout: ADR018 cells, ADR030 row, CLI checklist, pre-GA price check | 30m | t001, t004 |
| t006 | Render parity check (Blueprint corpus + dashboard vs live)  | 30m  | t005             |
| t007 | Simplify pass over the changed code                         | 30m  | t006             |
| t008 | Test coverage: conformance corpus + dashboard suite         | 45m  | t007             |
| t009 | Closeout (incl. the ADR082 e2e mock-cluster drill)          | 30m  | t008             |

## Definition of done

`scripts/app-apply.sh` (→ `/v1/blueprints/deploy`) accepts a `render.yaml` whose service carries `disk {name, mountPath, sizeGB}` and produces a running disk-bearing App; omission preserves an existing disk, explicit shrink fails validation pre-write, removing the block does not delete the disk; the Blueprint review panel shows a Disks pricing group at $0.175/GB-month. The dashboard's Manage sidebar shows **Disk** with the captured contract (empty-state card, warning list, mount-path + 1/5/10/50/100 GB chips defaulting to 10, usage graph, grow, snapshot list/restore with confirmation, delete). ADR018's disk row cells reflect shipped surfaces; the CLI checklist carries the n/a-upstream row; the authenticated Hetzner `GET /v1/pricing` volume-price check is recorded. Dashboard suite + backend conformance corpus green; the ADR082 § Verification e2e drill (write→redeploy→survives; snapshot→restore) passes on the mock cluster.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D6 dashboard, D7, D11 stages 4–5, § Record changes); live dashboard capture [docs/render-artifacts/disks.md](../../../docs/render-artifacts/disks.md) (2026-08-23); anti-goal re-opened 2026-08-22.
- **Goal linkage:** ADR049's honest-parity contract — the largest fail-closed Blueprint refusal class becomes a translated handler; ADR018 ledger truthfulness.
- **Expected outcome:** end-to-end product parity: YAML, API, UI, billing, and records all agree; a Render disk user can migrate without edits.
- **Why now:** final stage of ADR082 D11 — consumes m83–m85's mechanism, verbs, and snapshots; closes the reversal's paper trail so no record contradicts shipped reality.
- **Render parity closing task included** (t006): Blueprint + dashboard surfaces change.
