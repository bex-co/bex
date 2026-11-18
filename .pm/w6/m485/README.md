# w6 · m485 — A platform release drops no request the activator is holding through a cold start

**Worker:** worker6 **Goal:** when a deploy replaces an activator pod, every request it is holding for a waking free service, or proxying to one, gets the service's answer, and requests routed to it while its endpoint is withdrawn are still served; Render's promise that a request to a spun-down free service "loses nothing" (w8/178) holds through bex's own releases. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The activator drains on SIGTERM](t001.md) | 1h | — |
| t002 | [The activator Deployment gives a draining pod its bound](t002.md) | 30m | t001 |
| t003 | [Tests: a request held at SIGTERM is answered; the process exits once idle](t003.md) | 45m | t001 |
| t004 | [Record the rule in ADR007](t004.md) | 15m | t002 |
| t005 | [Ship and verify](t005.md) | 45m | t002, t003, t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- A test sends SIGTERM to the activator while a request is held for a waking App and the request still gets the App's response, not a reset or a 502; another shows a request arriving within the propagation window after SIGTERM is served.
- The activator process exits as soon as it is idle after SIGTERM, and at the latest at its bound; the Deployment's grace period covers the bound.
- A non-HTML request to a hibernated free fixture, in flight while its activator pod is replaced, answers 200 (on production in `bex-canary`, or on the local cluster if no production deploy lines up, recorded which).
- ADR007 records the rule and its limit.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-17 (item 4 after verification), approved by the user's `/pmx them all to w6` 2026-11-18. Evidence:
  - `lego/operator/cmd/activator/main.go:147-157` calls `srv.ListenAndServe()` with no signal handling and no `Shutdown`; no package the activator links installs a SIGTERM handler (checked with `go list -deps` 2026-11-18), so the Go runtime exits at once on SIGTERM. `config/activator/deployment.yaml` has a TCP readiness probe, no `preStop` and the default 30 s grace.
  - `cmd/activator/hold.go:37-64` (w8/178): a non-browser request to a sleeping App is held up to 90 s while it wakes, then reverse-proxied (response header timeout 100 minutes).
  - The activator runs bex's image (`controller:latest` placeholder), so every production deploy replaces both pods (about 4–6 deploys a day). Traefik has no retry middleware in front of it.
  - bex-api already drains (`serve.UntilShutdown`, `lego/backend/cmd/api/main.go:742`), and static-server and the edge-cache sidecar call `Shutdown`; the activator is the exception. `serve` lives in `lego/backend`, which the operator may not import (depguard).
- **Goal linkage:** ADR008 § Render parity first. Render delays the first request to a spun-down free service until it is back up and then serves it (w8/178, [render.com/docs/free](https://render.com/docs/free)); a webhook sent during bex's own release should not be lost when the same webhook a minute earlier is kept.
- **Expected outcome:** webhooks and API calls to sleeping free services stop failing during platform releases.
- **Why now:** it happens on every deploy and is the last component on the tenant HTTP path without a drain, after w6/m469, w6/m470 and w6/m484. The loss window is narrow (only requests in flight at the moment a pod stops), so this is ranked after m482–m484.
- **Production usage not measured:** held-request volume was not read (production access failed on a stale host key during the brainstorm); t005 records it from the activator's logs or metrics.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard change; the tenant-visible effect is checked in t005.
