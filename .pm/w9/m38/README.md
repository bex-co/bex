# w9 · m38 — One error dialect: every error path speaks Render's shape

**Worker:** worker9 **Goal:** Every non-2xx response bex-api emits carries `Content-Type: application/json` and Render's `message` key — the second, bare-`{"error"}`/text-plain dialect is retired. **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Route auth-gate 401/503 + GraphQL decode 400 through `core.WriteErr`                   | 30m | —          |
| t002 | Route metrics 400s + deploy-hook 405 through `core.WriteErr`; sweep remaining sites    | 40m | —          |
| t003 | Render parity                                                                           | 30m | t001, t002 |
| t004 | Simplify                                                                                | 20m | t003       |
| t005 | Test coverage                                                                           | 30m | t003       |
| t006 | Closeout                                                                                | 15m | t005       |

## Definition of done

Every error response from bex-api is JSON with `Content-Type: application/json` and a `message` field: the unauthenticated 401, the auth-upstream 503s, the GraphQL body-decode 400, the four `/v1/metrics/*` param 400s, and the deploy-hook 405 all match `core.WriteErr`'s `{"id","error","message"}` shape; a regression test asserts content type + `message` on the 401/400/405 paths; a grep for `http.Error(`/bare-`{"error"}` writers in `lego/backend` handler code comes back clean (or each survivor is annotated why).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — mechanical-consistency mining (error-shape angle); all sites verified by reading handlers: `internal/api/auth.go:140,151,303`, `internal/api/server.go:740`, `internal/metrics/rest.go:78,82,107,111`, `internal/deploys/deployhook.go:270`.
- **Goal linkage:** Render API compatibility (docs/ADR006-bex-api.md "one core, thin adapters") — the error shape is part of the wire contract; Render clients (incl. the official CLI) key on `message`.
- **Expected outcome:** unauthenticated/malformed requests stop returning `text/plain` bodies without `message` to Render clients; one error dialect platform-wide.
- **Why now:** the auth-gate path fires on every bad-credential request — the highest-traffic wire-format defect left after the CLI checklist's fifteen root-cause fixes.
- **Render parity:** included — error shape is a REST/GraphQL surface change and the whole point; t003 also smoke-checks the official CLI's login-failure rendering.
