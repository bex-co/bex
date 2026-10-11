# w4 · m200 — Preserve logical records in mixed stdout/stderr pod logs

**Worker:** worker4 **Goal:** complete stdout and stderr records survive live/fallback pod-log reads without joining or splitting their message bytes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Prove complete-record framing at the Kubernetes log producer | 50m | — |
| t002 | Build and pin the proved producer artifact through the worker-image workflow | 55m | w4/m200/t001 |
| t003 | Audit and verify the global producer consumers and temporal options | 50m | w4/m200/t001 |
| t004 | Verify the producer artifact through the real pod API and live dashboard | 45m | w4/m200/t002, w4/m200/t003 |
| t005 | Render parity | 20m | w4/m200/t004 |
| t006 | Simplify | 15m | w4/m200/t005 |
| t007 | Test coverage | 50m | w4/m200/t005 |
| t008 | Closeout | 20m | w4/m200/t006, w4/m200/t007, w4/m199/t006 |

## Definition of done

- Repeat the quiet owned Free Python fixture from [finding.md](finding.md): short stdout, short stderr, a 48,065-byte stdout-only JSON control, and a 48,067-byte stdout JSON surrounding an independent 89-byte stderr line. Both larger batches produce exactly the **five complete expected messages** through SSE and generic REST/GraphQL history. In particular the mixed pair is intact 48,067 + 89 bytes; neither the 32,857-byte joined prefix nor the 15,299-byte orphan suffix exists.
- Repeat after a fresh page load/new live connection. The 24 KB control remains complete. Preserve original message bytes, instance and first-chunk logical timestamps; the message fields must survive byte comparisons, not just line counts.
- At fresh desktop and 390×844, Application logs + search `qa-a179-b2-mixed` show exactly the complete stdout and stderr records with Live off, on, after reload and after toggling. This complete identity guarantee requires m199's first-chunk timestamp correction to be deployed and verified; do not bypass it with lossy dedupe.
- Keep the pod-backed Live availability contract from ADR010. Verify the scoped/global producer behavior and all required sibling/lifetime/temporal checks from t003/t007 before declaring the artifact suitable.
- Walk all four original m96 DoD items: two scheduled + one manual Cron, equal messages with literal data-space/newline variants, successful/manual/canceled-run controls, and the Cron deletion/detail-404/Overview disappearance. Those are required follow-up checks, not a179 live coverage.
- Delete every owned fixture, verify public/detail 404 plus workspace absence, revoke only the run's QA session, and retain complete probes with the actual producer artifact identity.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, 2026-10-11 UTC, pass a179; [finding.md](finding.md) includes two independent emissions, desktop/mobile replays, exact history/SSE bytes and an executed declared-version CRI reader probe. Local screenshot/ledger paths are supplementary; complete probes are embedded durably.
- **Goal linkage:** ADR008's Render-alternative hosting promise and ADR010 observability: developers must be able to read their application's actual logs through the supported API/dashboard.
- **Expected outcome:** mixed stdout/stderr partial output retains its original complete messages through the pod-log response; no fabricated prefix+stderr record or orphan stdout suffix reaches customers.
- **Why now:** two normal below-cap messages are corrupted in Live while historical ingestion is correct. Reload/toggle adds the corrupt record beside the correct historical pair. A frontend repair cannot reconstruct the erased stream boundary.
- **Severity / size:** major; 305m across eight tasks. The wider artifact/consumer work warrants a milestone.
- **Scope:** global node log-producer correction with an explicit consumer audit. Prefer a demonstrated upstream artifact; a narrow source-pinned kubelet patch/build is the documented fallback. No node configuration or product code was changed by the hunt.
- **Dependencies:** m199 retains ownership of first/final CRI timestamps and gates this milestone's combined identity closeout. w4/256 owns cap alignment independently. Blocked m174's fast-exit ingestion loss remains open.
- **Render parity:** included because message integrity/identity affects user log surfaces. Preserve the existing Render-shaped envelopes and supported/refused transports rather than inventing a new public record format.
- **Unverified:** live node version, the proposed producer patch/artifact, non-web resources, WebSocket/CLI/MCP, node rollover, and the original Cron controls. Concrete verification work is assigned, not claimed complete.
