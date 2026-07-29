# w7 · m54 — Static-site multi-tenant trust boundaries

**Worker:** worker7 **Goal:** Close the browser-domain, Kubernetes alias-authority, and object-store credential boundaries around the shared static-site serving plane. **Status:** todo (t001, t004 done)

## Tasks (in order)

| id   | title                                                          | est | depends_on      |
| ---- | -------------------------------------------------------------- | --- | --------------- |
| t001 | Capture the static-site threat model and reproducible baseline — **DONE** | 45m | —               |
| t002 | Put `onbex.co` on the Public Suffix List                       | 45m | t001            |
| t003 | Enforce operator-only ExternalName alias authority             | 45m | t001            |
| t004 | Provision separate bucket-scoped static-site identities — **DONE**        | 45m | t001            |
| t005 | Wire, rotate, and production-prove read/write S3 separation    | 45m | t004            |
| t006 | Verify Render-equivalent browser-origin isolation              | 30m | t002, t003, t005 |
| t007 | Simplify the static-site security implementation               | 30m | t006            |
| t008 | Complete negative-path and production smoke coverage           | 45m | t006, t007      |
| t009 | Close out the milestone                                        | 15m | t008            |

## Definition of done

`onbex.co` is present in the browser-consumed Public Suffix List and a real-browser probe proves that one tenant subdomain cannot set a parent-domain cookie that is sent to another tenant subdomain. Tenant-facing identities are denied from creating or mutating Ingresses and ExternalName Services, an attempt to retarget an operator alias to an internal service is rejected, and the operator can still reconcile static-site and maintenance aliases. The static-server uses a dedicated read-only identity scoped to the static-content bucket; publish/purge Jobs use a separate write/delete identity scoped to that bucket; both identities are denied from Terraform-state, backup, and unrelated buckets; the old shared credential is revoked. CI is green and production evidence shows a fresh static deploy publishes, serves over the platform hostname, and deletes without leaking secret values.

## Source + Goal linkage

- **Source:** User `$ship` then `$pm` handoff on 2026-07-28 after repairing `srv-d9e5sbd5qe4s73b1mjq0`; the security review found that browser-to-tenant-private-network routing remains closed, but the PSL, ExternalName authority, and shared S3 credential boundaries need explicit closure.
- **Goal linkage:** Advances `GOAL.md` V0 #7 (security review) and the w7 tenant-isolation charter by protecting the public static-site edge and its platform-side credentials without changing the operator → types ← backend architecture.
- **Expected outcome:** Tenant static sites are browser-isolated like Render-hosted sites, cannot turn Traefik's ExternalName support into an internal routing primitive, and cannot expose Terraform state or other buckets through a compromised static serving/publishing component.
- **Why now:** The production repair activated cross-namespace ExternalName routing and exposed the build-namespace static credential path. Closing the adjacent trust boundaries before more tenants use `*.onbex.co` avoids baking in a shared-cookie domain and an account-wide object-store blast radius.
- **Render parity:** Included because the platform hostname is a tenant-facing browser security surface: `onrender.com` already participates in the Public Suffix List, and the milestone must verify equivalent cookie/site isolation. No REST, GraphQL, MCP, or dashboard schema change is expected.
