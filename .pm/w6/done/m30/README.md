# w6 · m30 — Custom domains: sibling redirect, the real 3xx

**Worker:** worker6 **Goal:** Close the divergence `w6/m23` consciously shipped and `docs/ADR005-custom-domain.md:60` records: the auto-paired www↔apex sibling 3xx-redirects to the canonical host (the one the tenant explicitly added), matching Render — instead of being served identically. **Status:** done

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on | status |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Capture Render's exact sibling-redirect behavior live (status code, path/query preservation, cert on the redirecting host) → `docs/render-artifacts/` | 30m | — | — **DONE** |
| t002 | Operator: per-sibling-host redirect — Traefik redirect middleware + router on the sibling host (cert still issued), canonical = the tenant-added host | 60m | t001 | — **DONE** |
| t003 | Surface it: domain views mark the sibling `redirect → <canonical>` on REST/GraphQL/MCP; dashboard add-dialog copy; replace ADR005's divergence paragraph | 45m | t002 | — **DONE** |
| t004 | Render parity — behavior vs t001's capture, three-surface field consistency; refresh the ADR018 domains row | 20m | t003 | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed | 20m | t004 | — **DONE** |
| t006 | Test coverage — envtest: rendered middleware/router per pairing; redirect-vs-serve regression | 30m | t004 | — **DONE** |
| t007 | Closeout — DoD met → move milestone to `done/` | 10m | t006 | — **DONE** |

**Completed 2026-07-15.** Render capture pinned both directions at permanent `301`, preserving the full path/query with valid TLS on the redirecting host. The operator now projects a dedicated TLS-bearing redirect Ingress with one Traefik `RedirectRegex` middleware per auto-paired sibling; REST, GraphQL, MCP, and dashboard views expose `redirectForName`. Full backend, operator, and dashboard suites pass. The mock-cluster matrix verified both directions, explicit-both direct serving, and delete cleanup with trusted TLS.

## Definition of done

Curling an auto-paired sibling host returns Render's captured redirect (same status code and path/query behavior) with a valid certificate, while the canonical host serves 200; deleting the pairing removes the redirect cleanly; ADR005's "Diverges from Render on the redirect half" paragraph is replaced with the shipped behavior; verified live on the mock cluster.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 10 (2026-07-15) — `docs/ADR005-custom-domain.md:60` documents the honest divergence m23 shipped ("bex has no per-host Ingress-redirect mechanism today"); the missing mechanism (per-host Traefik middleware) has since landed in-tree via `w7/m32`'s service ipAllowList middleware work.
- **Goal linkage:** Render parity (pillar 1) — the ADR018 custom-domains row's remaining behavioral note.
- **Expected outcome:** tenant domains behave exactly like Render's: one canonical host, its sibling redirecting, both with valid certs — no duplicate-content serving.
- **Why now:** the divergence is days old with its blocker since removed; polishing shipped features is this round's charter.
- **Render parity closing task: included** (t004) — REST/GraphQL/MCP domain views + dashboard copy change.
