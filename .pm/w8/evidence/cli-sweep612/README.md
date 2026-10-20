# CLI sweep 612 — failing health-check path: new rollout stalls honestly, old release keeps serving

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa612-bb` (`srv-dbba90sgnrlc73bhssvg`, free busybox httpd serving only `/`), deleted at close: REST 404, zero `qa612` names. (A first attempt on hello-app was void — that app answers 200 on every path, so the check could never fail; busybox httpd returns a real 404.)

- **Setup sanity:** `/` → 200 (`qa612-root`), `/qa612-nope` → 404 from the app itself.
- **Breaking the check:** `PATCH healthCheckPath: /qa612-nope` opened a new deploy that sat in `update_in_progress` for the full 3-minute window with `stallReason: "the container is running but its startup health check has not succeeded, so the rollout is waiting…"`. The service phase read `Deploying`.
- **Zero-downtime held:** the public URL answered 200 from the old release at every poll — the failing revision never took traffic.
- **Recovery:** restoring `healthCheckPath: /` queued a fresh deploy that went `live` within ~30s, phase back to `Running`, content unchanged. No manual intervention beyond fixing the path was needed.

## Unrun

- A health-check path that returns 500, or redirects (3xx treated as healthy?).
- First-deploy health-check failure (no old release to fall back on) and its terminal verdict timing.
