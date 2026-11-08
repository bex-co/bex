# w32 · m283 — Sandbox network policy as on Render: allow-all by default, plus an HTTPS hostname allow-list

**Worker:** worker32 **Goal:** a sandbox created without a policy can reach the internet as on Render, `--allow-egress-https` limits it to listed hostnames over HTTPS, and the platform floor (cluster, metadata, private ranges, other tenants) stays denied under every policy **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR042 and ADR018: user sandboxes default to allow-all; define allow-list | 1h | — |
| t002 | Enforce allow-all and allow-list per sandbox with Cilium policy | 2.5h | t001 |
| t003 | Accept allow-all and allow-list on create across REST, GraphQL and MCP | 1h | t002 |
| t004 | Live drill: each policy enforced on the mock cluster, then production | 1.5h | t003 |
| t005 | Render parity | 15m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- `bex sandboxes create` with no policy reports `allow-all`, and an exec in it reaches a public HTTPS host and a public non-HTTPS endpoint, but not cluster, metadata or private addresses.
- `bex sandboxes create --allow-egress-https api.github.com` reaches `api.github.com` over HTTPS and nothing else (other hosts, subdomains, non-HTTPS all fail).
- `--network-policy deny-all` is unchanged; agent-session sandboxes stay deny-all.
- REST, GraphQL and MCP accept and report the same policies; invalid rules answer named 400s.
- ADR042 and the ADR018 row describe the shipped behavior; the checklist rows for `--network-policy` and `--allow-egress-https` are `[x]`.
- Verified live on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w32, 2026-11-07, item 3. Render docs (render.com/docs/sandboxes-creating) make `allow-all` the default; bex's deny-all default and `allow-list` refusal (`docs/render-artifacts/ea-sandbox.md:48`, ADR018 § Sandbox create flags row `networkPolicy`) have no recorded user sign-off.
- **Goal linkage:** Pillar 5 (sandboxes, reopened 2026-07-27) and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** Render-written sandbox code that assumes internet access works on bex unchanged, and `--allow-egress-https` gives users real egress confinement instead of a refusal.
- **Why now:** It is an unsigned divergence on a default, so every create without a policy hits it; the Cilium FQDN/SNI machinery it needs already runs for sandboxes. Sequence after w2/m278's suspend work lands, since policy must survive suspend/resume.
- **Render parity:** Included: the milestone changes REST/GraphQL/MCP sandbox create and read. The reference is Render's sandbox contract; the dashboard is N/A (bex has no sandbox UI and Render documents none).
- **Divergent alternative:** Keeping deny-all as the default needs the user's sign-off recorded in ADR018; this milestone ships the parity default.
