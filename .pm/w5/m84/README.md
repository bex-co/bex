# w5 · m84 — Agent-context continuity across sandbox generations (resume / steer / redispatch)

**Worker:** worker5 **Goal:** a resumed or steered agent session remembers its conversation — the fresh agent process is primed per the ADR047 D3 continuity ladder instead of cold-starting while the dashboard replays history it doesn't have. **Status:** in progress — native restoration and missing-state fallback now verified live; historical error replay hides later turns in the dashboard. replay presentation repair and reviews complete; t003/t006 await production UI verification.

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Driver: transcript re-priming + task re-delivery (ladder rungs 2–3) — **DONE**                     | 60m | —          |
| t002 | Rung 1: snapshot carries agent session-state dirs + ACP `session/load` when advertised — **DONE** | 60m | t001       |
| t003 | Surface the applied rung: turn annotation + dashboard restored/fresh-context hint       | 45m | t001       |
| t004 | Simplify — `/simplify` over the changed code — **DONE** | 30m | t002, t003 |
| t005 | Test coverage — ladder selection, preamble bounds, no double-render, empty-snapshot resume — **DONE** | 45m | t004       |
| t006 | Live E2E: production resume + steer-redispatch answer a context-dependent prompt        | 45m | t005       |
| t007 | Closeout                                                                                | 15m | t006       |

## Definition of done

On production: (a) a hibernated session with real prior turns is resumed and a context-dependent steer ("continue where you left off") is answered **in context**; (b) a steer that redispatches onto a fresh sandbox retains context the same way; (c) resuming a session whose prior turns all failed during setup (the `ags-da9mh5vj596c73en5eq0` shape) re-delivers `agent_config.task` — a bare "try again" retries the task instead of asking what to retry; (d) the dashboard indicates restored-vs-fresh agent context. Unit tests pin ladder selection, the preamble byte budget, and that re-primed context is never double-rendered in replay.

## Source + Goal linkage

- **Source:** user report 2026-08-30 — post-fix resume of `ags-da9mh5vj596c73en5eq0` (dashboard.bex.co/agents/…): agent answered "I need more context. What should I try again?" beneath a fully replayed history. Root: ADR047 D3's continuity promise (`session/load` + "transcript replay is the universal fallback") was never implemented agent-side — replay reaches only the client; ADR059 D3's snapshot scope omitted agent session-state dirs; the setup-failed-resume case was unspecified. ADR047/D3, ADR059/D3+D4, ADR051 amendments (2026-08-30) now specify the contract this milestone implements.
- **Goal linkage:** pillar 5 agent-session product quality; direct follow-on to the m82/m83 reliability arc (those made sessions *run*; this makes multi-turn sessions *coherent*).
- **Expected outcome:** resume/steer behaves like the continuous conversation the UI presents; the "amnesia beneath replayed history" class is gone.
- **Why now:** every hibernate→resume and every redispatch hits this today; it defeats the purpose of hibernation (durable resume) and of the durable transcript.
- **Render parity:** omitted — bex-native agent-session surface (ADR047); Render has no comparable product surface. The t003 hint is a stream/data-part annotation, not an API schema change.

## Production audit in progress (2026-09-05 PDT / 2026-09-06 UTC)

The original continuity commit `c10c4f37e` is included in successfully deployed `88e052053` (run `33994179845`, attempt 2). API image `dee4585cee48d49e1eef46c230632c988412fb17eddcf6a2dfe69b33d2765e77` and agent image `ba730a416972daee207f65a456a39be3eae8e6968fd74445e0426d4842ef2162` were confirmed before testing. Scratch sessions only; the reported existing session was not modified.

- `ags-daeav32uqgtc739gsjm0`: initial chat remembered MARIGOLD-742 and 37. Completed-session redispatch on turn 2 returned both values with `transcript-reprime`; archive-triggered hibernation, explicit resume, and subsequent Steer on turn 3 did the same.
- `ags-daeb00quqgtc739gsjpg`: an empty private scratch repo caused bootstrap failure (`remote base branch main does not exist`) with a durable task and zero transcript parts. After seeding the repo, a hibernated-session Steer containing only `try again` returned the original task answer, COPPER-915, with `task-redelivery`. Both fresh-context hints were visible at desktop/mobile widths.
- **Remaining defect:** direct Steer from a hibernated real-history session (turn 4, 00:20:57 UTC) failed native `session/load` with `Query closed before response received`. `acp.ts` propagated load rejection instead of selecting the specified transcript fallback. t002 is reopened; its repaired driver must ship and pass this same live path before closeout.


### Live continuity evidence before the repair

| Path | Completed (UTC, 2026-09-06) | Delivery / annotation | Result |
| --- | --- | --- | --- |
| Initial conversation, session `ags-daeav32uqgtc739gsjm0`, turn 1 | 00:11:24 | fresh | Remembered MARIGOLD-742 and 37. |
| Completed-session follow-up, turn 2 | 00:12:24 | redispatch / transcript-reprime | MARIGOLD-742 and 37. |
| Archive → hibernate at 00:16:10 → explicit Resume → Steer, turn 3 | 00:18:41 | redispatch / transcript-reprime | MARIGOLD-742 and 37. |
| Setup-only failure, session `ags-daeb00quqgtc739gsjpg`, then bare “try again”, turn 2 | 00:16:39 | rehydrate / task-redelivery | The recovery codename is COPPER-915. |

The positive session used fresh sandbox generations `d96ee32d-aa0f-4218-9919-d6c0cc7c16e7`, `f63bc721-1ff1-437b-b7c0-82b7ead7c6cc`, restore-only `c77f4156-70d9-4ac1-a6ee-2718ce4c2611`, and follow-up `ec5b15b7-a7ed-470c-aecd-583ae338b744`. The setup retry used `69113142-0f5b-4258-946f-1360261c1739`. Successful turns were durable and neither incomplete nor truncated. The setup fixture was a new private scratch repository whose missing main branch was seeded after the failure; no existing user session was retried.

Dashboard checks waited for the exact continuity hints at desktop and narrow-mobile widths. Local ignored captures: `.playwright-mcp/m84-reprime-{desktop,mobile}.png` and `.playwright-mcp/m84-redelivery-{desktop,mobile}.png`. Raw failure capture: `.playwright-mcp/m84-native-load-failure.json`. These local artifacts are not committed.


### Native-state repair

The pinned `@zed-industries/claude-code-acp@0.16.2` bundles Claude Agent SDK `0.2.44`. Inspection of that installed SDK found a concrete persistence race: `appendEntry` enqueues transcript writes without awaiting them, the writer waits up to 100 ms, and successful result delivery awaits `flush()` only when `CLAUDE_CODE_EAGER_FLUSH` (or cowork mode) is enabled. The driver terminates the adapter after receiving the result. The reviewed Claude profile now requests eager flushing; no timing sleep is used. ACP and SDK session IDs are identical, and both use the `.claude/projects` path already included in the snapshot.

The driver also recovers recognized missing/corrupt native state by cleaning up the failed adapter and starting exactly one fresh process. That process uses the existing bounded transcript/task continuity ladder. Authentication, routing, unknown internal failures, cancellation, and timeouts remain fatal. Diagnostics use existing credential redaction. Review additionally fixed process-group cleanup when an exited adapter leaves a descendant holding ACP pipes open.

### Repair validation before production rollout

All 117 driver tests and the TypeScript build passed. New regression tests exercise missing/corrupt/localized state errors, the observed query-closed error, exactly one replacement process, transcript re-priming without replay duplication, redacted diagnostics, authentication/routing/unknown-error rejection, cancellation/timeouts, replacement failure, and inherited-stdio descendant cleanup. Profile tests verify eager flushing reaches the child environment and cannot be disabled by runtime overrides. Dashboard agent-session tests passed 242 tests in 22 files. Backend profile/sandbox tests passed; full backend unit suite was rerun after the manifest update (real PostgreSQL coverage remains the successful fresh-container m77 run in this session).

The three simplify reviews found no reuse or quality changes needed; the efficiency finding about orphaned descendant pipes was fixed and re-reviewed. Final local image `bex-agent-m84-final` built successfully with digest `a9794aa1ecdb726f544ff2cd7f06ba34c0118b9459f26a2bfc3d48fc38ade266`. The milestone remains open until the repaired images pass production restoration checks.


### Repair shipment and legacy snapshot

Repair shipped as `522d8f9bbc85f3da91cc6d2987dbe56da4edf56f`; production workflow `34001608697`. Both platform and agent image must advance together because the backend and driver enforce the same release profile environment.

Before rollout, the original-image session completed turn 5 at 00:34:24 UTC, again answering MARIGOLD-742 and 37 with `transcript-reprime`, then hibernated at 00:35:10 UTC (5,938-byte snapshot). This preserves a real pre-repair snapshot for the repaired loader's first live test.

Deployment prerequisite note: run `33999536851` was superseded and skipped image rollout; all tests and CVE gates passed and final credential scrub completed. After it remained in post-scan cache work, cancellation was requested to release the production queue. The repair run retains every normal test/build/security/rollout gate.


A pre-rollout native-history audit found an additional replay bug: the pinned adapter's `loadSession` invokes `replaySessionHistory`, which emits prior assistant/tool notifications before loading returns. The driver forwarded those through the new turn's mapper, duplicating durable/displayed history. Current-turn output must exclude setup/load notifications, including a partial load replay followed by fallback. Run `34001608697` was canceled during its test jobs before any image build or production mutation so the complete fix can follow all gates together.


The replay guard passed all 119 driver tests and build. Two new regression cases cover successful native loading and historical assistant/tool replay followed by recoverable load failure. Both assert that historical bytes are absent from the hub, POST mirror, and new-turn JSONL, while current prompt text/tools remain present. Setup command metadata remains supported. The flag remains active through failed-attempt cleanup to exclude late historical notifications. Simplify quality/efficiency reviews found no further changes needed. Final local image after this guard: `0c9f2ab7fd3da836ae7efe3328f19f787c669f9e458e9811b2fd4fd6c91d7679`.


The final replay repair shipped as `04e8fa215cf56f2fd3073cd04f7d911097487b91`. Its production workflow `34002482856` passed secret scanning and all four suites (backend with CI integration services, dashboard, operator, OpenSandbox controller) before starting image builds. Final reuse review also accepted the dedicated loading phase and existing fixture/helper reuse.


### Repaired production restoration (2026-09-06 UTC)

Workflow `34002482856` completed successfully. GitOps write-back `ee13635e69492f3bc032d5160e3a9b0ec63fe852` pins source `04e8fa215cf56f2fd3073cd04f7d911097487b91`. API image `d9be56f5a148470b061795b0aea6e31cc1957e114871ebbaf61d5d3f5102aa62` and agent image `b9dbcaa578cf9004d13baec16a223767c006c7512b392aa46280b7c48907199d` were verified, including full API rollout and the scratch pod's actual image.

- Legacy snapshot → turn 6, sandbox `2aa6981e-e627-45f4-a10e-818e7553e972`, completed 01:41:02 UTC. Native loading produced the same query-closed error; the redacted runtime diagnostic recorded fresh-process recovery. Annotation `transcript-reprime` at 01:40:56.684 UTC; answer exactly MARIGOLD-742 and 37. Nine parts, complete, not truncated. The repaired turn wrote native transcript files and hibernated at 01:41:48 UTC (9,452-byte snapshot).
- Repaired snapshot → turn 7, sandbox `ec41c9c8-8fb7-4bd9-b3bc-9c81cca83da0`, completed 01:42:47 UTC. Annotation `session-load` at 01:42:45.356 UTC; answer exactly MARIGOLD-742 and 37. Nine parts, complete, not truncated; native history did not duplicate into the new turn. Local evidence: `.playwright-mcp/m84-fallback-result.json`, `.playwright-mcp/m84-native-result.json`.

The native hint capture revealed a separate replay presentation defect: the page shows turns 1–3 and stops at turn 4's historical error, despite all seven turns existing durably. A reproduction with the installed AI SDK confirmed raw error chunks abort message assembly. Earlier-turn errors must become nonfatal conversation entries while current errors remain fatal. Backend/dashboard/mobile parity is being repaired; t003/t004/t005 are reopened and the milestone remains live.


### Historical-error replay repair validation

Earlier-turn errors are projected only while serving replay as `data-bex-turn-error` with the historical turn and original redacted error text. Stored transcript bytes and quota accounting are unchanged; current-turn and transport errors remain fatal. The dashboard renders a bounded, localized failure entry without mislabeling it as an incomplete transcript. Mobile has no transcript/ACP stream consumer; its current-failure overview remains unchanged and all 342 unit tests passed.

Gateway real HTTP/SSE regression verifies historical error → later continuity → later answer ordering, terminal-error behavior, original-byte quota, and no replay persistence mutation; the original implementation fails it. The whole gateway package passed the race detector. The full backend unit suite passed. All 2,997 dashboard tests in 397 files passed, including an actual AI SDK/useChat transport regression that reproduces the failed-turn-to-later-answer path. Simplify consolidated structural filtering and historical error projection into one JSON decode; reuse, quality, and efficiency reviews found no further cleanups.
