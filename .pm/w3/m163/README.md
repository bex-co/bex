# w3 · m163 — Payment wall: turn the self-host exit into a deliberate delete-and-self-host

**Worker:** worker3 **Goal:** The payment wall's self-host escape hatch stops being a bare outbound link and becomes an explicit, confirmed "delete this account and self-host" — so someone who does not want to add a card leaves cleanly instead of leaving an abandoned workspace behind. **Status:** todo (t001–t003 done; t004–t007 open)

## Tasks (in order)

| id   | title                                                          | est | depends_on   |
| ---- | -------------------------------------------------------------- | --- | ------------ |
| t001 | Rewrite the payment-wall exit copy (en + zh) — **DONE**                     | 30m | —            |
| t002 | Add the typed-confirmation dialog and wire account deletion — **DONE**      | 1h  | t001         |
| t003 | Update ADR075 § Positioning to match the new exit semantics — **DONE**      | 20m | t002         |
| t004 | Render parity check for the changed surface                      | 30m | t002         |
| t005 | `/simplify` over the milestone's changes                         | 20m | t004         |
| t006 | Test coverage for the new exit                                   | 45m | t004         |
| t007 | Closeout                                                         | 15m | t006         |

## Definition of done

- The payment wall's self-host exit reads as an explicit account deletion, not as "Self-host bex instead".
- Choosing it opens a confirmation dialog that states the account is permanently deleted, that the user was never charged, and that nothing is kept — and requires typing the account email to proceed.
- Confirming deletes the account and forwards to the self-hosting guide. Dismissing leaves the account untouched.
- The exit cannot delete an account in a single click from the wall.
- `docs/ADR075-user-onboarding.md` § Positioning describes the exit as it now behaves.

## Source + Goal linkage

- **Source:** user direction 2026-09-16, during the "web services stuck at 7" investigation — "把绑卡页面的去 self host 的选项变成 delete this account and go to self-host".
- **Goal linkage:** [ADR075 user onboarding](../../../docs/ADR075-user-onboarding.md) § Positioning — the payment wall's exits are meant to be honest paths out, not dead ends. bex being open source is the genuine alternative to adding a card.
- **Expected outcome:** someone who declines the card gate leaves with their account removed and a clear path to running bex themselves, instead of becoming an abandoned workspace that still counts as a signup.
- **Why now:** the same investigation showed the wall is doing its job as an intake filter — of 31 signups, 12 opened checkout and 10 walked away, with zero card declines. Those 10 left nothing behind but dormant workspaces. The exit is the one part of the wall that currently does nothing.
- **Render parity closing task included:** this changes a user-facing dashboard surface and triggers account deletion, so the surfaces it exposes must be checked for consistency.

## Design note — why this is two steps, not one

The exit is currently a plain link to GitHub: clicking it costs nothing. Putting an irreversible account deletion behind that same click is a large escalation on a screen where most visitors are still evaluating the product, and misclicking destroys the account. The repo's existing workspace deletion already demands a typed confirmation phrase (`lego/backend/internal/workspaces/service.go:1195`); this exit must not be looser than that. Hence: the wall's button states the intent, and the confirmation dialog is where the destructive action actually lives.

The confirmation is typed against the account **email**, not the workspace name — at payment-wall stage the workspace is usually an auto-generated `tea-*` id, and asking someone to transcribe a random identifier is both poor confirmation and needlessly hostile.
