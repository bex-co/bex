# CLI sweep 538 — restart while the first deploy is still building

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build at `df689525f`, 2026-10-19T10:39Z.

One repo-backed free web fixture (`bex-co/bex`, Docker runtime) created; with its first deploy still `created`/`queued` (no live release in existence):

```text
bex restart srv-… --confirm
Error: failed to restart resource: received response code 409:
conflict: service "srv-dbav6o605jqs73cjk390" has no live deploy to restart; deploy it first
```

The refusal is pure — the in-flight first deploy remained `queued`, uncanceled and un-superseded — and it names the public service ID (the w8/128 fix, re-verified in passing). The fixture was then deleted by ID (deploy reads 404, former-URL 404), canceling its never-completed build.

- Unrun: restart during a *later* in-flight deploy over a live release (the m119 overlap-policy territory, implemented and live-verified by its own milestone today), restart of a service whose only deploys all failed (same "no live deploy" gate on the code path, not separately reproduced).
- Deploy lag means deployed revisions can precede local `main` (production currently runs the `8e57d4eaa121` pin).
