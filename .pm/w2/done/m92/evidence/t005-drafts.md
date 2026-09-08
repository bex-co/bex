# DRAFT — apply only after Phase 3 completes (t005)

Do not commit these edits to ADR055 / runbook / inbox until phases 1–3 are executed and verified. Kept here so t005 is a fill-in-the-blank once the window starts.

## ADR055 F2/F3 table cell (replace “Deferred — migration-gated” update clause)

**Update (w2/m92, `<DATE>`):** phases 1–3 executed `<DATE>`; phase 4 armed — 14-day clean window ends `<DATE+14d>`. Production identities are workspace-scoped; dual-read + legacy blobs remain until phase 4. F2/F3 **not** fully closed.

## Runbook Phase 4 header addition

**Clean window started:** `<DATE>` (w2/m92 phase 3 complete). **Do not start Phase 4 before:** `<DATE+14d>`, and only with zero dual-read hits + explicit STOP authorization.

## Inbox note skeleton (rescan number before filing; was `035` at materialization)

```markdown
# Phase 4: drop dual-read + delete legacy registry/static blobs (ADR055 F2/F3 close)

**STOP + window-gated.** Do not promote/execute until the 14-day clean window from w2/m92 phase 3 ends (`<DATE+14d>`) and an explicit change-window authorization is recorded.

## Scope
- Disable/remove `BEX_REGISTRY_DUAL_READ` migration window (operator).
- Delete legacy Zot repos `A` and S3 prefixes `A/` (including `.bex-tombstone`) for tombstoned Apps only.
- Confirm no unlabeled sibling still owns `A`.
- Annotate ADR055 F2/F3 fully closed.

## Risk
Destructive; rollback = restore from registry/S3 backup. No in-place un-delete.

## Source
w2/m92 t005 arming; runbook `docs/runbooks/registry-static-identity-migration.md` § Phase 4.
```
