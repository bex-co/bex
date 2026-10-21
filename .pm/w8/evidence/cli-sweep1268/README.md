# CLI sweeps 1268–1269 — Render schema defaults against bex create and read (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, HEAD-built pinned CLI, human device-login grant.

The pinned Render OpenAPI's create-body descriptions that state a default or an "if omitted" rule were listed: `dockerfilePath` `./Dockerfile`, static `publishPath` `public`, `numInstances` 1, the web plan `0.5c-512mb`, `region` `oregon`, route `priority` last, disk `sizeGB` 1, previews `off`. Each was compared with bex.

- **Static `publishPath`** filed as [w8/153](../../153.md). `bex services create --type static_site --repo https://github.com/mdn/beginner-html-site --branch main -o json --confirm` without `--publish-directory` → exit 1 in 7.1 s, `400 bad request: publishPath is required for a static_site`. A by-name list returned `[]`, so no fixture was created. The pinned CLI omits the field, and Render defaults it to `public`.
- Not re-tested in this sweep: `dockerfilePath`, `numInstances`, route priority and disk size. The plan default (free rather than `0.5c-512mb`) and region (`fsn1`) are recorded divergences; region is w8/152.

Also noted: deploy run 39530428130 for `e1f0e472f` stayed `pending` for 40+ minutes. `gitops (render)` failed because the runner could not download Helm. Both are CI-runner network issues, not product findings.

## Sweep 1269

Read-only API probes, with no fixtures:

- `GET /v1/services?ownerId=…` filters: `name` is exact and case-sensitive (`hello` and `HELLO-GO` → `[]`); repeated `name` ORs; `env=docker` matches; the non-Render `runtime` parameter → 400 `unsupported query parameter` (recorded strict-query behavior). Nothing new.
- Route priority: `AddRoute` appends when `priority` is omitted, matching Render's "Defaults to last". The `PATCH …/routes/{routeId}` response wraps the route in `headers`, which is what Render's schema literally specifies.
- Docker read defaults: the pre-existing `hello-go` (read-only GET) reports `dockerfilePath: ""`. Render reports `./Dockerfile`, and w8/m128 already defaults the sibling `dockerContext` to `.`. Filed as [w8/154](../../154.md).
