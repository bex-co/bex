# CLI sweep 801 — static-site headers/routes: bulk works, single-rule ops missing → filed w8/137 (live, 2026-10-20)

Disposable static site (`qa801-static`, sweep-780 recipe: `bex-co/bex`, `echo build`, publish `.`) created via CLI, probed via REST, rules cleared, site deleted (204, then GET 404). One bug filed: [w8/137](../../137.md).

- `POST …/headers` and `POST …/routes` → 405 `method not allowed`; `DELETE …/headers/{id}`, `DELETE …/routes/{id}`, `PATCH …/routes/{id}` → bare 404. Only `GET`/`PUT` are registered (`lego/backend/internal/apps/rest.go:1279-1282`); the pinned Render OpenAPI defines all five single-rule operations. No ADR018/gap-backlog/DO_NOT_DO entry records the absence — non-duplicate.
- Control on the same fixture: `PUT …/headers`/`PUT …/routes` → 200 with minted `hdr-…`/`rte-…` ids, `GET` returns the `{header|route, cursor}` envelopes, `PUT []` clears both.
- Two harness notes: `--plan` is refused for `static_site` by the CLI itself (`--plan is not supported for static_site`), and a REST create with `runtime:"static"` is refused by name — the CLI recipe is the supported create path.
