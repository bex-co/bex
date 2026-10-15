# w8 · m67 — Keep native live logs running after a transient disconnect

**Worker:** worker8 **Goal:** the released Bex CLI follows logs through transient connection loss with visible retry status and exact record resumption. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Qualify the released reconnecting tail against Bex lifecycle controls](t001.md) | 45m | — |
| t002 | [Adopt the qualified upstream pin and review its hosting wire delta](t002.md) | 35m | t001 |
| t003 | [Verify affected hosting callers through candidate Bex and upstream CLI](t003.md) | 30m | t002 |
| t004 | [Render parity](t004.md) | 20m | t003 |
| t005 | [Simplify](t005.md) | 10m | t004 |
| t006 | [Test coverage](t006.md) | 20m | t004 |
| t007 | [Closeout](t007.md) | 10m | t005, t006 |

Total: 2h50m, seven tasks. [Researched finding](finding.md), [native fault/recovery evidence](evidence/cli-sweep272/README.md).

## Definition of done

- The exact adopted upstream pin is recorded and Bex's embedded consumer passes the transient-disconnect controls: visible status, resumed records A/B exactly once, continued follow until explicit cancellation.
- Changed supported hosting wire/output contracts, Bex branding/config/device-login seams and completed/no-build tails are qualified. Existing YAML/TUI limitations are honestly graded; unrelated commands retain their supported/partial/non-goal classification.
- Required CLI/pin checks and parity acceptance pass with all own fixtures/configs/processes cleaned.
- After an explicitly requested CLI release, published Bex delivery embeds the qualified fix and native acceptance passes. Candidate-only qualification leaves this delivery gate open; filing does not authorize a release. Current published0.3.2 remains affected until that happens.
- All status/frontmatter/paths are synchronized and the milestone moves to done only after these observable gates hold.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep272. Six current-pin failure cases and six current-pin recovery failures silently exit0; the released upstream2.29 tag build passes three recovery controls. Upstream has already implemented the fix; this is Bex dependency adoption, not another upstream implementation request.
- **Goal linkage:** ADR008 usable hosting and ADR018 accurate Render CLI compatibility; a successful log follow must continue observing a service when an ordinary connection drops.
- **Expected outcome:** Bex's customer CLI visibly retries transient loss, resumes without replay duplicates and exposes terminal failures honestly.
- **Why now:** the current Bex0.3.2/Render2.27 consumer loses the follow silently; upstream2.29.0 was published2026-10-12 and provides a tested fix. Qualify its new retry/normal-close policy before broad pin adoption and release. This spans dependency, shared hosting contracts and delivery, so it is a multi-task milestone.
- **Render parity included:** the imported CLI is the user-facing surface, and its generated contracts plus existing REST/GraphQL/MCP/dashboard semantics need explicit review. No backend defect is asserted by the loopback reproduction.

## Boundaries

Current pin remains2.27/a764810a7682; it is the contract oracle until qualified adoption. The new2.29 comparison is an unmodified tag build, not a Bex candidate or installed release. No production failure was induced. The hunt changed only PM evidence, with no dependency edit, commit/push, external submission or release. See finding.md for source locations, controls, dedupe and unverified areas.
