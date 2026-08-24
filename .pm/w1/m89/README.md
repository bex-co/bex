# w1 · m89 — One confirm dialog for the dashboard

**Worker:** worker1 **Goal:** the dashboard has exactly one confirm-dialog implementation, and a guard that keeps it that way. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Extract the shared `ConfirmDialog` | 45m | — |
| t002 | Migrate the heaviest call sites | 45m | t001 |
| t003 | Migrate the remaining call sites and add the lint guard | 1h | t002 |
| t004 | Simplify pass | 30m | t003 |
| t005 | Test coverage | 30m | t003 |
| t006 | Closeout | 30m | t005 |

## Definition of done

`grep -rl AlertDialogCancel dashboard/src/features dashboard/src/common` returns only the shared primitive and the shadcn kit. `RevokeIconButton` and the disk tab's typed-phrase dialog are both implemented on top of it rather than beside it. Every migrated site's existing tests pass unchanged. A guard test fails when a component reintroduces a hand-rolled AlertDialog.

## Source + Goal linkage

- **Source:** [.pm/w1/075.md](../done/075.md), filed during the w1/m86 simplify pass.
- **Goal linkage:** dashboard consistency. Any improvement to the shape — a pending spinner on confirm, a consistent destructive variant, an aria fix — currently has to land 27 times or land inconsistently, which in practice means it lands inconsistently.
- **Expected outcome:** one place to change confirm-dialog behavior, and destructive-action UX that cannot drift between features.
- **Why now:** the disk tab just added copies 28 and 29, including a typed-phrase variant it had to hand-roll because no shared primitive existed. The pattern is actively multiplying, and each new one raises the migration cost.
- **Render parity task omitted:** this is a behavior-preserving internal refactor. No REST/GraphQL/MCP surface changes, and no dialog's copy or semantics change — only where the JSX lives. Migrated sites must pass their existing tests unchanged, which is the parity check that actually applies here.
