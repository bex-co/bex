# w4 · m698 — `*` in a redirect or rewrite rule works anywhere in the path, as on Render

**Worker:** worker4 **Goal:** a static site's redirect/rewrite rule matches a `*` at any position of its source and expands a `*` at any position of its destination, as Render documents, instead of treating every `*` except a trailing `/*` as literal text and saving rules that never fire. **Status:** done (every task done; the production walk on pin `486e3f4afbf6` passed, 2026-11-27)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A route source's `*` matches any string from its position, without changing header path matching — **DONE** | 1h30m | — |
| [t002](done/t002.md) | A route destination's `*` expands to the captured string wherever it appears — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Tests, a walk on a fixture static site, and the ADR029/ADR018 record — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

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

**2026-11-27, unblocked (verified finding).** The gate held: the newest `chore(deploy): pin platform images to 486e3f4afbf6` on `origin/main` (`a6f9307d74`) carries `e3a10a9958` (`git merge-base --is-ancestor e3a10a9958 486e3f4afbf6` exits 0). No decision was involved and no scope changed. Next: t003 steps 1 and 2 on production as the QA user in `bex-canary` (add `/old-*` → `/new-*` as a redirect and `/guides/*` → `/*/index.html` as a rewrite to a static site there, a disposable one when none fits; `curl -I` shows `/old-page` 301 to `/new-page`, `/guides/<dir>` serving `<dir>/index.html` with 200, and `/old` and `/guides` matching neither rule; remove the rules or delete the disposable site), record the walk here, then t007.

## Production walk (t003 steps 1–2, 2026-11-27, pin `486e3f4afbf6`): PASS

Signed in as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`), which had no static site, and called REST with that session. **Fixture:** static site `qa-20261127-m698-st` (disposable, `srv-dc4m42eo8i4s738lh700`), `bex-co/bex` `main` with `rootDir` `examples` and `publishPath` `.`, so `/static-site/index.html` and `/blueprint-refs/site/index.html` are published and nothing sits at `/`. Its first deploy went live in 31 s. Before any rule, `/`, `/old-page`, `/guides/static-site`, `/old` and `/guides` answered 404. `PUT /v1/services/{id}/routes` with `/old-*` → `/new-*` (redirect) and `/guides/*` → `/*/index.html` (rewrite) answered 200 with both rules at priorities 0 and 1. The static server applied them within 10 s, with no deploy. `curl -I` against `https://qa-20261127-m698-st.onbex.co`:

| Path | Answer | Rule |
| --- | --- | --- |
| `/old-page` | 301, `location: /new-page` | redirect, `*` = `page` |
| `/old-a/b` | 301 to `/new-a/b` | redirect, `*` spans a `/` |
| `/old-` | 301 to `/new-` | redirect, empty capture |
| `/guides/static-site` | 200 `text/html`, 1548 bytes, byte-identical to `/static-site/index.html` and the repository's `examples/static-site/index.html` | rewrite |
| `/guides/blueprint-refs/site` | 200, byte-identical to `/blueprint-refs/site/index.html` | rewrite, `*` spans a `/` |
| `/old`, `/oldpage` | 404 | neither |
| `/guides` | 404 | neither (`/x/*` never matches the bare `/x`, w8/325) |

**Seen, not filed:** `/guides/` (an empty capture) answers 400 "invalid rewrite target". The expansion is `//index.html`, a protocol-relative URL, which `expandDestCaptures` (`lego/operator/internal/staticserver/staticserver.go`) refuses as non-local on purpose. Render's answer for this path is not captured (t004), and the DoD does not cover it. **Cleanup:** `PUT …/routes` with `[]` answered 200, `GET …/routes` read `[]`, and `/old-page` and `/guides/static-site` went back to 404. The fixture was then deleted (`DELETE` 204, then `GET` 404), and `bex-canary` lists only the 3 services it had before. This run's Kratos session was logged out (`qa-login.sh --logout` → `ok logged-out`) and its jar and scratch directory were removed.
