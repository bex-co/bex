# w3 · m38 — Per-session repo credentials: gateway-refreshed token, branch-confined (ADR047 D2)

**Worker:** worker3 **Goal:** agent sessions push to tenant repos with per-session GitHub App installation tokens that never land on disk in the sandbox — fetched on demand through a gateway-proxied credential helper, refreshed past the 1h TTL, and server-side confined to `bex-agent/*` branches. **Status:** todo (t001–t006 done; t007 awaits m39/m42/m41's production OpenSandbox proof)

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | bex-api mint verb: per-session installation token, repo-scoped `contents:write`, `bex-agent/*` confinement — **DONE** | 60m | —    |
| t002 | Gateway internal refresh endpoint (proxied authorized re-mint), audit-logged — **DONE**             | 45m | t001       |
| t003 | In-sandbox git credential helper (fetch-on-demand, never on disk) + image integration contract — **DONE** | 45m | t002       |
| t004 | Snapshot hygiene: scrub credentials + tenant key pre-snapshot; re-fetch on resume — **DONE**        | 30m | t003       |
| t005 | Simplify pass over the credential path — **DONE**                                                  | 20m | t004       |
| t006 | Test coverage: mint scoping, refresh authz, helper behavior, scrub — **DONE**                       | 45m | t004       |
| t007 | Closeout                                                                                           | 10m | t006       |

## Definition of done

- bex-api mints a per-session GitHub App installation token (ADR026 integration, `contents:write` added) scoped to the session's target repo, and only for sessions whose push target is a `bex-agent/*` branch — branch confinement is server-side policy at mint time, never in-sandbox enforcement.
- The isolated ssh-gateway exposes an internal-only endpoint (sandbox-exec listener pattern) that verifies the caller is the session's sandbox, proxies an authorized re-mint to bex-api, and audit-logs every issue — solving 1h TTL vs multi-hour sessions with no standing credential in the guest.
- A git credential helper inside the sandbox fetches the token on demand for clone/fetch/push; `git config` never stores it and no token string exists on the sandbox filesystem between git operations.
- Pre-snapshot scrub removes any credential material (composing with w3/m37 t004's key scrub); a resumed sandbox transparently re-fetches through the helper.
- Tenant guidance recommends branch protection on default branches (docs note, ADR047 D2).

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) D2 + gap 4; `/pm-brainstorm` decomposition 2026-08-01. Rides shipped w2/m8–m9 (GitHub App, `lego/backend/internal/github/client.go`) and the gateway (ADR035).
- **Goal linkage:** pillar 5 — sessions must push; this is also the answer `.pm/w3/008.md` asks for (credential brokering: tokens stay outside the guest at rest) applied to the GitHub leg.
- **Expected outcome:** multi-hour sessions clone and push without ever holding a standing credential; pushes are provably confined to `bex-agent/*`.
- **Why now:** ADR047 wave 1, parallel with m37 (integration contract: the helper's install path in the template image, agreed with w3/m37/t001). Render parity omitted: internal credential plumbing — no REST/GraphQL/MCP/UI surface change (the mint verb is internal-only, not a public API).
