# w7 · m35 — Outbound webhook SSRF guard (shared dial-time IP block with the activator)

**Worker:** worker7 **Goal:** Close the one open ADR028 security finding: the outbound-webhook worker POSTs to tenant-supplied URLs with no SSRF protection, so a tenant can register `http://169.254.169.254/…` or an internal `10.x`/`192.168.x` endpoint and bex-api will POST to it. Port `w1/m37`'s dial-time IP guard (built for the maintenance-page fetch) to the webhook path, extracting it to a shared helper. **Status:** todo

## Tasks (in order)

| id   | title                                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Extract m37's dial-time IP guard from the activator into a shared helper (one implementation, two call sites) | 45m | —          |
| t002 | Apply it to `webhooks/worker.go`'s `defaultClient` (block loopback/private/link-local/unspecified at dial, no redirect-follow); activator switches to the shared helper | 30m | t001       |
| t003 | SSRF test battery: `169.254.169.254`, `10.x`, `127.0.0.1`, DNS-rebind-to-private, redirect-to-metadata — all blocked; public destinations still deliver | 30m | t002       |
| t004 | Simplify — `/simplify` over the code this milestone changed                                         | 20m | t003       |
| t005 | Closeout — DoD met → move milestone to `done/`                                                      | 10m | t004       |

## Definition of done

A webhook endpoint whose destination resolves to any loopback/private/link-local/unspecified/cloud-metadata address fails to deliver — blocked at dial time and logged — while a public destination still delivers; a redirect from a public URL to a private one is not followed; the activator's maintenance-page fetch and the webhook worker share one guard implementation (no duplicated block-list). Proven by the t003 battery.

## Source + Goal linkage

- **Source:** `docs/ADR028-security-review.md` follow-up register, the **open** finding filed 2026-07-15 (`w1/m37` discovered it while building the maintenance-page fetch, shipped the guard there — `net.Dialer.Control` blocking loopback/private/link-local/unspecified, no redirect-following — but left the identical pre-existing webhook gap unfixed to stay scoped, recommending "port the same guard, or extract it to a shared helper"). `/pm-brainstorm` round 11, 2026-07-15.
- **Goal linkage:** GOAL.md #7 security; w7's tenant-isolation/abuse-hardening charter; docs/ADR006-bex-api.md § Outbound event webhooks.
- **Expected outcome:** a tenant can no longer use a webhook endpoint as an SSRF primitive against cloud metadata or internal services.
- **Why now:** it's a live SSRF hole on a multi-tenant control plane, already recorded as open, with the exact fix already designed and shipped one file over — the cheapest it will ever be to close.
- **Render parity closing task: omitted** — security mechanism on an existing endpoint; no REST/GraphQL/MCP/UI field/shape change (the webhook CRUD surface is unchanged).
