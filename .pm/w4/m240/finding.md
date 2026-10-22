# Finding — route placeholders are stored, then matched as literals (run r7k2, 2026-10-22)

**Severity:** major. **Fixture:** `qa-20261022-r7k2-static` (`srv-dbd9u5o7tisc73d57040`), public Git `https://github.com/bex-co/bex` `main`, root `examples/static-site`, build `echo "page2" > other.html`, publish `.`. Deleted in-run.

## Probe

`PUT /v1/services/srv-dbd9u5o7tisc73d57040/routes` (cookie session, `fetch(..., {credentials:'include'})` from the dashboard page):

```json
[
  { "type": "redirect", "source": "/old/*", "destination": "https://example.com/new/*" },
  { "type": "rewrite", "source": "/p2", "destination": "/other.html" },
  { "type": "redirect", "source": "/blog/posts/:postid", "destination": "/blog/:postid" },
  { "type": "redirect", "source": "/updates/:month/:year", "destination": "/changelog/:year/:month" },
  { "type": "rewrite", "source": "/item/:id", "destination": "/other.html" }
]
```

→ `200`, all five stored verbatim (placeholder rule ids `rte-hbrp0v38tpsipcocndqs`, `rte-me86va6rlcljps6p0jjq`, `rte-fm1jqmdlbcq4m6vnu8p1`).

45 s later, `curl -sD- https://qa-20261022-r7k2-static.onbex.co<path>`:

| Path                  | Got                               | Render documents                     |
| --------------------- | --------------------------------- | ------------------------------------ |
| `/blog/posts/my-post` | `200` index.html                  | `301` → `/blog/my-post`              |
| `/updates/03/2024`    | `200` index.html                  | `301` → `/changelog/2024/03`         |
| `/item/7`             | `200` index.html                  | `200` other.html (`page2`)           |
| `/blog/posts/:postid` | `301` `location: /blog/:postid`   | (literal source matched as a string) |
| `/p2` (control)       | `200` `page2`                     | same                                 |
| `/old/x` (control)    | `301` `https://example.com/new/x` | same                                 |

The controls prove the rule set was applied; only `:name` fails. Render's examples are from render.com/docs/redirects-rewrites § Placeholders (`/blog/posts/my-post → /blog/my-post`, `/updates/03/2024 → /changelog/2024/03`).

The `200` index.html on a miss is a separate, already-fixed behavior: `28c849cd0` ("answer a static-site miss with 404 and its 404.html") is on `main` but not in pin `b3cd5bc74de8` (deploy lag). After it deploys, the placeholder misses read `404`, not `200`, until this milestone lands.

## Root cause

- `lego/operator/internal/staticserver/staticserver.go:721-738` `matchPattern` supports `/*` and a trailing `/foo/*`; "any other pattern is an exact match", so `:postid` is a literal.
- `expandDest` (`staticserver.go:749-790`) substitutes only `:splat` and a trailing `/*`; named tokens stay literal.
- `matchRoutes` has exactly two callers (`grep -n 'matchRoutes(' lego/operator` → `staticserver.go:317` GET/HEAD miss path, `:949` non-GET external rewrite). Header selection (`matchHeaderPattern`) is a separate grammar and is not affected.
- `lego/backend/internal/apps/service.go:5295` `validateRoutes` accepts any `/`-rooted source, so no writer warns. ADR029 (`docs/ADR029-static-sites.md:132-134`) documents only `/*` and `:splat`.

## Target behavior

- A source segment `:name` (`[A-Za-z_][A-Za-z0-9_]*`, the whole segment) matches exactly one non-empty path segment; it may combine with a trailing `/*`.
- Each destination `:name` the source declares is replaced by the captured, decoded segment, through the same escape-once path as `:splat` (w4/m154 invariants: captured `?`/`#`/`%` stay path data; configured query/fragment literal).
- `:splat` keeps its meaning; a source with no `:name` behaves byte-for-byte as today; rule order and existing-file precedence are unchanged.
- Writers (REST, GraphQL, MCP, Blueprint `routes:`, dashboard) refuse a destination `:name` the source does not declare, and a duplicate name in one source, with `400` naming the rule index and token.
- Already-stored rules that would newly fail validation keep serving (decide and record whether reads flag them).

## Unverified this run

- Blueprint `routes:` apply path — reasoned to reach the same static-server matcher, not probed.
- Placeholders inside an external (`http(s)://`) destination.
- Render's handling of a placeholder in a destination's query string.
