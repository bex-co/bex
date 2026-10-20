# CLI sweep 676 — registry credentials: REST create/update unreachable (live, 2026-10-19)

Filed as [w8/135](../../135.md). Fixture hygiene: nothing created by the REST probes; the GraphQL-created credential `rgc-dbbblhavfpec73dm0k9g` was deleted via REST DELETE (204) and list-verified empty on REST and GraphQL.

## Refusal walk (REST `POST /v1/registrycredentials`)

| Body | Result |
| --- | --- |
| `{}` | 400 `invalid request body at /name` |
| `{"name","registry":"docker.io"}`-style guesses | 400 `invalid request body at /registry` |
| `registry=DOCKER` (valid Render enum) | 400 `invalid request body at /ownerId` |
| + `ownerId`, missing username | 400 `invalid request body at /username` |
| username present, `password` instead of `authToken` | 400 `invalid request body at /authToken` |
| bex shape `{"name","host":"ghcr.io","username","authToken"}` | 400 `invalid request body at /registry` |
| full Render shape `{"name","registry":"DOCKER","ownerId","username","authToken"}` | 400 bare `bad request` |

`PATCH /registrycredentials/{id} {"name":…}` → 400 `invalid request body at /registry`.

## Working surfaces (same afternoon)

- `GET /registrycredentials` → 200 `[]`; `GET …/{id}` → 200 (no secret field); `DELETE …/{id}` → 204.
- GraphQL `createRegistryCredential(name:"qa676-gql", host:"ghcr.io", username, authToken)` → 200 with id `rgc-dbbblhavfpec73dm0k9g`, secret never echoed.

## Root cause (source)

The pinned-Render OpenAPI request validator (`lego/backend/internal/api/render_openapi.go`, `renderRequestValidator`) requires Render's `registry` enum body for `create-registry-credential`/`update-registry-credential`, while the handler deliberately generalizes to `host` (`lego/backend/internal/registrycreds/rest.go:27-36`, struct at :71) and strict-decodes. No `renderRequiredCompatibility`/`renderBodyPropertyExtensions` entry bridges them, so each dialect is refused by the other layer.
