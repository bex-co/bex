# w1 · m89 — One confirm dialog for the dashboard

**Worker:** worker1 **Goal:** the dashboard has exactly one confirm-dialog implementation, and a guard that keeps it that way. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Extract the shared `ConfirmDialog` | 45m | — | — **DONE**
| t002 | Migrate the heaviest call sites | 45m | t001 | — **DONE**
| t003 | Migrate the remaining call sites and add the lint guard | 1h | t002 | — **DONE**
| t004 | Simplify pass | 30m | t003 | — **DONE**
| t005 | Test coverage | 30m | t003 | — **DONE**
| t006 | Closeout | 30m | t005 | — **DONE**

## Closeout (2026-08-24)

**t001 done.** `common/components/confirm-dialog.tsx` supports all three shapes that are actually in use — uncontrolled `trigger`, controlled `open`/`onOpenChange`, and a `phrase` gate — plus `pending` and a `children` slot. `RevokeIconButton` was reimplemented on top of it and its existing tests pass unchanged, which is what proves the API fits rather than merely compiles.

**t005 done.** Five tests at the primitive cover both trigger shapes, cancel reporting dismissal, the phrase gate refusing a near-miss, `pending`, and the children slot.

**t002 partial — 3 sites migrated:** `disk-section.tsx` (which had hand-rolled its own phrase dialog because no shared one existed — that local copy is now deleted), `cron-runs-section.tsx` (both dialogs, one of them correctly marked non-destructive), and `revoke-icon-button.tsx`. **22 files remain.**

**Completed in a second pass: all 25 sites migrated, plus 3 the initial grep had missed.** The guard test found those three — `key-value-confirm-dialogs.tsx`, `blueprints.$blueprintId.tsx`, `project.$projectId.settings.tsx` — because they import other kit pieces without `AlertDialogCancel`, which is exactly the kind of gap a grep-driven sweep leaves and a structural guard closes.

**The API grew from the call sites, not from guesswork.** Six additions, each forced by a real site rather than anticipated: `confirmLabel` as a node (sites swap in a spinner), cancel disabled while `pending` (dismissing mid-flight hides an operation that is still going to happen), controlled-*and*-triggered in one path (several sites are both), `onCancel` (a navigation blocker must be reset, not merely left), `confirmDisabled` (sites that render the shared `SudoCommandField` and own their own gate), and `contentClassName` (the blueprint sync dialog's scroll cap). The phrase input also gained a placeholder, which preserved an existing test's query and improves every phrase-gated site.

**Two hand-rolled phrase gates were absorbed:** the disk tab's `SudoConfirmDialog` and `webhook-settings-card.tsx`'s own `confirmText`/`confirmed` state — both existed only because no shared primitive did.

**A bulk transform was attempted and abandoned** in the first pass. A regex converter handled 22 blocks on its first pass but produced unparseable JSX: descriptions that mix prose with an interpolation cannot become a plain attribute value. Tightening it to convert only single-expression blocks then matched nothing, which is the honest signal that these call sites vary more than a regex should be trusted with. The work was reverted from a backup and the rest were converted by hand, one at a time, with each site's tests re-run — the right call, since the shapes varied more than any regex would have survived.

**Test note, stated plainly:** the full suite flaked on 7 different files across 4 runs at a machine load average of ~150 (unrelated containers). Every one passed in isolation, including files this milestone never touched — the last run's failure was `new-session-composer.test.tsx`, which is untouched here and passes 24/24 alone. Every file this milestone changed passes. The flakes are resource contention, not regressions.

## Definition of done

`grep -rl AlertDialogCancel dashboard/src/features dashboard/src/common` returns only the shared primitive and the shadcn kit. `RevokeIconButton` and the disk tab's typed-phrase dialog are both implemented on top of it rather than beside it. Every migrated site's existing tests pass unchanged. A guard test fails when a component reintroduces a hand-rolled AlertDialog.

## Source + Goal linkage

- **Source:** [.pm/w1/075.md](../done/075.md), filed during the w1/m86 simplify pass.
- **Goal linkage:** dashboard consistency. Any improvement to the shape — a pending spinner on confirm, a consistent destructive variant, an aria fix — currently has to land 27 times or land inconsistently, which in practice means it lands inconsistently.
- **Expected outcome:** one place to change confirm-dialog behavior, and destructive-action UX that cannot drift between features.
- **Why now:** the disk tab just added copies 28 and 29, including a typed-phrase variant it had to hand-roll because no shared primitive existed. The pattern is actively multiplying, and each new one raises the migration cost.
- **Render parity task omitted:** this is a behavior-preserving internal refactor. No REST/GraphQL/MCP surface changes, and no dialog's copy or semantics change — only where the JSX lives. Migrated sites must pass their existing tests unchanged, which is the parity check that actually applies here.
