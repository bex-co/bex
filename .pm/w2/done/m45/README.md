# w2 · m45 — Blueprint `initialDeployHook`: one-time first-deploy command

**Worker:** worker2 **Goal:** Render's Blueprint `initialDeployHook` — a command that runs exactly once, on a service's first successful deploy — works in `bex.yml`, riding the existing pre-deploy Job mechanism with a ran-once marker, retiring another "accepted but ignored" blueprint field. **Status:** DONE 2026-07-16 — annotation-based ran-once tracking; `bex.co/initial-deploy-hook` persists the command, `bex.co/initial-deploy-hook-ran` marks after `PreDeploySucceeded`; 4 new stack tests; full suite green

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| t001 | Pin Render semantics; parse + persist the field with a ran-once marker      | 30m | —          | — **DONE**
| t002 | Execute via the pre-deploy Job path, gated on first successful deploy only  | 45m | t001       | — **DONE**
| t003 | Surface echo + no-rerun verification + ledger updates                       | 30m | t002       | — **DONE**
| t004 | Render parity                                                                | 30m | t003       | — **DONE**
| t005 | Simplify                                                                     | 20m | t004       | — **DONE**
| t006 | Test coverage                                                                | 30m | t004       | — **DONE**
| t007 | Closeout                                                                     | 15m | t006       | — **DONE**

## Definition of done

A `bex.yml` with `initialDeployHook` provisions a service whose hook command runs exactly once, on the first successful deploy (observable in the pre-deploy Job path); subsequent deploys and blueprint re-syncs do not re-run it (test-asserted via the ran-once marker); the field echoes on blueprint reads; `docs/ADR018-render-parity.md:49`'s "recorded as unsupported" marker is replaced with evidence.

## Source + Goal linkage

- **Source:** `w2/013` (filed by `/pm-brainstorm` round 12's docs miner from `docs/ADR018-render-parity.md:49` + the ADR006 "ignored" list); promoted by `/pm-brainstorm` round 17.
- **Goal linkage:** Blueprint/IaC parity (pillar 4, deploy-from-chat — agents declare whole stacks; ADR018 Blueprint row).
- **Expected outcome:** one fewer accepted-but-ignored blueprint field — the exact class the divergence-mining rounds exist to eliminate.
- **Why now:** the smallest remaining unowned blueprint gap; the `w1/m33` pre-deploy Job mechanism it rides is stable and shipped, so this is marker-plus-gating work, not new mechanism.
- **Render parity:** included — blueprint validate/sync surfaces (REST/GraphQL/MCP) and the field's echo shape change.
