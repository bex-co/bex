# w6 · m94 — Auto-hibernate/wake Ingress-backend race can leak a raw Traefik error to the first request

**Worker:** worker6 **Goal:** the first request against a service that just hibernated always gets bex's documented wake contract, never raw infrastructure output **Status:** todo

## Tasks (in order)

| id   | title                                                                | est | depends_on |
| ---- | --------------------------------------------------------------------- | --- | ----------- |
| t001 | Characterize the auto-hibernate Ingress-backend race and decide the fix | 30m | —           |
| t002 | Close the auto-hibernate Ingress-backend race                       | 40m | t001        |
| t003 | Render parity: confirm the wake-response contract is unaffected     | 15m | t002        |
| t004 | Simplify                                                             | 10m | t003        |
| t005 | Test coverage                                                        | 40m | t004        |
| t006 | Closeout                                                             | 10m | t005        |

## Definition of done

- Repeated live trials (hibernate a fresh `qa-` free-tier service, then request it immediately on the next reconcile) no longer produce Traefik's raw `no available server` — every request gets either the real app response or bex's documented wake contract (`docs/render-artifacts/wake-interstitial.md`) — live-verifiable on `dashboard.bex.co`.
- The resume-path (reverse) race is explicitly addressed as in-scope or explicitly deferred with its own note, not silently ignored.
- Manual suspend and maintenance-mode routing are unaffected.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-25 (run from a `/loop 10m` session), journey 15 (free-tier sleep → wake). Repro: `qa-20260825-sleep` (Free, `bex-co/bex` @ `examples/hello-go`, `idleTTLSeconds=45`) — first `curl` immediately after `server.phase` read `Hibernated` got `HTTP/2 503`, `no available server`, `content-type: text/plain`, no `Retry-After`; three follow-up requests ~20s later all got the real app's `200`.
- **Goal linkage:** [ADR029](../../../docs/ADR029-static-sites.md)'s sibling economics note and `docs/render-artifacts/wake-interstitial.md`'s documented contract — the free tier's core value proposition depends on the wake path being trustworthy on the very first request, not just eventually consistent. `w6/m47` (done) fixed the structural always-reproducing version of this same promise; this is the transient race that remained after that fix.
- **Expected outcome:** a hibernated free service's first wake request never surfaces raw Traefik/infrastructure text to an end user or API client.
- **Why now:** caught live in the same continuous `/qa-find-bugs` cadence that produced `w6/m47`, `w6/m93`, `w6/041` — filing while the repro and code citations are fresh, per the same lineage as `w9/m89`/`w9/m92`.
- **Render parity task included:** t003 — confirms the fix doesn't change the publicly observable wake-response contract `w6/m47`'s own Render-parity work established; this milestone changes reconcile timing, not response shape, so it's a confirmation pass rather than new surface work.
