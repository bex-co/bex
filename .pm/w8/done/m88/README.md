# w8 · m88 — Preserve graceful shutdown through native npm startup

**Worker:** worker8 **Goal:** a normal npm-started Node app receives termination and completes its in-flight response during the configured shutdown delay. **Status:** done

**Severity:** major, fixed. **Estimate:**250m across8 tasks. Source implementation af4f7fefb and its completed test/audit work are accepted by [the original hosted both-client replay](evidence/cli-sweep411/README.md).

## Verified outcome —2026-10-17

Both fresh native clients keep the exact original public source, written program, npm startup, environment and shutdown45. Both native restarts0 and replacement children Live; both original40s POSTs finish200 with the old boot/token, all44 short reads200. Native logs prove SIGTERM with the token active, then hold_end and closed. Six shared metadata reads agree. The helper was first observed actually present/running as PID1 in sweep410. Both replay parents/four children/URLs and owned local resources are absent, original baselines remain and concurrent quota is reconciled.

The original386 failures, direct-only counterfactual and local fail-before/pass-after evidence remain in [the original finding](finding.md) and [original evidence](evidence/cli-sweep386/README.md). This closeout performs hosted QA and board updates; no new product implementation or unrun paid/browser/OCI/physical claim. Evidence/status moves are uncommitted.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Implement termination forwarding and descendant lifetime — **DONE** | 45m | — |
| [t002](done/t002.md) | Deliver the helper in primary Node serving images — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Audit launch callers aliases types and existing boundaries — **DONE** | 40m | t002 |
| [t004](done/t004.md) | Replay both original native npm restart journeys — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

Fresh original explicitly Free Node24 fixtures, unchanged source/program/npm start and shutdown45, through published Bex and unchanged-pin Render: one bounded40s POST is active before actual root restart; both native restarts exit0, replacement children reach Live, and the original POST finishes200 with its old boot/token. App logs show SIGTERM with the token active, then hold_end and closed; all bounded short reads200. Complete six shared command/source/shutdown readbacks and exact-owned parent/child/URL cleanup preserve baselines/zero termination. The actual fixed serving launch is exercised; a direct-Node workaround or the local prototype is not acceptance.

The maintained real generated-image and process tests prove signal forwarding plus terminating-descendant waiting, normal startup/exit/error/status/output/cwd/PATH/env behavior and launch/exit races; audit every counted launch consumer and native line/package-manager/exclusion neighbor. Required suites/lint pass. No unrun paid/browser/OCI/physical claim. Only after all clauses hold, move tasks and milestone to done.

## Source + Goal linkage

- **Source:** authorized ordinary functional CLI QA sweep386. [Finding](finding.md) and [complete durable evidence](evidence/cli-sweep386/README.md) retain both independent HTTP502 reproductions, the passing only-start-command direct-Node control, local forward/wait control, source/contract/dedupe and cleanup.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) hosting reliability, [ADR004](../../../../docs/ADR004-app-deployment.md) graceful instance replacement and [ADR018](../../../../docs/ADR018-render-parity.md) native runtime parity.
- **Expected outcome:** npm command trees preserve the app's opportunity to complete work on termination; configured shutdown delay has the intended effect.
- **Why now:** both native clients return restart0/new Live while the active response becomes502 before its40s completion, despite configured45s. Short requests all pass. The same code handles termination and finishes200 under direct Node; the process-tree lifetime is isolated rather than speculative.
- **Boundaries/dedupe:** [w1/m154](../../../w1/done/m154/README.md) already owns the10s routing drain and [w8/m61](../m61/README.md) owns PATH/base-entrypoint behavior. Preserve both. [Node111](../../blocked/111.md) remains runtime-only production default. The original filing found no duplicate descendant-lifetime repair; the implemented supplier and this hosted replay now satisfy the original gate.
- **Standing closing tasks:** Render parity, simplify, test coverage and closeout are included because startup/restart/shutdown affect tenant-visible runtime behavior. REST/GraphQL/MCP/Blueprint/dashboard keep their existing inputs; no new product control.

## Node26 Yarn-start neighbor — sweep437 (2026-10-17)

[Fresh both-client Node26/Yarn restart evidence](../../evidence/native-node26-yarn-shutdown-sweep437/README.md) passes two first Free deployments, two native restarts, both original40s POSTs completed200 with their old boot/token, bounded short reads200 and SIGTERM-active → hold_end → closed native events. The original app/public files/shutdown45 are preserved; the variant selects Node26 and `yarn --cwd qa386 start`. Six shared source/command/shutdown reads and two exact env reads match. Exact parents/four children/former URLs and local resources are removed, baseline/current quotas reconcile. This adds a hosted wrapper/line neighbor to the accepted npm24 case; no new bug or product implementation, other wrappers/lines are not inferred. Status stays done. Evidence uncommitted.

## Rust Cargo process and HTTP drain — sweep446

[Fresh Rust/Cargo hosted neighbor](../../evidence/native-rust-cargo-shutdown-sweep446/README.md) passes both native first Free services and one restart each with45s shutdown. Both old40s requests finish200 with exact UTF8 bytes on their original boots; native SIGTERM-active → hold-end → closed events and new Live boots agree. Pre/post source/commands/empty env and REST/MCP delay readbacks pass. Local same-byte Cargo/direct controls also drain40s under the checkout helper. All own parents/four children/URLs and local artifacts are removed. This extends process-wrapper coverage while m88 stays done; it does not claim the original Node program was used for this Rust variant.
