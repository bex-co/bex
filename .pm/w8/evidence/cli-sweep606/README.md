# CLI sweep 606 — `restart` dialects: by-name, suspended, static, synthetic

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixtures `qa606-web` (`srv-dbba2kabpdss73ci0ncg`, free hello-app) and `qa606-static` (`srv-dbba31qbpdss73ci0ngg`, static from the example repo), both deleted: REST 404, zero `qa606` names.

- **By id on a live service:** `"… restarted successfully"`, exit 0.
- **By name fails — upstream contract, not a bug:** `bex restart qa606-web` → `failed to restart resource: unknown resource type`, exit 1. The verb is documented as "Restart a service by resource ID"; the pinned client derives the resource type from the ID prefix (`pkg/resource/service.go:220,244` in the pinned module) and a bare name has none. The unmodified Render CLI behaves identically.
- **Suspended service:** restart → `409 conflict: service "…" is suspended`, and the service stayed suspended (URL 503, state unchanged). Resume afterward worked normally.
- **Synthetic id:** `404 (NOT_FOUND): not found`.
- **Static site before its first deploy:** restart → `409 conflict: service "…" has no live deploy to restart; deploy it first`. Suspend/resume on the static site both returned 202.
- **Create dialects found on the way:** `--plan` on a static create fails client-side (`--plan is not supported for static_site`); omitting the publish directory fails server-side with `400 publishPath is required for a static_site`; `--publish-directory .` succeeds.

## Unrun

- Restart of a Postgres/Key Value resource id (`dpg-…`/`red-…` prefixes route through the same verb).
- Restart during an in-progress deploy (queued/update states).
