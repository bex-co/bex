# CLI sweep 594 — rollback targets that never went live: both refused by name

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixture `qa594-web` (`srv-dbb98q4gnrlc73bhsru0`, free hello-app), deleted: REST 404, URL 404, zero `qa594` names. Completes sweep 593's Unrun pair.

- **Canceled target:** deploy A (`hello-app:2.0`) was superseded by a following deploy and settled `canceled`. `POST rollback {deployId: A}` → `409 conflict: deploy "…" never went live — nothing to roll back to`. No new deploy row created; the live release kept serving `Version: 1.0.0`.
- **Failed target:** deploy F (`hello-app:9.9`, nonexistent tag) was still `update_in_progress` (pull failing) at probe time. Rollback → the same named `409 … never went live`. The refusal keys on "went live", not on the terminal label, so an in-flight deploy is refused with the same honest reason.
- Together with sweep 593 (live → `409 already live`; deactivated → accepted), the rollback target matrix is fully named: only a deploy that once went live and no longer is can be rolled back to.

## Unrun

- Rollback to a deploy that went live but whose image was since deleted from the registry.
- Rollback on a Git-backed service (commit restoration).
