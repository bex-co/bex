# CLI sweep 604 — `deploys create --commit` on a Git-backed service: old SHA, short SHA, garbage, nonexistent

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa604-git` (`srv-dbb9psabpdss73ci0n4g`, free express-hello-world from sweep 603's repo), deleted at close: REST 404, URL 404, zero `qa604` names. Completes sweep 603's Unrun item.

- **Old commit deploys and goes live:** after the initial deploy of HEAD (`039c3477…`), `deploys create --commit 59bb07278c918ac8798e2532b4251f7c34e365f5` built and went `live` carrying exactly that commit; the app served throughout.
- **Short SHA accepted:** `--commit 59bb072` minted a deploy resolved to the same full commit (it was then superseded by the full-SHA create — the usual newest-wins cancellation, sweep 584).
- **Garbage refused up front:** `--commit 'not-a-sha!'` → `400 commitId must be a git ref (no whitespace, shell metacharacters, or leading dash)`, no deploy row.
- **Well-formed but nonexistent SHA fails honestly at build:** `--commit 0000…0000` minted a deploy that settled `build_failed`; the detail's `failureReason` and the build log both say `build failed in the clone step: fatal: remote error: upload-pack: not our ref 0000…`. The previously live release kept serving (`Hello from Render!`) the whole time — a failed build never took the service offline.

## Unrun

- `--commit` naming a branch or tag instead of a SHA (the refusal wording accepts any git ref).
- Commit deploy on a service whose repo has become unreachable since creation.
