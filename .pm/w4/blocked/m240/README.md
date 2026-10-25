# w4 · m240 — Static-site route placeholders (`:name`) match and expand, as Render documents

**Worker:** worker4 **Goal:** a static-site redirect or rewrite whose source names a path segment (`/blog/posts/:postid`) matches that segment and substitutes it into the destination (`/blog/:postid`), exactly as [Render documents](https://render.com/docs/redirects-rewrites). Today REST and the dashboard accept and store such a rule, and the static server then matches its source as a literal string, so the rule silently never fires. **Status:** blocked — t001–t006 done 2026-10-25 (loopx w4); t007 waits for the release pin to include the fix, then QA replays the DoD below on a fresh owned fixture in `bex-canary`

## Tasks (in order)

| id              | title                                                                      | est | depends_on                 |
| --------------- | -------------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Operator: named-segment capture and expansion in the static-server matcher | 45m | —                          | — **DONE**
| [t002](done/t002.md) | Refuse undeclared or duplicate placeholders on every rule writer           | 30m | w4/m240/t001               | — **DONE**
| [t003](done/t003.md) | Editor hint, ADR029 grammar and ADR018 note                                | 20m | w4/m240/t001               | — **DONE**
| [t004](done/t004.md) | Render parity                                                              | 20m | w4/m240/t002, w4/m240/t003 | — **DONE**
| [t005](done/t005.md) | Simplify                                                                   | 15m | w4/m240/t004               | — **DONE**
| [t006](done/t006.md) | Test coverage                                                              | 20m | w4/m240/t004               | — **DONE**
| [t007](t007.md) | Closeout (live DoD replay)                                                 | 15m | w4/m240/t005, w4/m240/t006 |

## Definition of done

Live, on an owned fixture in `bex-canary`, after the release pin includes the fix. Fixture shape as in the source run: public Git `https://github.com/bex-co/bex`, `main`, root `examples/static-site`, build command `echo "page2" > other.html`, publish path `.`.

- `PUT /v1/services/<id>/routes` with these five rules answers `200`: `[{type:redirect,source:"/old/*",destination:"https://example.com/new/*"},{type:rewrite,source:"/p2",destination:"/other.html"},{type:redirect,source:"/blog/posts/:postid",destination:"/blog/:postid"},{type:redirect,source:"/updates/:month/:year",destination:"/changelog/:year/:month"},{type:rewrite,source:"/item/:id",destination:"/other.html"}]`
- Then `curl -sD- <site><path>`:
  - `/blog/posts/my-post` → `301`, `location: /blog/my-post`
  - `/updates/03/2024` → `301`, `location: /changelog/2024/03`
  - `/item/7` → `200`, body `page2`
- Controls unchanged: `/p2` → `200` `page2`; `/old/x` → `301` `location: https://example.com/new/x`.
- `PUT …/routes` with `[{type:redirect,source:"/a/:x",destination:"/b/:y"}]` → `400` naming `routes[0]` and `:y`; GraphQL `setStaticRoutes` with the same rule returns the same refusal.
- Fixture deleted (`GET` → `404`); QA session revoked.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop run r7k2, 2026-10-22 UTC (`muse.env`, workspace `bex-canary` `tea-daif693dqjvc73e7as3g`, prod pin `b3cd5bc74de8`). Owned fixture `qa-20261022-r7k2-static` (`srv-dbd9u5o7tisc73d57040`), deleted in-run (`GET` → `404`); session revoked. Full probe, root cause and unverified surfaces in [finding.md](finding.md) — the probe itself is the durable artifact.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); static sites ([ADR029](../../../docs/ADR029-static-sites.md)); parity ledger ([ADR018](../../../docs/ADR018-render-parity.md)).
- **Expected outcome:** a `render.yaml` or dashboard rule copied straight from Render's docs behaves the same on bex; a placeholder typo is refused with a named error instead of emitting a literal `:name` path.
- **Why now:** severity major — every write surface reports success while the served behavior silently ignores the rule, so migrating Render users lose redirects with no signal. w4/m150 explicitly left "route placeholder expansion" out of scope (its README Dedupe line) and nothing filed it since. The Render parity closing task is included because the REST/GraphQL/MCP/Blueprint/dashboard rule surfaces and the served response all change.
