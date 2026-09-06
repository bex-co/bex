# w5 · m84 — Agent-context continuity across sandbox generations (resume / steer / redispatch)

**Worker:** worker5 **Goal:** a resumed or steered agent session remembers its conversation — the fresh agent process is primed per the ADR047 D3 continuity ladder instead of cold-starting while the dashboard replays history it doesn't have. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Driver: transcript re-priming + task re-delivery (ladder rungs 2–3) — **DONE** | 60m | — |
| t002 | Rung 1: snapshot carries agent session-state dirs + ACP `session/load` when advertised — **DONE** | 60m | t001 |
| t003 | Surface the applied rung: turn annotation + dashboard restored/fresh-context hint — **DONE** | 45m | t001 |
| t004 | Simplify — `/simplify` over the changed code — **DONE** | 30m | t002, t003 |
| t005 | Test coverage — ladder selection, preamble bounds, no double-render, empty-snapshot resume — **DONE** | 45m | t004 |
| t006 | Live E2E: production resume + steer-redispatch answer a context-dependent prompt — **DONE** | 45m | t005 |
| t007 | Closeout — **DONE** | 15m | t006 |

## Definition of done

On production: (a) a hibernated session with real prior turns is resumed and a context-dependent steer ("continue where you left off") is answered **in context**; (b) a steer that redispatches onto a fresh sandbox retains context the same way; (c) resuming a session whose prior turns all failed during setup (the `ags-da9mh5vj596c73en5eq0` shape) re-delivers `agent_config.task` — a bare "try again" retries the task instead of asking what to retry; (d) the dashboard indicates restored-vs-fresh agent context. Unit tests pin ladder selection, the preamble byte budget, and that re-primed context is never double-rendered in replay.

## Source + Goal linkage

- **Source:** user report 2026-08-30 — post-fix resume of `ags-da9mh5vj596c73en5eq0` (dashboard.bex.co/agents/…): agent answered "I need more context. What should I try again?" beneath a fully replayed history. Root: ADR047 D3's continuity promise (`session/load` + "transcript replay is the universal fallback") was never implemented agent-side — replay reaches only the client; ADR059 D3's snapshot scope omitted agent session-state dirs; the setup-failed-resume case was unspecified. ADR047/D3, ADR059/D3+D4, ADR051 amendments (2026-08-30) now specify the contract this milestone implements.
- **Goal linkage:** pillar 5 agent-session product quality; direct follow-on to the m82/m83 reliability arc (those made sessions _run_; this makes multi-turn sessions _coherent_).
- **Expected outcome:** resume/steer behaves like the continuous conversation the UI presents; the "amnesia beneath replayed history" class is gone.
- **Why now:** every hibernate→resume and every redispatch hits this today; it defeats the purpose of hibernation (durable resume) and of the durable transcript.
- **Render parity:** omitted — bex-native agent-session surface (ADR047); Render has no comparable product surface. The t003 hint is a stream/data-part annotation, not an API schema change.

## Implementation and validation

The driver selects native `session/load`, bounded transcript re-priming, or original-task redelivery from actual available context. Native history directories remain in scrubbed hibernation snapshots. The pinned Claude SDK 0.2.44 queues transcript writes; the reviewed `CLAUDE_CODE_EAGER_FLUSH=1` profile setting makes the result wait for native persistence before the driver terminates the adapter. The flag reaches the nested SDK process and runtime overrides cannot disable it.

Recognized missing/corrupt native state triggers exactly one clean adapter replacement and the existing transcript/task fallback. Authentication, routing, unknown errors, cancellation, and timeouts remain fatal. Cleanup also terminates descendants retaining ACP pipes. Historical notifications from native loading never enter the new turn's stream, mirror, or JSONL, including partial replay before a failed load.

Earlier-turn errors in durable replay become nonfatal `data-bex-turn-error` entries, so a previous failure cannot hide a later recovered turn. Current-turn and transport errors remain fatal. Stored bytes and quota accounting are unchanged. Dashboard failure entries are localized, escaped, and bounded. Mobile has no ACP/transcript-stream consumer and retains its current-failure overview.

Validation: all 119 driver tests and build; all 2,997 dashboard tests in 397 files plus full lint/typecheck/knip; all 342 mobile unit tests; full backend suite and backend lint (zero issues); gateway race tests. Real SDK/HTTP regressions verify historical failure → later native hint → later answer, original-byte quota, no storage mutation, and current-error behavior. The failing original implementations were checked against the new regressions. The final production workflow's backend (with integration services), dashboard, operator, and controller gates all passed. Simplify reviews covered reuse, quality, and efficiency; replay adaptations share one JSON decode per part.

## Production evidence (2026-09-06 UTC)

Only disposable sessions were used. The reported existing session `ags-da9mh5vj596c73en5eq0` was not modified.

| Scenario | Session / turn | Completed UTC | Delivery / continuity | Result |
| --- | --- | --- | --- | --- |
| Initial conversation | `ags-daeav32uqgtc739gsjm0` / 1 | 00:11:24 | fresh | Remembered MARIGOLD-742 and 37. |
| Completed-session follow-up | same / 2 | 00:12:24 | redispatch / transcript-reprime | MARIGOLD-742 and 37. |
| Hibernate → explicit Resume → Steer | same / 3 | 00:18:41 | redispatch / transcript-reprime | MARIGOLD-742 and 37. |
| Setup-only failure → bare “try again” | `ags-daeb00quqgtc739gsjpg` / 2 | 00:16:39 | rehydrate / task-redelivery | The recovery codename is COPPER-915. |
| Legacy snapshot with rejected native load | positive session / 6 | 01:41:02 | rehydrate / transcript-reprime | MARIGOLD-742 and 37. |
| Snapshot created by repaired driver | positive session / 7 | 01:42:47 | rehydrate / session-load | MARIGOLD-742 and 37. |

Successful turns were complete and not truncated. The setup fixture was a new empty private repository: bootstrap failed because main did not exist, leaving a durable original task and zero transcript parts. Seeding main allowed the task-only retry to recover without a fresh task prompt. The literal resume scenario hibernated at 00:16:10, restored into sandbox `c77f4156-70d9-4ac1-a6ee-2718ce4c2611`, then steered onto `ec5b15b7-a7ed-470c-aecd-583ae338b744`.

The legacy native-load failure was reproduced on turn 4. A successful old-driver turn 5 produced the preserved 5,938-byte snapshot at 00:35:10. After repair, turn 6 in `2aa6981e-e627-45f4-a10e-818e7553e972` logged the same query-closed error and recovered in a fresh process. Its annotation was at 01:40:56.684; nine parts contained only the new answer. That turn hibernated at 01:41:48 into a 9,452-byte snapshot. Turn 7 in `ec41c9c8-8fb7-4bd9-b3bc-9c81cca83da0` used native loading (annotation 01:42:45.356), again with exactly nine parts and no duplicated history.

At 02:42:10 UTC, the final deployment rendered Turn 4’s historical failure followed by the later rebuilt-context and native-resume answers. Desktop (1280×720) and narrow-mobile (390×844) screenshots were inspected; there was no mobile horizontal overflow. The task-redelivery hint was rechecked on both widths afterward. Local ignored evidence: `.playwright-mcp/m84-native-{desktop,mobile}.png`, `m84-history-error-{desktop,mobile}.png`, `m84-redelivery-{desktop,mobile}.png`, and `m84-ui-final-result.json`.

### Deployment provenance

- Baseline continuity code `c10c4f37e` was included in successful production run `33994179845` for `88e052053`. API digest `dee4585cee48d49e1eef46c230632c988412fb17eddcf6a2dfe69b33d2765e77`, agent digest `ba730a416972daee207f65a456a39be3eae8e6968fd74445e0426d4842ef2162`.
- Native repair and replay guard shipped in `522d8f9bb` and `04e8fa215`. Successful run `34002482856` and GitOps write-back `ee13635e6` deployed API digest `d9be56f5a148470b061795b0aea6e31cc1957e114871ebbaf61d5d3f5102aa62` and agent digest `b9dbcaa578cf9004d13baec16a223767c006c7512b392aa46280b7c48907199d`; both readiness and the scratch pod's actual image were verified before turns 6–7.
- Historical-error presentation repair shipped in `b6b342f42496e3988d2453d34a57e8670ddf1956`, workflow `34005139041`. The workflow completed successfully. GitOps write-back `203fb863def1b7e4f9fe75d34203f73e336117c8` pins API/gateway digest `14fc266309dfbe1fc7268f6daa43ea297c97afced2cf43e2ac8c18e4457b3450`, dashboard digest `76c2c5fe508b21c24e1dc93d185d478956d59782f18cccd2074fb93514e76a8e`, and agent digest `8e1679210bb21215ed0ca43100e11a33b8356f9ccd0e15245a7241cb37870f71`. Actual images and completed API, gateway, and dashboard rollouts were checked before the final UI verification.

### Cleanup

At 02:44 UTC both scratch sessions were deleted through the public API and returned REST 404; snapshot deletion is part of the successful delete operation. No pods remained for either session. Private repository `bex-co/qa-m84-empty-20260905` was deleted and GitHub returned 404. The temporary QA authentication state was removed. Local screenshots and redacted test evidence remain ignored for review; no existing user session was changed.
