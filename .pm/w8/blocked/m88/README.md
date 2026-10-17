# w8 · m88 — Preserve graceful shutdown through native npm startup

**Worker:** worker8 **Goal:** a normal npm-started Node app receives termination and completes its in-flight response during the configured shutdown delay. **Status:** blocked — t001–t003, t005–t007 done; t004/t008 need the operator deployed plus the hosted both-client npm restart replay

**Severity:** major. **Estimate:**250m /4h10m across8 tasks. Research/filing only; source fix and hosted acceptance pending.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Implement termination forwarding and descendant lifetime | 45m | — — **DONE 2026-10-16** |
| [t002](done/t002.md) | Deliver the helper in primary Node serving images | 45m | t001 — **DONE 2026-10-16** |
| [t003](done/t003.md) | Audit launch callers aliases types and existing boundaries | 40m | t002 — **DONE 2026-10-16** |
| t004 | Replay both original native npm restart journeys | 30m | t003 |
| [t005](done/t005.md) | Render parity | 20m | t004 — **DONE 2026-10-16** |
| [t006](done/t006.md) | Simplify | 15m | t005 — **DONE 2026-10-16** |
| [t007](done/t007.md) | Test coverage | 45m | t005, t006 — **DONE 2026-10-16** |
| t008 | Closeout | 10m | t007 |

## Definition of done

Fresh original explicitly Free Node24 fixtures, unchanged source/program/npm start and shutdown45, through published Bex and unchanged-pin Render: one bounded40s POST is active before actual root restart; both native restarts exit0, replacement children reach Live, and the original POST finishes200 with its old boot/token. App logs show SIGTERM with the token active, then hold_end and closed; all bounded short reads200. Complete six shared command/source/shutdown readbacks and exact-owned parent/child/URL cleanup preserve baselines/zero termination. The actual fixed serving launch is exercised; a direct-Node workaround or the local prototype is not acceptance.

The maintained real generated-image and process tests prove signal forwarding plus terminating-descendant waiting, normal startup/exit/error/status/output/cwd/PATH/env behavior and launch/exit races; audit every counted launch consumer and native line/package-manager/exclusion neighbor. Required suites/lint pass. No unrun paid/browser/OCI/physical claim. Only after all clauses hold, move tasks and milestone to done.

## Source + Goal linkage

- **Source:** authorized ordinary functional CLI QA sweep386. [Finding](finding.md) and [complete durable evidence](evidence/cli-sweep386/README.md) retain both independent HTTP502 reproductions, the passing only-start-command direct-Node control, local forward/wait control, source/contract/dedupe and cleanup.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) hosting reliability, [ADR004](../../../../docs/ADR004-app-deployment.md) graceful instance replacement and [ADR018](../../../../docs/ADR018-render-parity.md) native runtime parity.
- **Expected outcome:** npm command trees preserve the app's opportunity to complete work on termination; configured shutdown delay has the intended effect.
- **Why now:** both native clients return restart0/new Live while the active response becomes502 before its40s completion, despite configured45s. Short requests all pass. The same code handles termination and finishes200 under direct Node; the process-tree lifetime is isolated rather than speculative.
- **Boundaries/dedupe:** [w1/m154](../../../w1/done/m154/README.md) already owns the10s routing drain and [w8/m61](../../done/m61/README.md) owns PATH/base-entrypoint behavior. Preserve both. [Node111](../111.md) remains runtime-only production default. No duplicate npm descendant-lifetime fix or newer relevant source is found.
- **Standing closing tasks:** Render parity, simplify, test coverage and closeout are included because startup/restart/shutdown affect tenant-visible runtime behavior. REST/GraphQL/MCP/Blueprint/dashboard keep their existing inputs; no new product control.

## Research outcome

Two original npm restarts fail HTTP502 after26.186s/24.700s with19/17 short reads200. The direct-only start control finishes the same response200 after41.532s and20 short reads200; handler/active token/end/closed are retained. The local pinned-base direct/npm/supervised-npm cases pass/fail/pass. Two parents/six children/two URLs are absent, baselines preserved and termination0. Native restart wire, exact live OCI/ancestor tree, fixed generated artifact, broader hosted variants and browser remain unverified. One read-only wrong nested command is fully excluded/reconciled in the evidence. Uncommitted scheduling; no implementation, paid operation or shared rollout.
