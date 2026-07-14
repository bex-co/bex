# w9 — Deploy experience (worker9)

**Worker:** worker9 Created 2026-07-14 from a user request (`/pm for w9`): close the gap between bex's deploy UX and Render's. bex has the deploy _machinery_ (w2/m5 deploy history + trigger, w2/m10 cancel/rollback, w2/m30 manual-deploy body, w7/m28 build logs, w1/m33 pre-deploy logs) but not Render's deploy _experience_ — the per-deploy page every deploy action lands on. Ordered UX-first: the deploy detail page is the anchor other deploy-experience work (history tab, richer statuses) would hang off.

## Milestones

- [x] **m1** — Deploy detail page: Manual Deploy jumps to a per-deploy page with its logs (9 tasks) ← from user request 2026-07-14
