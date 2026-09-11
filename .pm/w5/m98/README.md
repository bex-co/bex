# w5 · m98 — Live-verification sweep on dev-5

**Worker:** worker5 **Goal:** the five residuals that closed w5 milestones recorded as "deferred — cluster unavailable" (and that nothing on the board owns) each get a dated PASS or FAIL line from a live run, with every FAIL fixed in-task when small or filed as an inbox note carrying `file:line`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Bring up `dev-5`, reprovisioning the shared kind/CAPD cluster if needed; record what was rebuilt            | 30m | —                      |
| t002 | m89–m91: multi-replica Metrics walk — instance selection, aggregation, percentages, selection persistence    | 45m | t001                   |
| t003 | m88: agent-turn latency/outcome series via a repo-less session, or record the model-key blocker             | 45m | t001                   |
| t004 | 053: confirm the production availability board's edge 5xx log panel shows platform-host request lines       | 15m | —                      |
| t005 | m65 / m66: `agent-session-verify.sh` legs incl. 1b and a real ACP turn, or record the GitHub-App blocker    | 30m | t001                   |
| t006 | Render parity                                                                                              | 30m | t002, t003, t004, t005 |
| t007 | Simplify                                                                                                   | 20m | t006                   |
| t008 | Test coverage                                                                                              | 30m | t006                   |
| t009 | Closeout                                                                                                   | 15m | t008                   |

## Definition of done

- Every residual named in t002–t005 has a dated **PASS** or **FAIL** line in this README's Evidence section with the exact command/verb, the surface exercised, and the observed result; "reasoned from code shape" is not evidence. A residual that is blocked by credentials this environment cannot supply gets a **BLOCKED** line naming the exact missing input (same standard as `w3/m79` t012).
- Every **FAIL** is fixed inside its task (≤ ~30m, with a regression test) or filed as a `w5/NNN` inbox note carrying `file:line`, and the source milestone's deferred bullet is annotated with the outcome and pointer.
- No w5 closeout under `done/` still reads "deferred", "not verified live", or "awaiting live verification" without an annotation pointing here.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 6 — a sweep of w5 closeouts: `done/m89/README.md:59` ("Live multi-replica walk deferred when cluster unavailable"), `done/m88/README.md:58` ("live dev-5 registry/panel walk deferred (kubectl/dev-5 unavailable at closeout)"), `done/m65/README.md:58` ("`scripts/agent-session-verify.sh` legs — including the new 1b — need a real cluster + GitHub App and have not been run"), `done/m66/README.md:26` ("Live verification (a real `claude-code-acp` turn)…"), and `done/053.md` ("awaiting live verification by the coordinator"). None of these is among the ten residuals `w1/m139` owns.
- **Facts verified 2026-09-10:** the local kind cluster's API refused connections at 21:06 local (`infra/local/bex.kubeconfig`, `127.0.0.1:55003`); `w1/m139`'s evidence records the control plane crash-looping on etcd latency on 09-09 and a green `mock-cluster.sh` re-run on 09-10, so bring-up may again need reprovisioning (pre-approved for `dev-N` and `mock-cluster.sh` per root `CLAUDE.md`).
- **Goal linkage:** pillar 1 correctness of already-shipped work (Metrics page parity, agent-session reliability, ADR088 incident explainability); the board's own rule that a milestone lands in `done/` when its observable end state is real.
- **Expected outcome:** the deferred-verification debt in w5 stops compounding; any defect hiding behind "identical code shape" surfaces with evidence; the 053 panel is proven to explain platform-API incidents.
- **Why now:** each new closeout since 2026-08-20 has added to this list, and `w1/m139` is doing the same sweep for its own residuals — running both while the cluster is being nursed back shares the bring-up cost. Honest caveat: the cluster is down at filing time, so this ranks below the shipping milestones (m94–m97).
- **Render parity included:** in-task fixes may touch the dashboard Metrics UI or the metrics/events API; t002 explicitly compares the dashboard read against REST/GraphQL/MCP for the same window.
