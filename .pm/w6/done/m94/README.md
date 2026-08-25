# w6 · m94 — Auto-hibernate/wake Ingress-backend race can leak a raw Traefik error to the first request

**Worker:** worker6 **Goal:** the first request against a service that just hibernated always gets bex's documented wake contract, never raw infrastructure output **Status:** done — both races fixed and gated green; live verification carried to `w6/040` (blocked on the deploy pipeline)

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Characterize the auto-hibernate Ingress-backend race and decide the fix | 30m | — | — **DONE** |
| t002 | Close the auto-hibernate Ingress-backend race | 40m | t001 | — **DONE** |
| t003 | Render parity: confirm the wake-response contract is unaffected | 15m | t002 | — **DONE** |
| t004 | Simplify | 10m | t003 | — **DONE** |
| t005 | Test coverage | 40m | t004 | — **DONE** |
| t006 | Closeout | 10m | t005 | — **DONE** |

## Definition of done

- Repeated live trials (hibernate a fresh `qa-` free-tier service, then request it immediately on the next reconcile) no longer produce Traefik's raw `no available server` — every request gets either the real app response or bex's documented wake contract (`docs/render-artifacts/wake-interstitial.md`) — live-verifiable on `dashboard.bex.co`.
- The resume-path (reverse) race is explicitly addressed as in-scope or explicitly deferred with its own note, not silently ignored.
- Manual suspend and maintenance-mode routing are unaffected.

## Outcome (2026-08-25)

**The race is real and confirmed in code — and it has a second, worse sibling the filing raised only as a question. Both are fixed.**

The ordering claim held up exactly as written: the Deployment scale-to-0 write (`:1611`) precedes the Ingress swap to the activator (`:1661`/`:1665`), and the two are non-atomic writes Traefik ingests independently.

Walking the wake path then settled t001's open step 4. The activator does not proxy — it answers `503` with `Retry-After: 5` and stamps `lastActive`, which makes the very next reconcile hand the route straight back to a Deployment whose pods are still starting. So a client retrying at exactly the second bex told it to hit an endpoint-less Service and got the same raw `no available server`. That is not a narrow window like the hibernate side: it is close to deterministic for any app slower than ~5s to become ready, on the path the user is actively waiting on.

Two fixes, both in `app_controller.go`:

- **Readiness-gated routing** — the activator holds the public route whenever the App has no ready endpoint and the activator is its wake path, not merely once the idle TTL elapses. The Ingress never names a Service with nothing to serve. This closes the resume race and, for free, gives a free-tier App that is unready for any other reason (fresh deploy, crash loop) the documented interstitial instead of raw Traefik text.
- **Route-before-drain with a bounded grace** — the hibernate pass writes the activator route, keeps the pods, and requeues 10s later to scale down. Reordering alone was considered and rejected for the reason t001 itself flagged: Traefik's propagation is asynchronous and unobservable to the operator, so no ordering of two non-atomic writes can close the window, only narrow it. Buying bounded time is the honest cover for an unobservable dependency, and an already-idle App pays nothing for it.

**Maintenance mode is exempt from the hold** — caught by an existing test, not by inspection: the first cut held unconditionally, which would have stopped a maintenance-mode App hibernating forever, since its Ingress will never name the activator. Manual suspend and workers are untouched by both changes.

One pre-existing test asserted the racy contract (route returns to the App's Service on the first post-wake reconcile). Its intent — the alias is a sleeping-state detail, not a permanent indirection — is preserved; it now requires a ready pod first, plus a new negative case. The `gocyclo` gate also caught the additions pushing `reconcileKubernetes` from 30 to 36; rather than suppress it, the additions were folded down and the self-contained disk/snapshot/restore sequence extracted, leaving the function at **28** — below where it started.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-25 (run from a `/loop 10m` session), journey 15 (free-tier sleep → wake). Repro: `qa-20260825-sleep` (Free, `bex-co/bex` @ `examples/hello-go`, `idleTTLSeconds=45`) — first `curl` immediately after `server.phase` read `Hibernated` got `HTTP/2 503`, `no available server`, `content-type: text/plain`, no `Retry-After`; three follow-up requests ~20s later all got the real app's `200`.
- **Goal linkage:** [ADR029](../../../docs/ADR029-static-sites.md)'s sibling economics note and `docs/render-artifacts/wake-interstitial.md`'s documented contract — the free tier's core value proposition depends on the wake path being trustworthy on the very first request, not just eventually consistent. `w6/m47` (done) fixed the structural always-reproducing version of this same promise; this is the transient race that remained after that fix.
- **Expected outcome:** a hibernated free service's first wake request never surfaces raw Traefik/infrastructure text to an end user or API client.
- **Why now:** caught live in the same continuous `/qa-find-bugs` cadence that produced `w6/m47`, `w6/m93`, `w6/041` — filing while the repro and code citations are fresh, per the same lineage as `w9/m89`/`w9/m92`.
- **Render parity task included:** t003 — confirms the fix doesn't change the publicly observable wake-response contract `w6/m47`'s own Render-parity work established; this milestone changes reconcile timing, not response shape, so it's a confirmation pass rather than new surface work.
