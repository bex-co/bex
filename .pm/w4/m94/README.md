# w4 · m94 — Preserve static assets and request-path headers under rewrites

**Worker:** worker4 **Goal:** Render-style static rules preserve existing files and apply custom headers to the visitor's request path. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Serve existing objects before redirect and rewrite rules | 45m | — |
| t002 | Match custom headers against the original request path | 25m | — |
| t003 | Audit shared handler behavior and origin failure classes | 35m | t001, t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 35m | t004 |
| t007 | Closeout | 15m | t005, t006 |

## Definition of done

- Create the disposable static fixture in t001. With the UI-suggested `/* → /index.html` rewrite saved, GET `/render.yaml` still returns the original 810-byte YAML file, while GET `/qa-route` returns the HTML fallback. Removing and restoring the rule, followed by a fresh page load and external curl, preserves both outcomes.
- With the two explicit redirect rules in t001, GET/HEAD `/render.yaml` returns the existing file with 200 and no Location, while `/qa-old` returns 301 Location `/index.html`.
- With the wildcard rewrite and t002's two headers saved, GET/HEAD `/qa-route` includes `X-QA-Path: request-path` and `X-QA-All: all-paths`. Turning the explicit rewrite off and on does not remove either header. The public response remains the same HTML page.
- The API retains the saved rules/headers and project/environment membership; fresh UI shows the same values. Delete the test service, environment and project and verify list/API absence.
- Complete shared-handler and sibling verification in t003/t006. The live hunt did not exercise origin errors, security headers, other tenants, custom domains, or built JS/CSS assets.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs w4`, 2026-09-06, loop pass 3. Two major findings reproduced on dashboard.bex.co and the public onbex.co URL. Exact probes in t001/t002; verified local screenshots `.playwright-mcp/qa-static-spa-rules-1.png` and `qa-static-headers-1.png` are gitignored, so durable evidence is in the task text.
- **Goal linkage:** ADR008 Render-compatible hosting; ADR029 static serving and ADR018 static-site parity; ADR006 shared API contracts.
- **Expected outcome:** the documented SPA setup no longer substitutes HTML for published files or silently removes request-path header rules.
- **Why now:** the dashboard recommends this exact wildcard rewrite. Its interaction with existing objects and path-scoped headers was missing from the original independent rule/header tests.
- **Render parity included:** [Render explicitly serves existing resources before evaluating rules](https://render.com/docs/redirects-rewrites), and [header patterns match request paths](https://render.com/docs/static-site-headers). Public primary documentation checked 2026-09-06; no authenticated Render deployment was created.
- **Dedupe:** searched open/done work for existing file/asset precedence, rewrite/header and request-path terms, scanned open milestones/inbox notes across workstreams and DO_NOT_DO. w1/done/m21 shipped individual rules/headers and its TestSPAFallbackRoute only tests a missing path; these interactions were never covered, not a measured recurrence of an earlier fix. w5/done/m57 and w5/done/m79/t022 are UI layout/skeleton work. w3/m46's deliberate implicit extensionless SPA fallback remains valid and is preserved. Main 4e6bd00ff retains both code paths; history traces rule-first serving to original 84e221650, with no pending fix. The buildless-static wizard/image work in w8/m32 is unrelated.
