# w8 · m64 — Keep Blueprint environment copies consistent across applies

**Worker:** worker8 **Goal:** sibling copies equal the configuration their source will receive, across group links and seed-once edits/reapply. **Status:** blocked — t001–t005 done and shipped (2026-10-13); t006 needs the API deployed to production plus fresh pinned native validation captures and hosted/snapshot dispositions.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve known values from effective link and seed state — **DONE 2026-10-13** | 55m | — |
| t002 | Audit all parser/current-state consumers and inherited 099 DoD — **DONE 2026-10-13** | 35m | t001 |
| t003 | Render parity — **DONE 2026-10-13** | 15m | t002 |
| t004 | Simplify — **DONE 2026-10-13** | 10m | t003 |
| t005 | Meaningful test coverage — **DONE 2026-10-13** | 30m | t003 |
| t006 | Closeout — **BLOCKED: deployed API + native receipts** | 10m | t004, t005 |

## Definition of done

- All seven [actual group diagnostic cases](evidence/cli-sweep213/actual-backend-results.json) have matching source/copy configuration, or a located refusal before any write where the value is not known; current-state, first/reapply, duplicate, retained/omitted/new links and service-own literal/empty controls are covered with real feature services.
- All five [actual seed/edit/reapply cases](evidence/seed-copies-sweep214/actual-backend-results.json) have truthful source/copy values or a located preflight refusal when unknown. Seed-once keeps edits; first-create seed hints and immutable own literal/empty controls keep correct behavior. Audit prompt and mixed seed/reference/group neighbors without dynamic-value expansion.
- Preserve the complete original 099 acceptance in [finding.md](finding.md), including its now-passing 16 native linked-value/precedence/refusal checks, masks/external limits and all four parser callers. Verify metadata-backed/public IDs and nested/consumer-first cases; no global link-order change, dynamic-value/reference-chain expansion or CLI fork.
- All shared validation surfaces have consistent eligibility/errors at the authored caller entry. Record exact pinned native wire, actual deployment identity only if observable, hosted/snapshot limitations and any tracked follow-up explicitly; declarations are not hosted apply evidence.
- Meaningful regressions fail on the present wrong index and pass corrected. The diagnostic overlays expect current bugs and must not be copied as fixed expectations. Appropriate checks and simplify pass.
- Every future owned fixture/config/process is cleaned or precisely accounted for. All tasks and the complete observable DoD hold before moving to done; shipping code alone is insufficient.

## Source + Goal linkage

- **Source:** promoted w8/099 and functional CLI sweep213, 2026-10-13 UTC; [researched finding/history](finding.md) and [native/source/operator evidence](evidence/cli-sweep213/README.md).
- **Goal linkage:** ADR008 reliable hosting, ADR049 Blueprint configuration, ADR013's deliberate last-linked precedence and ADR018 unchanged CLI contract.
- **Expected outcome:** an accepted sibling copy cannot silently disagree with its source because the parser guessed a different group order.
- **Why now:** the linked-literal fix 7a06a13f4 is live, but four actual source models expose silent mismatches, including a fresh duplicate shape admitted by both native clients; the original current-state acceptance gap is a real implementation gap.
- **Sizing:** remaining work is 155m across two implementation/audit tasks and four standing closing tasks, so the prior sub-hour note is promoted. Completed original core work and all original DoD dispositions remain; this is one continued issue.
- **Render parity included:** shared REST/GraphQL/MCP/dashboard validation changes. Keep Bex's documented last-linked divergence; authenticated Render-production precedence was not measured.

## Source extension — sweep214

Three edited-seed configured-value mismatches join the four group-order mismatches under the same known-value index. [Actual feature-method proof and qualified native admission](evidence/seed-copies-sweep214/README.md). Source e2cc474297ba, deployed revision unknown; all local/native copies cleaned, no hosted resource. The milestone remains todo and retains every inherited original 099 obligation. No QA source implementation or ship.
