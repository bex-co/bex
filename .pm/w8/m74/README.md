# w8 · m74 — Preserve Blueprint host-reference origins and target checks

**Worker:** worker8 **Goal:** each host reference is checked against its actual target and every refusal identifies its own authored field before apply writes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve authored host-reference origins | 35m | — |
| t002 | Validate existing host target type and addressability | 40m | t001 |
| t003 | Audit shared consumers and pre-write behavior | 25m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Meaningful regression coverage | 35m | t004 |
| t007 | Live closeout | 20m | t005, t006 |

## Definition of done

- All nine314 origin documents return the correct authored paths/line/column through published Bex and unchanged Render; six previous mislocations are fixed and three controls keep their decisions. Preserve one error per actual reference and original messages/order.
- Fresh own Free static/web targets reproduce all eight316 documents after deployment: four invalid existing-target cases now refuse with their exact origin/type field, four controls retain results, and native/API validation errors agree. Current evidence:28 native calls/14 false accepts and14 controls.
- Shared source tests cover all effective target kinds, reference spellings/defaults/declared precedence, validation/apply/generation/disconnect/sync, repeated keys/targets, DB/KV/pooler and earlier051/081/096/100/126/206 behavior. Invalid existing-target applies return400 before any write. Appropriate module/API checks, lint and simplify pass.
- Existing REST/GraphQL/MCP/dashboard/native error envelopes/locations and valid web/private hosts remain consistent; actual live scope and any unverified external parity are explicit.
- All owned parents/children/configs/processes are cleaned with baseline/quota preserved. Every task and observable acceptance holds before moving tasks/milestone to done; code shipping alone is insufficient.

## Source + Goal linkage

- **Source:** promoted unshipped w8/110 from functional CLI sweep314 ([preserved note](110.md), [31 native receipts](../evidence/110/README.md)) plus sweep316's [existing-target finding](finding-targets.md) and [native/source evidence](evidence/cli-sweep316/README.md),2026-10-15 UTC.
- **Goal linkage:** ADR008 dependable hosting, ADR018 unchanged CLI compatibility, ADR041 internal service addresses and ADR049 actionable Blueprint diagnostics.
- **Expected outcome:** validation and apply reject invalid existing targets before writes, and each error identifies its actual caller occurrence.
- **Why now:**314 loses origin and316 loses supplied/actual target metadata in the same hostRef/unknownServiceReferences path. Separate implementation tasks within one coordinated change avoid parallel incompatible rewrites; original evidence and done-neighbor behavior remain preserved.
- **Sizing:**185m/seven tasks; two distinct bugs plus shared pre-write/surface validation exceed the prior55m inbox scope. Number110 is retained as110.md within this milestone and is not reused.
- **Render parity included:** shared public validation/apply/preview surfaces and dashboard errors change. Existing host-only workspace references are a Bex extension to preserve within its documented addressability/type contract; authenticated Render hosting is unverified.

No product fix, new commit/push, release, platform deploy or external submission is supplied by QA. The source regressions intentionally fail on current behavior; apply diagnostics are observations, not passing fixed acceptance.
