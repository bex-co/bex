# w7 · m54 — Static-site multi-tenant trust boundaries

**Worker:** worker7 **Goal:** Close the accepted browser contract, Kubernetes alias-authority, and object-store credential boundaries around the shared static-site serving plane. **Status:** done

## Tasks (in order)

| id   | title                                                          | est | depends_on      |
| ---- | -------------------------------------------------------------- | --- | --------------- |
| t001 | Capture the static-site threat model and reproducible baseline — **DONE** | 45m | —               |
| t002 | Evaluate `onbex.co` PSL inclusion — **DONE (SKIPPED BY OWNER)** | 45m | t001            |
| t003 | Enforce operator-only ExternalName alias authority — **DONE** | 45m | t001            |
| t004 | Provision separate bucket-scoped static-site identities — **DONE**        | 45m | t001            |
| t005 | Wire, rotate, and production-prove read/write S3 separation — **DONE** | 45m | t004            |
| t006 | Verify and document browser-origin divergence — **DONE**       | 30m | t002, t003, t005 |
| t007 | Simplify the static-site security implementation — **DONE**    | 30m | t006            |
| t008 | Complete negative-path and production smoke coverage — **DONE** | 45m | t006, t007      |
| t009 | Close out the milestone — **DONE**                             | 15m | t008            |

## Definition of done

PSL membership is an explicitly accepted non-goal: real-browser evidence must report parent-domain cookie behavior without gating it (the closeout capture shows an `onbex.co` cookie crossing sibling tenants), while local storage and Service Workers stay origin-local and platform authentication remains isolated on `bex.co`. Tenant-facing identities are denied from creating or mutating Ingresses and ExternalName Services, an attempt to retarget an operator alias to an internal service is rejected, and the operator can still reconcile static-site and maintenance aliases. The static-server uses a dedicated read-only identity scoped to the static-content bucket; publish/purge Jobs use a separate write/delete identity scoped to that bucket; both identities are denied from Terraform-state, backup, and unrelated buckets; the old shared credential is revoked. CI is green and production evidence shows a fresh static deploy publishes, serves over the platform hostname, and deletes without leaking secret values.

## Source + Goal linkage

- **Source:** User `$ship` then `$pm` handoff on 2026-07-28 after repairing `srv-d9e5sbd5qe4s73b1mjq0`; the security review found that browser-to-tenant-private-network routing remains closed, while browser-domain behavior, ExternalName authority, and shared S3 credential boundaries needed explicit decisions.
- **Goal linkage:** Advances `GOAL.md` V0 #7 (security review) and the w7 tenant-isolation charter by protecting the public static-site edge and its platform-side credentials without changing the operator → types ← backend architecture.
- **Expected outcome:** The accepted browser divergence remains explicit and reproducible; tenant static sites cannot turn Traefik's ExternalName support into an internal routing primitive or expose Terraform state or other buckets through a compromised static serving/publishing component.
- **Why now:** The production repair activated cross-namespace ExternalName routing and exposed the build-namespace static credential path. The milestone closed the controllable cluster/object-store risks and forced an explicit owner decision on the remaining shared-cookie behavior.
- **Render parity:** No REST, GraphQL, MCP, or dashboard schema changed. Real Chrome proves local storage and Service Workers remain origin-local but parent-cookie behavior differs from `onrender.com`; the owner explicitly accepted that divergence and waived PSL inclusion on 2026-07-30.
