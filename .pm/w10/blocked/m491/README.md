# w10 · m491 — An agent session's unpushed work survives into the next message however its turn ended, as on Devin

**Worker:** worker10 **Goal:** a turn that ends with uncommitted changes or unpushed commits, whether it completed, failed or sat idle until its sandbox was closed, leaves that work in the checkout of the session's next turn for a bounded time, and every turn whose work was kept or lost says so to the user and to the next turn's agent. **Status:** blocked (t001–t010 done 2026-11-18; t011, the live check, waits for the shared cluster's agent-session leg, held by `dev-9`; see "Gate: t011")

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: every turn end with undelivered work keeps its disk; ADR047 D12 and ADR018 widened — **DONE** | 30m | — |
| [t002](done/t002.md) | Facts: a failed turn reports its checkout's Git facts too, and the turn records them — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | A failed turn with undelivered work keeps its disk instead of being torn down at once — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | The next message keeps a completed turn's undelivered work: save before the previous sandbox is replaced — **DONE** | 2h | t002 |
| [t005](done/t005.md) | The idle close keeps a completed turn's undelivered work: save instead of terminate — **DONE** | 1h15m | t004 |
| [t006](done/t006.md) | Say what was kept or lost: REST, GraphQL, MCP, and the next turn's agent — **DONE** | 1h30m | t003, t005 |
| [t007](done/t007.md) | Dashboard and `bex agent` show it on every turn that kept or lost work — **DONE** | 1h15m | t006 |
| [t008](done/t008.md) | Render parity (Devin) — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 20m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008, t009 |
| [t011](t011.md) | Live check on `dev-10`: a failed, a followed-up and an idle-closed turn keep their work | 1h30m | t010 |
| [t012](t012.md) | Closeout | 10m | t011 |

Total: **12 tasks, about 12h30m**.

## Definition of done

1. A turn that **completes** with an uncommitted file and an unpushed commit, followed by a message within the idle grace, runs that message in a sandbox whose checkout still has both. A real-Postgres test with the sandbox fake fails on today's code, where `runSteerDispatch` tears the previous sandbox down and boots a fresh clone.
2. The same turn left alone past the idle grace (`BEX_AGENT_SANDBOX_IDLE_TTL`) keeps its disk when the reaper closes the sandbox, and a later message continues from it while it is kept. A clean checkout is closed exactly as today, with nothing saved.
3. A turn that **fails** with undelivered work (a driver error, the no-progress watchdog, the turn ceiling, a missing structured output) keeps its disk the same way instead of being torn down at once. A sandbox already gone (`failLostSandbox`) records the loss and the reason.
4. Every turn whose undelivered work was kept, continued from or lost says so on REST, GraphQL, MCP, the dashboard and `bex agent`, with the time it is kept until or the reason it was lost, and what was at stake (the number of unpushed commits and whether there were uncommitted changes). The next turn's agent is told in its prompt whether it starts from the restored checkout or from a fresh one that lost that work.
5. m413's rules hold for every kept disk unchanged: counted against the workspace storage cap, kept for `BEX_AGENT_INTERRUPT_RETENTION` (or its renamed successor), deleted on expiry, cancel, delete, archive, workspace delete and after a continuation; an open editor, a full cap or a failed save degrades to today's fresh start with the loss shown.
6. ADR090 is unchanged: neither the driver nor bex-api commits or pushes anything. ADR047 D12 and ADR018 record the widened design and its Devin basis.
7. Live on `dev-10` with the `agent-stub` test double and real sandboxes (m413's recipe): the three cases in items 1–3 each show the uncommitted file and the unpushed commit in the next turn's sandbox, and a shortened retention shows the loss. The production `isolation-matrix.yml -f verify_agent_retention=true` probe still passes.

## Source + Goal linkage

- **Source:** inbox note [w10/013](../../done/013.md), filed 2026-11-18 from the agent-automation runs of 2026-11-15..18 (sessions `ags-dbu0pfdikg6s738lam7g`, `ags-dbu0pfmc1l2s739d3n3g`, `ags-dbutgej906ns73ff3kb0`), promoted by `/loopx w10` on 2026-11-18. It also covers the last clause of [w10/016](../../016.md) ("before failing a turn on the watchdog, snapshot unpushed work"); 016's own fix (counting a running tool's output as progress) stays there.
- **Triage evidence (main at `fd9787d29`, 2026-11-18):**
  - Retention is gated on a Stop: `settleCompleted` keeps a disk only when `outcome == turnOutcomeInterrupted` (`lego/backend/internal/agentsessions/completion.go:756`), and `turnRetention` answers `none` for any turn that is not `Interrupted` (`turn_retention.go:85`).
  - The loss is wider than the note says. Every follow-up message tears the previous sandbox down and boots a fresh one (`runSteerDispatch`, `service.go:2166-2190`; ADR047 D8, a new prompt cannot ride the old sandbox), so a completed turn's unpushed work is gone at the **next message**, not only after the idle close. `dispatchImage` boots the retained image, else the repository environment's image, else the template: none holds the old checkout.
  - A failed turn skips the idle grace and is torn down at its settle (`teardown`, `skipGrace := … || record.Phase == PhaseFailed`, `completion.go:997`).
  - A completed turn's idle close calls `terminate` with no save (`completion.go:1009`).
  - The driver already measures what is at stake on a successful or stopped turn (`GitFacts.dirty`, `unpushedCommits`, `lego/agent-image/driver/src/delivery-check.ts`), but its failed status carries no `git` object (`session.ts:513-517`, and the setup failure at `:124`), so a failed turn's stake is unknown today.
- **Reference product (Devin):** a session's machine is kept while it sleeps and resumes with its state for 14 days ([Devin release notes](https://docs.devin.ai/release-notes/2026), 2026-09-25, 2026-10-05, 2026-10-09), whatever ended the last exchange; a follow-up never discards the machine. w22/m413 cites the same basis.
- **Design direction (t001 settles it):** reuse m413's retained-image mechanism (`retention.go`, `completion_retention.go`, `agent_session_retained_images`) and widen when it runs: at a failed turn's settle, at a follow-up before the previous sandbox is torn down, and at the idle close. Save only when the checkout has undelivered work (a clean tree skips the pause, closing one of m413's follow-ups for these paths). A push to a recovery ref, the note's other option, would make bex-api push work the agent did not deliver and conflicts with ADR090's custody, so it is not taken.
- **Goal linkage:** ADR008 pillar 5 (cloud coding agents, [ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)); unattended automations are the product's main use, and a run that loses its work delivers nothing the user paid for.
- **Expected outcome:** an agent whose push was refused ([w10/014](../../014.md)), whose turn failed or which was simply followed up finds its commits and edits where it left them, and a user whose work could not be kept is told so instead of finding a clean clone.
- **Why now:** three production sessions lost work in two days, one a verified fix; the w10/014 push refusal makes it recur on every session pinned to an older commit.
- **Render parity included (reference product: Devin):** the turn's retention fields change on REST, GraphQL and MCP, and the dashboard and `bex agent` show them.

## Progress 2026-11-18 (/loopx w10)

t001–t010 shipped together with this board move (one commit). What is true now, against the definition of done:

- **Items 1–3 (the three triggers), as code and tests.** A completed turn with an unpushed commit and an uncommitted file, followed by a message, runs that message in a sandbox booted from its disk (`TestAMessageAfterACompletedTurnContinuesFromItsUndeliveredWork`, real Postgres with the sandbox fake); the idle close saves instead of terminating, a clean checkout is closed as before and an archived one at once (`TestTheIdleCloseKeepsACompletedTurnsUndeliveredWork`); a failed turn with undelivered work keeps its disk and still reads failed with its reason, a clean one is torn down at once, Cancel releases the image (`TestAFailedTurnKeepsItsUndeliveredWork`); a lost sandbox records `lost`, `sandbox_lost` (`TestALostSandboxRecordsTheLoss`). Each of the five trigger tests fails with its hook reverted (checked 2026-11-18). The driver writes `git` facts on a failed status (`a failed repo turn records its checkout's facts`, which fails without the fix).
- **Item 4 (surfaces).** REST, GraphQL and MCP carry `retentionCause`, `unpushedCommits`, `uncommittedChanges` and the new `sandbox_lost`, from one `turnRetention` and shared descriptions; the next turn's start note and the agent's `<checkout-state>` preamble (`BEX_AGENT_CHECKOUT_NOTE`) say where the checkout comes from; the dashboard (all eight locales) and `bex agent logs`/`get` show it on every turn that kept or lost work.
- **Items 5–6.** m413's handle, claim, cap, expiry and releases are reused unchanged; a full cap or failed save on a close ends the sandbox and records the loss; an editor open on a replaced sandbox records `editor_open`. ADR090 is unchanged. ADR047 D12 ("As built (w10/m491)") and ADR018 record the design and its Devin basis.
- **Deviations, stated.** The facts are recorded in their own write just before the settle, not inside the finalize transaction (no reader that acts on them can see the settled turn without them). An unknown checkout is read in the sandbox for every trigger, a failed turn's included (t003 said unknown keeps the teardown), because a session with no repository never reports facts. `BEX_AGENT_INTERRUPT_RETENTION` keeps its name. `retentionCause` is on the API; the dashboard and the CLI word the line by state, reason and stake, not by cause. `scripts/dev-env.sh` gained `DEV_AGENT_SANDBOX_IDLE_TTL` and `DEV_AGENT_TURN_TIMEOUT` for t011.
- **Suites run:** `lego/backend` `go test ./...` against real Postgres, OpenFGA, OpenBao and Loki; `make lint` (four modules); the driver's `npm test`; `cd lego/cli && go test ./...`; dashboard `yarn lint` and `yarn test`.

## Gate: t011 (the live check) — BLOCKED 2026-11-18

**Needs:** the shared kind/CAPD cluster's agent-session leg, which is cluster-wide and exclusive, released by its holder: `bash scripts/dev-env.sh 9 agent-down` from w9's checkout (or the user's say-so that w10 may take it over). On 2026-11-18 the `opensandbox-system` namespace carries `app.bex.co/dev-env=dev-9` (created 19:26 UTC, its server pod restarted 98 minutes before the check, so it is live, not stale), and `agent-up` for `dev-10` refused with "the agent-session leg is currently held by dev-9". A worker is read-only on another workstream's stack, so this worker did not release it. Then the production probe of step 6 needs a deploy carrying this commit.

**Already done for it:** `bash scripts/mock-cluster.sh` recovered the cluster (this checkout's kubeconfig had gone stale; every check passed, nothing reprovisioned), `mock-cluster.sh operator` and `dev-env.sh 10 up` came up, and `dev-10` was taken down again (no agent leg, nothing to run). No sandbox was created.

**Next worker, in order:** confirm the leg is free (`kubectl get ns opensandbox-system -o jsonpath='{.metadata.labels.app\.bex\.co/dev-env}'` empty or `dev-10`); `bash scripts/dev-env.sh 10 up`, then `DEV_AGENT_SANDBOX_IDLE_TTL=2m DEV_AGENT_TURN_TIMEOUT=3m bash scripts/dev-env.sh 10 agent-up` and `agent-stub`; run t011 steps 2–5 (the failed case is `stub-run:` then `stub-hold:600`, which the 3-minute ceiling fails); step 6 once production runs this commit; `dev-env.sh 10 down`; then t012.

