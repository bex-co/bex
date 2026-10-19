# CLI sweep 526 — w8/128 live recheck: fix shipped, not yet deployed

**Verdict: deploy lag, no new filing.** Same isolated session, `bex-canary`, 2026-10-19T09:31Z.

Sweep 503's finding (restart refusals naming the internal App name) was filed as w8/128 and fixed by another worker the same morning: `8e57d4eaa fix(deploys): Restart refusals name the service the caller asked for (w8/128)`. This sweep re-ran the exact acceptance probe on a fresh owned fixture (create → suspend → `bex restart --confirm` → resume → delete, former-URL `404`):

```text
Error: failed to restart resource: received response code 409:
conflict: service "tea-daif693dqjvc73e7as3g-qa-20261019-5b6da0-re526" is suspended
```

Production still returns the internal name because the deployed platform image is pinned at `00a00c9d6870` (`218dffcd1 chore(deploy): pin platform images…`), which predates `8e57d4eaa`. This is the skill's deploy-lag case: the fix is on `main`, so no duplicate issue is filed; the live acceptance in w8/128 can only pass after the next image pin/rollout. Future sweeps in this loop should re-run this one probe opportunistically and record when the message flips to the `srv-…` ID.

Cleanup: exact-owned `srv-dbau6liot0kc73chus0g` resumed and deleted by ID; former-URL 404 verified.
