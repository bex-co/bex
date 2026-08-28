# w6 · m135 — Make auto-added custom-domain pairs delete coherently

**Worker:** worker6 **Goal:** deleting one half of an auto-added www↔apex pair never silently turns the generated sibling from a redirect into a directly served domain; the resulting lifecycle is deliberate, disclosed, and consistent on every surface **Status:** todo (t001–t006 done; t007 awaits post-deploy live verification)

## Tasks (in order)

| id   | title                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------ | --- | ---------- |
| t001 | Decide pair-deletion semantics and preserve explicit user claims — **DONE**   | 40m | —          |
| t002 | Implement coherent pair deletion in the authoritative claim store — **DONE** | 50m | t001       |
| t003 | Disclose the pair outcome in the dashboard delete flow — **DONE**            | 35m | t002       |
| t004 | Render parity — **DONE**                                                      | 30m | t003       |
| t005 | Simplify — **DONE**                                                           | 20m | t004       |
| t006 | Test coverage — **DONE**                                                      | 45m | t004       |
| t007 | Closeout                                                          | 10m | t006       |

## Definition of done

- The intended delete result is decided for both add directions, pending and verified claims, and explicitly claimed versus platform-generated siblings.
- Deleting either half never silently changes a surviving domain from redirecting to directly serving; the backend either removes the generated pair coherently or preserves it only under a disclosed, intentional rule.
- REST, GraphQL, MCP, and dashboard reads all report the same post-delete claims and `redirectForName` state.
- The dashboard confirmation names every domain that will be removed or explains the surviving sibling's exact behavior before the write.
- Authorization, uniqueness, ownership verification, TLS cleanup, and retry/idempotency behavior remain intact.
- A live www-first and apex-first matrix confirms the chosen behavior for pending and verified pairs.

## Source + Goal linkage

- **Source:** promoted from [`w6/056`](../done/056.md), live-reproduced 2026-08-27: deleting the apex left the auto-added `www` claim alive and stripped its `redirectForName`.
- **Goal linkage:** [ADR005](../../../docs/ADR005-custom-domain.md), [ADR006](../../../docs/ADR006-bex-api.md), and [ADR018](../../../docs/ADR018-render-parity.md): custom-domain lifecycle must be predictable and identical across public surfaces.
- **Expected outcome:** one user action cannot leave behind a domain the platform created, serving with a meaning the user never selected.
- **Why now:** this is a user-visible ownership and routing change hidden behind an ordinary delete confirmation, and it affects the default first-domain flow because bex auto-adds the sibling immediately.
- **Render parity:** included because custom-domain deletion and www↔apex behavior are tenant-facing REST/GraphQL/MCP/dashboard semantics.

## Closeout evidence

- **Implemented 2026-08-28:** deleting the canonical/direct claim atomically deletes its generated `redirectForName` sibling; deleting only the generated sibling preserves the canonical claim; explicitly re-added direct siblings remain independent.
- **Executable matrix:** managed pending and verified claims, apex-first and `www`-first adds, canonical and generated deletes, explicit sibling preservation, and idempotent retry are covered in `TestManagedDomainPairDeletionMatrix` and adjacent storeless/Postgres tests.
- **Surface evidence:** REST, GraphQL, MCP, and dashboard regression tests assert the same cascade and disclosed survivor behavior.
- **Green gates:** `cd lego/backend && go test ./...`; the real `TestPGStore` suite against disposable Postgres 17; `make lint-backend`; `dashboard/yarn typecheck`; `dashboard/yarn lint`; `dashboard/yarn test` (379 files, 2,780 tests).
- **Remaining for t007:** deploy through the authorized ship workflow, then repeat the live pending/verified apex-first and `www`-first matrix. No commit, push, or production deploy was performed in this worktree-only implementation pass.
