# w8 · m75 — Validate Blueprints with an implicit repository

**Worker:** worker8 **Goal:** the unchanged CLI can validate a normal Git-backed Blueprint with no literal repo while actual creation still requires its resolved source. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Validate implicit Git source with an explicit read context | 55m | — |
| t002 | Audit shared source projections and planner behavior | 30m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Meaningful regression coverage | 45m | t003 |
| t006 | Live closeout | 20m | t004, t005 |

## Definition of done

- The nine qualified implicit-repository documents (six minimal root/nested runtime cases, unchanged Ghost/WordPress,10GB Mattermost variant) pass full standalone validation and deployed published-Bex/unchanged-Render checks. Explicit repository/prebuilt controls pass, malformed repo and missing image remain refused.
- The original1GB Mattermost example remains subject to Bex's documented disk floor; it is excluded from a blanket valid:true promise. The source-context error must no longer mask that legitimate refusal. All36 original per-kind native controls remain correct.
- Known-parent repository preview/sync, existing-resource plan/source switches, actual imperative/stack source requirements and zero-write failures stay correct. Shared quota/domain/create-plan/aggregation/default/presence checks remain enforced where their inputs are available, and no fabricated repository URL appears in public plans or persistence.
- REST/GraphQL/MCP/dashboard and native HTTP/exit/error behavior agree. Meaningful regressions, appropriate backend checks, lint and simplify pass; actual live/unknown-context limits are explicit.
- No fixture is allocated by the validation replay; all future owned inputs/configs/processes are cleaned, baseline/quota preserved. Complete/move tasks/milestone only when every observable gate holds; shipping code alone is insufficient.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep318,2026-10-15 UTC; [finding](finding.md), [123 actual native calls and84 qualified comparisons](evidence/cli-sweep318/README.md).
- **Goal linkage:** ADR008 dependable hosting, ADR018 unchanged CLI/Blueprint compatibility and ADR049 truthful structured preflight checks.
- **Expected outcome:** omitted enclosing Git repository is handled as unavailable read context instead of a missing imperative-create field; actual creation keeps its source requirement.
- **Why now:** Ghost/WordPress and six minimal implicit-repo files are rejected live and on current source; adding a known repository passes their full checks, while explicit/malformed/prebuilt controls isolate the missing-context branch. The per-kind blocker is now fixed, exposing the next independent refusal in unchanged Render files.
- **Sizing:**180m/six tasks; one contextual projection repair plus shared current-state/create-plan/surface verification exceeds a sub-hour note.
- **Render parity included:** existing public validation and dashboard consumers change behind unchanged CLI multipart input. Keep Bex's explicit service-volume floor; authenticated Render hosting and actual deployed revision are unverified.

This is scheduled work. Current diagnostic failures are real for the qualified cases and are not a fixed implementation. No product edit, new ship, release, platform deployment or external submission occurred in QA.
