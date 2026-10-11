# w4 · m195 — Save opaque renames into explicitly deleted names

**Worker:** worker4 **Goal:** save a valid deleted-destination rename in one staged transaction without revealing or changing its contents. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Order stored deletions before opaque renames — **DONE** | 25m | — |
| t002 | Permit one rename into an explicitly deleted destination — **DONE** | 35m | t001 |
| t003 | Verify shared service and group callers — **DONE** | 35m | t002 |
| t004 | Render parity — **DONE** | 25m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 35m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- On the owned Free image-service reproduction in [finding.md](finding.md), delete RIGHT and rename NEW into RIGHT without changing/revealing contents: one **Save only** succeeds for the variable-only, file-only and combined cases. Exact original source bytes are stored at RIGHT/right, source names are absent and final names remain unique.
- Fresh-load at 390×844 and replay the same file-only clicks: one successful Save retains the exact file contents, settles the draft and leaves no page overflow.
- On a fresh unlinked group with DEST before SOURCE, delete DEST and rename SOURCE into DEST. One Save succeeds for the mixed and isolated file cases, revision advances once and exact source values/files replace the deleted destination contents; links remain empty.
- Service Save-only does not mint a deploy or change running contents. A normal **Deploy latest image** reaches Live, serves the saved bytes under the destination names and clears pending changes, as the two-save control already demonstrated.
- Ordinary standalone opaque rename still succeeds and the observed occupied-destination cycle still refuses without committing changed contents. Failed saves retain the editable draft and report the real error.
- Replay and delete every owned fixture/group; inventories return to baseline and revoke only the QA session.

## Source + Goal linkage

- **Source:** functional qa-find-bugs, 2026-10-10 pass a73; [finding.md](finding.md) contains complete requests/responses, live controls, exact source lines, shared consumers, cleanup and unverified work.
- **Goal linkage:** [ADR008 dependable AI-native hosting](../../../../docs/ADR008-vision.md), [ADR013 stored environment maps](../../../../docs/ADR013-secrets.md), [ADR006 coherent API behavior](../../../../docs/ADR006-bex-api.md) and [ADR018 parity](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** a unique final environment draft can replace a deliberately removed destination with an existing opaque source in one write, preserving its bytes and current Save-only/deploy behavior.
- **Why now:** both service and group editors fail today with different operation-order errors; client ordering alone fails. Shared core changes need a separate caller audit and coordination with the distinct source-name reuse item [w4/234](../../234.md).
- **Scope:** approximately 3h across client/core changes, shared-caller checks and closing work. The fix is limited to explicit deletion followed by one opaque rename; preserve occupied-name/cycle refusals and ordered API semantics. REST/MCP and sibling resource types remain unverified work in t003/t004, not observed DoD claims. Render parity applies because UI and all three API surfaces share this behavior.

## Closeout

Closed 2026-10-11 after live acceptance at shipped `1cc56f66a`: one Save only now commits delete-then-opaque-rename for the combined, variable-only, file-only (including fresh 390×844) service cases and the mixed/file-only unlinked group cases with exact source bytes; Save-only minted no deploy and Deploy latest image served the moved bytes; the occupied swap still refuses with the real error and retains the draft. Evidence, caller audit, parity and unverified scope: [verification.md](verification.md).
