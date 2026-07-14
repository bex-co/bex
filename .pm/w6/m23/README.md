# w6 · m23 — Custom domains: www↔apex sibling pairing

**Worker:** worker6 **Goal:** Adding a custom domain pairs its www/apex sibling the way Render does — built honestly on a real public-suffix list — and the collision guard covers the sibling, closing w7/m6's documented per-host limitation. **Status:** todo

## Tasks (in order)

| id   | title                                                | est | depends_on |
| ---- | ---------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's pairing behavior live               | 30m | —          |
| t002 | Registrable-domain helper on `publicsuffix`          | 30m | t001       |
| t003 | Auto-pair sibling on add/delete per the capture      | 45m | t002       |
| t004 | Collision guard covers the sibling                   | 30m | t003       |
| t005 | DNS instructions for both records                    | 30m | t003       |
| t006 | Dashboard: paired-domain display                     | 30m | t005       |
| t007 | Render parity                                        | 30m | t004, t006 |
| t008 | Simplify                                             | 30m | t007       |
| t009 | Test coverage                                        | 45m | t007       |
| t010 | Closeout                                             | 15m | t009       |

## Definition of done

Adding `foo.com` (or `www.foo.com`) to a service produces the sibling behavior Render's capture documents (auto-added domain or redirect — whichever the evidence shows), with DNS instructions for both records; registering `www.foo.com` on service A now blocks `foo.com` on service B (and vice versa) with the same 409 the per-host guard uses; multi-label subdomains (`app.foo.com`) and public-suffix apexes (`foo.co.uk`) behave correctly via the PSL. The ADR018 domains row's "no www↔apex pairing" divergence is closed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 2, 2026-07-14 (item 3); `docs/ADR018-render-parity.md` Custom domains row ("omitted rather than faked — no public-suffix list") + `lego/backend/internal/apps/domains.go`'s own comment. Conscious reopen of a documented omission WITH the missing mechanism (`golang.org/x/net/publicsuffix`), per DO_NOT_DO's gap-analysis rule.
- **Goal linkage:** Render parity (custom domains); w6 takes it as cross-workstream capacity placement per the m19–m22 precedent (topical sibling w7/m6 — whose per-host limitation this closes — has a full queue).
- **Expected outcome:** the domains surface stops surprising users who expect Render's www/apex behavior, and the cross-app collision guard stops having a documented blind spot.
- **Why now:** w6 is down to two open milestones; the collision-guard half is a real guard gap (a squatter can today register the sibling of someone else's domain). Render parity task included — all-surface + UI change.
