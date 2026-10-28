# w8 · m141 — HTTP-aware Docker web listener selection

**Worker:** worker1 **Goal:** Render's multi-port RabbitMQ example serves its HTTP management interface without PORT. **Status:** blocked (t001–t006 done; t007 hosted acceptance and t008 closeout pending deployment)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Select a bounded HTTP listener from candidate ports — **DONE** | 35m | — |
| t002 | Gate candidate probes, rollout and digest reuse — **DONE** | 45m | t001 |
| t003 | Persist detected-port tenant narration — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t005 |
| t007 | Hosted acceptance | 30m | t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- A web-image candidate without explicit port/PORT selects the lowest port returning an HTTP status line, including401, before promotion; private and explicit port choices remain unchanged.
- Pending discovery cannot promote a binary TCP listener or drain the prior release; startup scan is bounded and noHTTP uses the recorded fallback.
- NoEXPOSE listeners can be discovered without advertising invented10000; fallback diagnostics truthfully name an empty EXPOSE set.
- A completed decision is reused per digest, including later releases; legacy already-serving choices are retained on operator upgrade.
- Exact successful deploy history contains Detected HTTP on port N narration, and old deploys cannot borrow another image's choice.
- Hosted RabbitMQ serves15672, nginx serves80 and the noEXPOSE control works; fixtures are deleted by ID with404 proofs. Relevant suites and simplify pass.

## Source + Goal linkage

- **Source:** w8/205, QA sweep5dba2026-10-26; w8/202 noEXPOSE diagnostics follow-up. Promoted2026-10-28.
- **Goal linkage:** Render parity first (ADR008); official Render Docker templates must expose the HTTP interface they promise.
- **Expected outcome:** RabbitMQ's public URL reaches management15672 while AMQP stays private, instead of reporting Live on epmd4369 and returning502.
- **Why now:** image-metadata selection currently applies the private-service rule to public web traffic and the TCP health check falsely passes.
- **Design:** direct bounded HTTP status-line probes after candidate startup, provisional HTTP probes and a150-second replacement hold, per-digest durable selection, existing serving choice retained, fallback after a90-second startup discovery window. The lowest HTTP tie-break is bex policy; Render documents the RabbitMQ outcome but not its algorithm.
- **Render parity:** included for shared REST/GraphQL/MCP/dashboard port projection and tenant deploy-log behavior.

## Verification checkpoint — 2026-10-28

Full operator and types suites, protocol race/socket tests and targeted operator/store/log/API regressions pass. Simplify completed all three reviews, shared the selection/reason constants, reused probe helpers, derived the pending rollout hold and retired completed scan throttle entries. A final operator rerun covers refreshed prior-release availability evidence. Workspace lint/dead-code and fresh-dependency backend ship gates must pass before push. Hosted acceptance remains pending the new operator/API/CRD deployment.

## Parked gate — 2026-10-28

The final full operator suite, types suite, workspace lint/dead-code and fresh-dependency backend ship gate all passed, including migration0172 and exact-release historical port persistence. **BLOCKED:** release pipeline deploys the operator/API/CRD and migration, then QA runs t007: fresh RabbitMQ noPORT reaches management15672 with its historical Detected HTTP narration, nginx remains80, and a noEXPOSE image selects its runtime HTTP port. Confirm explicitPORT/private controls remain unchanged and delete all fixtures by ID with404 proofs. No production fixture was created before the new build was available.
