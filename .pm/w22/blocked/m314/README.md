# w22 · m314 — Sandbox CPU billed on use, as on Render: a gVisor accuracy spike, then the cutover

**Worker:** worker22 **Goal:** a running sandbox is billed for the CPU it uses plus the memory allocated, and the billing switches in one step, or the fallback is recorded for the user's sign-off **Status:** blocked (t001, the production CPU-accuracy spike, has its thresholds recorded and no measurement; t002–t009 depend on its verdict and are not started)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Spike: does gVisor report usable per-sandbox CPU? Fix the thresholds, run, record the verdict | 1h30m | — |
| t002 | Record the pod name on every running interval | 1h | t001 |
| t003 | Meter used CPU from Prometheus: sandbox_cpu_seconds | 2h | t001, t002, w22/m313/t002 |
| t004 | Bill on use: price CPU and memory, stop billing the folded meter (or take the recorded fallback) | 1h30m | t003, w22/m313/t005 |
| t005 | Production check with a known load, and amend ADR023, ADR030 and ADR104 | 1h30m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- The CPU spike has a recorded verdict. Pass: used-CPU billing is live for every workspace from one dated timestamp and verified on production with a known load. Fail: the allocated-CPU charge is recorded in ADR018 as a divergence with the user's sign-off and a dated removal task. Escalate: the milestone is parked with the exact question for the user
- CPU is billed on use and memory on allocation while running; suspended sandboxes pay storage only (with w22/m313)
- `sandbox_compute_seconds` history and the agent-session cost are unchanged
- ADR023, ADR030, ADR018 and ADR104 D8 agree with the code

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gap 8 (CPU meter and cutover)
- **Goal linkage:** Render parity first (ADR008): Render bills actively-used CPU plus allocated memory; usage metering and billing (ADR023, ADR030, ADR040); pillar 5 sandboxes
- **Expected outcome:** An idle running sandbox stops costing as much as a busy one, and the bill for a known load matches the work done
- **Why now:** An idle-but-running sandbox costs the same as a busy one on bex today, the most visible billing difference from Render; it depends on w22/m313's meters and on a spike whose result decides the branch.
- **Depends on:** w22/m313 (storage meter, memory meter, rates and surfaces).
- **Render parity:** included, usage surfaces change.
- **Recommendation, not a user decision:** bill measured cgroup CPU with no subtraction (ADR104 D8). The user is asked only if the spike's escalation thresholds trip or it fails; then the fallback needs a signed ADR018 divergence.
- **Allowlists:** none; one dated effective timestamp for every workspace (parity rule).
