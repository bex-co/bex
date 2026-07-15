# w7 · m31 — `renderSubdomainPolicy`: disable the platform subdomain

**Worker:** worker7 **Goal:** Render's `renderSubdomainPolicy` (`enabled`|`disabled`) exists for bex: a service with custom domains can stop serving `<slug>.onbex.co`, with the operator dropping the platform host from its Ingress/cert while custom hosts keep working. **Status:** done

## Tasks (in order)

| id   | title                                                      | est | depends_on |
| ---- | ----------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's guard semantics                             | 30m | —          | — **DONE** |
| t002 | CRD field + `effectiveHosts` drops the platform host         | 45m | t001       | — **DONE** |
| t003 | REST/GraphQL/MCP with Render's exact enum                    | 40m | t002       | — **DONE** |
| t004 | Custom Domains section toggle                                | 30m | t003       | — **DONE** |
| t005 | Activator + static-resolver honor the absent host            | 30m | t002       | — **DONE** |
| t006 | Render parity                                                | 30m | t004, t005 | — **DONE** |
| t007 | Simplify                                                     | 30m | t006       | — **DONE** |
| t008 | Test coverage                                                | 40m | t006       | — **DONE** |
| t009 | Closeout                                                     | 15m | t008       | — **DONE** |

## Definition of done

A web service with a custom domain set to `disabled` stops answering on `<slug>.onbex.co` (Ingress rule + TLS host gone, status URL reflects the custom host) while the custom domain keeps serving; flipping back to `enabled` restores the platform host; the guard case (disabling with no custom domain) behaves per the t001 capture — never a silently unreachable service; the field round-trips with Render's exact name/enum on all three surfaces and toggles in the dashboard's Custom Domains section.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 6, 2026-07-14 — field-level spec-grep of Render's live OpenAPI (`renderSubdomainPolicy`: enum `enabled|disabled`, "controls whether render.com subdomains are available for the service", on `webServiceDetails` + POST/PATCH). Zero hits in `lego/` — never inventoried.
- **Goal linkage:** Render parity (service contract field); custom-domain tenants avoid duplicate-content serving on the platform host. Host policy is w7's domain-guard territory (m6's collision/reserved-host work; w4/m19's slug/effectiveHosts seam is the mechanism).
- **Expected outcome:** the platform subdomain becomes opt-out-able exactly as on Render; one fewer permanent allowlist entry for `w7/m30`'s conformance suite.
- **Why now:** verified-in-spec gap on the freshest seam on the board (`effectiveHosts`, reworked in w4/m19); w7 is at two open milestones. Render parity task included — all-surface change.
