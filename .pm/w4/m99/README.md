# w4 · m99 — Restore image-create compatibility and long-name alias admission

**Worker:** worker4 **Goal:** Valid CLI image creates accept an explicitly disabled setting, and long service names reconcile their required platform aliases so healthy first deploys reach live. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Accept explicitly disabled auto-deploy on prebuilt image creates | 35m | — |
| t002 | Admit bounded platform alias names without weakening target ownership | 45m | — |
| t003 | Verify both shared validation and alias families | 45m | t001, t002 |
| t004 | Render parity across create and deployment surfaces | 20m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test the CLI failures and admission boundary | 35m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Repeat t001's exact CLI create with a fresh owned free fixture and `--auto-deploy=false`: exit 0, disabled no/off state, and no unsupported Git automation. Omitted flag still succeeds.
- Repeat t002's long- and short-name image-service creates in a production-equivalent tenant namespace: both initial deploys reach `live`, both HTTPS endpoints serve 200, and legitimate bounded aliases pass admission with fixed targets and exact App ownership.
- Meaningful enabled/invalid-value and foreign-owner/actor/target admission controls preserve rejection. Shared callers have behavioral regression coverage.
- Delete each recorded fixture ID and verify detail/list/instances absence; no pre-existing resources are changed.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs-cli` requested for w4, 2026-09-09 PDT / 2026-09-10 UTC. Durable sanitized commands, wire bodies, output and attribution are in t001/t002.
- **Goal linkage:** Render-compatible hosting (ADR006/ADR018), tenant namespace isolation (ADR043), truthful image/Blueprint configuration (ADR049), and working free-tier wake routing (w6/m47).
- **Expected outcome:** no false rejection of explicit disabled image auto-deploy; no failed initial deployment caused by the operator's own long-name alias format.
- **Why now:** the shipped v2.27.0 CLI makes both journeys reproducible against production; current main still contains both causes. This schedules fixes, not completed implementation.
- **Render parity included:** both successful create semantics and deployment state are tenant-visible. Shared-code blast-radius work is separate t003.

## Hunt cleanup and limits

The long runtime fixture and short live control were deleted with the installed CLI; detail and instance lookups returned 404. The initial port-10000 fixture was also deleted; it is not positive runtime evidence. Final CLI list reconciliation returned exactly the original 10 IDs: zero baseline resources missing and zero owned fixtures remaining. No paid resources, workspace settings or pre-existing resources were changed. Platform revision was not observable; platform fixes and post-fix acceptance remain unverified.
