# w3 · m304 — Security review of the static-site external-rewrite proxy

**Worker:** worker3 **Goal:** the tenant-controlled outbound fetch that w8/184 shipped has a recorded review, its findings fixed, and a threat model that matches the code **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Threat-model the external-rewrite fetch path — **DONE** | 1.5h | — |
| t002 | Fix the confirmed findings with regression tests — **DONE** | 1.5h | t001 |
| t003 | Rewrite ADR029's threat model and record the review as ADR103 — **DONE** | 45m | t002 |
| t004 | Live probe on production: owned upstream allowed, private targets refused — **DONE** | 45m | t002 |
| t005 | Render parity — **DONE** | 30m | t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- ADR103 records a verdict with evidence for every attack in t001's list, and the sign-off
- Every finding above accepted is fixed with a regression test, or filed as its own note with the reason
- `docs/ADR029-static-sites.md` no longer says a site owner cannot choose an upstream server
- Live on production: an owned upstream is proxied (200, its body); a public name resolving to a private address, the metadata address and a cluster `.svc` name are refused; fixtures deleted with 404 proofs

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-09/10, item 3 (filed by `/pmx them all to w3`); `.pm/w8/done/184.md:58` ("a dedicated review sign-off is still owed"); `docs/ADR029-static-sites.md:54`
- **Goal linkage:** V0 roadmap item 7, security review; tenant isolation (ADR043)
- **Expected outcome:** a tenant-chosen outbound fetch from the platform has reviewed, tested limits
- **Why now:** it has been live since 2026-10-22 without its owed review, and open signup widens who can configure it
- **Render parity:** included conservatively — t002 will likely tighten rewrite validation on REST/GraphQL/MCP and the dashboard; if no surface changes, the parity task records that and closes

## Closed (2026-11-10, loopx w3)

- ADR103 records a verdict with evidence for each of the thirteen attacks and the sign-off.
- F1–F3 are fixed with regression tests (mutation-checked); four residuals are accepted with their reasons.
- ADR029 no longer says a site owner cannot choose an upstream server.
- Live on production the owned upstream was proxied (200) and metadata, private and loopback targets behind public names were refused (502), a cluster name at save (400); the fixture was deleted with a 404 proof.
