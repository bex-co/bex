# w1 · m16 — Config surfaces beyond env vars: environment groups + secret files

**Worker:** worker1 **Goal:** Add the two Render config surfaces bex omits, both extending the existing OpenBao-backed `secrets` feature (no new store): **environment groups** (a named, shared var/secret-file set linkable to many services) and **secret files** (files mounted at `/etc/secrets/<name>`), with three-adapter + dashboard parity. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Environment groups — Core verbs + store/OpenBao model (CRUD + link/unlink) + projection       | 45m | —          |
| t002 | Env groups — REST (`/v1/env-groups`) + GraphQL + MCP surfaces + dashboard Env-Groups view     | 45m | t001       |
| t003 | Secret files — Core + `/etc/secrets` file-mount projection; REST/GraphQL/MCP + Environment UI  | 40m | t001       |
| t004 | Render parity — env-groups + secret-files across REST/GraphQL/MCP/UI vs render.com             | 20m | t002, t003 |
| t005 | Simplify — `/simplify` over what this milestone changed                                        | 20m | t004       |
| t006 | Test coverage — meaningful tests for groups + linking + secret-file projection                 | 30m | t004       |
| t007 | Closeout                                                                                       | 10m | t006       |

## Definition of done

A tenant can create an environment group, add vars + secret files to it, and link it to services (linked services receive the group's values, materialized into their `<name>-env` Secret); and can add per-service secret files mounted at `/etc/secrets/<name>`. Both are exposed over REST (Render's `/v1/env-groups` + `/v1/services/{id}/secret-files` shapes), GraphQL, MCP, and the dashboard Environment tab; parity checked vs render.com. `make test` + dashboard tests green.

## Source + Goal linkage

- **Source:** inbox note `w1/010` (m13 audit), the two ✖ rows in `docs/render-parity.md` ("Secret files", "Environment groups", → w1/m16).
- **Goal linkage:** pillar 1 (Render parity — configuration).
- **Expected outcome:** shared config stops being copy-paste per service; secret files (certs, service-account JSON) become possible.
- **Why now:** env-vars parity shipped (w4/m6); these are the adjacent config surfaces the audit flagged, and both reuse the OpenBao path already in place — cheapest parity wins in the config area.
- **Render parity INCLUDED:** this milestone adds REST/GraphQL/MCP surfaces + dashboard UI — the standing Render-parity task checks both surfaces against render.com's env-group + secret-file behavior.
