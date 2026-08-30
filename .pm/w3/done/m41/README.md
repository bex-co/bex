# w3 · m41 — Phase-1 fire-and-forget: session → draft PR + evidence (ADR047 D4/D8)

**Worker:** worker3 **Goal:** the phase-1 product moment — a tenant fires `POST /v1/agent-sessions` and gets back a draft PR on their repo with Codex-style verifiable evidence, steerable by a follow-up prompt turn; live-verified end to end on the real substrate. **Status:** done (audited 2026-08-30; production verifier green 2026-08-18)

## Gating

Integration milestone — do **not** start until **w3/m37** (driver + image), **w3/m38** (repo credentials), and **w3/m39** (session API) are done; run the E2E under **w3/m40**'s egress policy if it has landed (recommended).

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| t001 | Completion flow: push `bex-agent/*` branch, open draft PR via the GitHub App | 60m | — — **DONE** |
| t002 | Evidence capture: command log / test-output tails into the session record    | 45m | t001 — **DONE** |
| t003 | Steering v1: new prompt turn resumes the session sandbox                     | 60m | t001 — **DONE** |
| t004 | Live E2E verification script (`scripts/agent-session-verify.sh`) on prod     | 90m | t002, t003 — **DONE** |
| t005 | Render parity: session result fields consistent across REST/GraphQL/MCP      | 30m | t004 — **DONE** |
| t006 | Simplify pass over the completion/steering code                              | 20m | t005 — **DONE** |
| t007 | Test coverage: completion, PR open, evidence, steering, failure paths        | 45m | t005 — **DONE** |
| t008 | Closeout                                                                     | 10m | t007 — **DONE** |

## Definition of done

- A completed session pushes its `bex-agent/*` branch with the m38 session token and bex-api opens a **draft PR** via the GitHub App (ADR047 D4); the PR URL and branch land on the session record and surface across REST/GraphQL/MCP.
- The session record carries verifiable evidence sourced from the driver's captured stream (m37 t002 session log): command log and test-output tails, Codex-style.
- Steering v1 (D8 phase 1): a new prompt turn against an existing session resumes/re-dispatches the sandbox and continues on the same branch — no interactive attach required.
- `scripts/agent-session-verify.sh` proves the whole path on the live substrate: create session on a real repo → agent commits → draft PR exists with evidence → steering turn produces a follow-up commit; failure paths (agent crash, mint refusal) surface as failed sessions, not hangs.
- Backend suite + lint green.

## Closeout evidence

- The original production run on 2026-08-04 (`65ea0e6d`) passed create → draft PR with evidence, steering to a second commit, and non-`bex-agent/*` refusal; `w3/m43` recorded and closed against that same verifier.
- The stricter post-recovery run on 2026-08-18 is recorded in [`w5/m72`'s production evidence](../../../w5/done/m72/evidence/2026-08-18-git-delivery-recovery.md): workflow run `32194179340` deployed the corrected image; session `ags-da2ege4qlbqc73e84d0g` opened draft PR #58, advanced the same branch/PR on steering (`turns=2`, `deliveryMode=redispatch`), retained an empty `failureReason`, refused both a protected branch and an unknown adapter, and the verifier ended with `ALL AGENT-SESSION CHECKS PASSED`.
- The same recovery milestone proved failed child/driver paths converge to a durable terminal state instead of hanging, closing the crash-path exception that the first 2026-08-04 run had called out. Current `main` retains the verifier's hard bounded failure assertion.
- Later successful production deployment run `33295901592` (2026-08-30, commit `c10c4f37`) contains the full m41 implementation and recovery fixes. The failed newer run `33296129237` built/pinned images but stopped on a missing `BEX_ONBEX_DNS_API_TOKEN`; it does not invalidate the already-running successful deployment.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../../docs/ADR047-cloud-coding-agent-sessions.md) D4 + D8 phase 1; `/pm-brainstorm` decomposition 2026-08-01.
- **Goal linkage:** pillar 5 — this is the shippable product shape (Copilot fire-and-forget); wave 2 (live attach, dashboard UI, token metering) is purely additive on this session model.
- **Expected outcome:** "code on the tenant's repo in our cloud" works end to end for a real tenant repo, with delivery and evidence a reviewer can trust.
- **Why now:** the serial integration point of ADR047 wave 1 — everything before it is parallel; wave-2 milestones are held until this lands (see `.pm/w3/009.md`). Render parity included (t005): the session result fields are tenant-facing across REST/GraphQL/MCP — cross-surface consistency per the m39 precedent (Render has no equivalent product; bex-extension row in ADR018).
