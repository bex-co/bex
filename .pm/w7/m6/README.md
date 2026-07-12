# w7 · m6 — Custom domain collision + reserved-host guard (Render "already in use" parity)

**Worker:** worker7 **Goal:** No App can claim a hostname already registered on another App (any tenant), and no App can claim a platform-owned host (`BEX_BASE_DOMAIN` apex, the dashboard host, or a foreign `<app>.<base>` auto host) — closing a cross-tenant hostname-collision path and a platform-subdomain-hijack path that cert-manager's ACME does not stop. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Cross-App uniqueness guard — reject a host already registered on another App (Render's "already in use")   | 45m | —          |
| t002 | Reserved / platform-host guard — reject base-domain apex, dashboard host, and foreign `<app>.<base>` hosts | 45m | —          |
| t003 | Surface the typed rejection across REST · GraphQL · MCP + the dashboard add-domain flow                     | 30m | t001, t002 |
| t004 | Render parity — collision + reserved rejection shape/semantics vs Render's "domain already in use"          | 30m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                 | 20m | t004       |
| t006 | Test coverage — meaningful tests for cross-App collision + reserved-host rejection                          | 30m | t004       |
| t007 | Closeout — DoD verified, milestone moved to `done/`                                                         | 15m | t006       |

## Definition of done

`AddDomain` rejects (typed 400-class error, not a 500) a hostname already registered on a **different** App, regardless of tenant — mirroring Render's "This domain already exists on another site." A host is also rejected when it matches a platform-owned name: the `BEX_BASE_DOMAIN` apex, the dashboard host, or a `<other-app>.<BEX_BASE_DOMAIN>` auto host that is not this App's own. The App's own auto host and a genuinely-free external custom domain still succeed. The rejection surfaces identically on REST, GraphQL, and MCP and renders as a validation message in the dashboard. External-domain ownership is **not** separately challenged — cert-manager's ACME already gates HTTPS serving on control of the domain (the same model Render relies on), so this milestone adds only the two guards ACME does not provide.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-12), re-scoped the same day after user pushback that Render performs no standalone ownership-verification step. Verified against Render docs 2026-07-12: Render's "Verify" is a DNS-resolution check, not a TXT ownership challenge; Render **does** block duplicate domains ("This domain already exists on another site. Please delete it from that site and try again."). Verified in bex 2026-07-12: `lego/backend/internal/apps/domains.go:207-235` (`AddDomain`) has **no** cross-App uniqueness check (only a same-App duplicate check at 218-221) and **no** reserved/platform-host guard; `platformHost` (`domains.go:66-73`) mints `<app>.<BEX_BASE_DOMAIN>` auto hosts that a custom host can currently shadow.
- **Goal linkage:** GOAL.md V0 #7 (security review) + pillar 1 (Render-compatible custom-domain flow — Render's duplicate-domain block); extends the w7 tenant-isolation track to the ingress/routing layer.
- **Expected outcome:** cross-tenant domain-collision (two Apps, undefined routing/cert outcome) is refused at add time like Render's, and the `*.onbex.co` / dashboard-host hijack — where platform-controlled DNS makes the ACME HTTP-01 challenge solvable, so a valid cert would issue for another App's platform subdomain — is closed. `docs/ADR018-render-parity.md` custom-domain row gains an accurate "duplicate-domain block" note.
- **Why now:** it is a **live gap** on already-shipped, prod-running code; the collision guard is cheap Render parity and the reserved-host guard closes a hijack cert-manager cannot prevent, best done before real multi-tenant custom domains exist to migrate.
- **Render parity: included** — t003 changes the add-domain error behavior on REST/GraphQL/MCP + dashboard, so t004 checks the collision/reserved rejection shape + message against Render's "domain already in use." The **standalone ownership-verification state machine originally proposed was dropped** (2026-07-12): Render has no such step, and cert-manager's ACME already proves external-domain control, so it was redundant — recorded here so it is not re-proposed.
