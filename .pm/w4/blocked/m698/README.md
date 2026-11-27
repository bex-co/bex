# w4 · m698 — `*` in a redirect or rewrite rule works anywhere in the path, as on Render

**Worker:** worker4 **Goal:** a static site's redirect/rewrite rule matches a `*` at any position of its source and expands a `*` at any position of its destination, as Render documents, instead of treating every `*` except a trailing `/*` as literal text and saving rules that never fire. **Status:** blocked (pin must carry `e3a10a9958`: t003's walk of `/old-*` → `/new-*` and `/guides/*` → `/*/index.html` on a static site in production, then t007; t001, t002, t004–t006 done, and t003's ADR029/ADR018 record shipped in `e3a10a9958`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A route source's `*` matches any string from its position, without changing header path matching — **DONE** | 1h30m | — |
| [t002](done/t002.md) | A route destination's `*` expands to the captured string wherever it appears — **DONE** | 45m | t001 |
| [t003](t003.md) | Tests, a walk on a fixture static site, and the ADR029/ADR018 record | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

Total: **7 tasks, about 4h20m**.

## Definition of done

- A rule source such as `/blog*`, `/*/feed` or `/*.html` matches the paths Render's rule says it matches, and `/x/*` still never matches the bare `/x` (w8/325). `:name` placeholders keep working, alone and next to a `*`.
- A destination such as `/new-*` or `/*/index.html` expands the captured string where the `*` is; a trailing `/*` behaves as today.
- Header path patterns keep Render's header rules exactly (`/*.css` matches `/tokens.css` but not `/assets/theme.css`; `/**/*.css` the reverse).
- On a fixture static site in production, `/old-*` → `/new-*` and `/*/index.html` rules redirect and rewrite as Render's docs describe. ADR029 and the ADR018 static-site row state the wildcard rule.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: static sites' redirects, rewrites and headers), item 2. First found in this session's earlier HTTP-edge proposal, which was never filed; re-checked by hand at `origin/main` `a4ce267dfd` before filing; no open, blocked or done item owns it.
- **Render:** [render.com/docs/redirects-rewrites](https://render.com/docs/redirects-rewrites) § Wildcards: "In *Source*, `*` matches _any_ string that appears starting at that position in the path." "In *Destination*, `*` applies the _entire string_ captured by the wildcard in *Source*."
- **bex today:**
  - `matchPattern` (`lego/operator/internal/staticserver/staticserver.go:898-909`) treats only `/*` and a trailing `…/*` as wildcards; any other pattern is an exact match.
  - The destination substitutes only a trailing `/*` (`:1022-1047`).
  - `validateRoutes` (`lego/backend/internal/apps/service.go:6835-6841`) only checks that a source starts with `/`, so such a rule saves on every surface and then never fires.
  - `matchHeaderPattern` (`staticserver.go:1095-1106`) falls back to `matchPattern`, and headers follow different, segment-bounded rules, so the change must stay on the route path.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and [ADR029](../../../../docs/ADR029-static-sites.md).
- **Expected outcome:** a static site moved from Render keeps its wildcard redirects; visitors get the page, not a 404.
- **Why now:** the failure is silent: the rule saves without error and simply never matches.
- **Render parity:** included: runtime behavior, plus route validation on REST, GraphQL, MCP and the dashboard.
