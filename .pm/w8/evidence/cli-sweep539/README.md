# CLI sweep 539 — restart when every deploy has failed

**Verdict: no bug filed.** Completes the restart state matrix (suspended → 409 by suspension, sweep 503; hibernated → accepted, 535; first build in flight → 409 no-live-deploy, 538; this sweep: never-served failure). Same isolated session, `bex-canary`, 2026-10-19T10:40–10:42Z.

Fixture: repo service on a nonexistent branch → sole deploy `build_failed` (clone-step ref error, same dialect as sweep 528), phase `Failed`, nothing ever served. Then:

```text
bex restart srv-… --confirm
Error: … 409: conflict: service "srv-dbav73vmnaes738pfotg"
has no live deploy to restart; deploy it first
```

Same named gate as the in-flight case, correctly refusing to fabricate a restart from the branch head (the `liveCommit` rule). Deleted by exact ID afterward.

- Deploy lag means deployed revisions can precede local `main` (production pin `8e57d4eaa121` at sweep time).
