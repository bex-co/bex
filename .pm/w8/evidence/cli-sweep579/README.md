# CLI sweep 579 — static-site header/route rules: bulk PUT contract, IDs, enforcement

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI, production pin `d98d07f62`, 2026-10-19. Owned fixtures: `qa579-static` (`srv-dbb89f7o17is73furu6g`, `static_site`) and the earlier wrong-type probe `qa579-routes` (`srv-dbb83jvo17is73furu40`, web service). Probe scripts `/tmp/qa501/routes579.py` and `/tmp/qa501/routes579_static.py`. Both fixtures deleted at close: REST detail 404, headers subresource 404, both public URLs 404, zero `qa579` names in the service list.

- **Static-only scope:** on the web-service fixture, bulk header/route PUTs were refused as static-only; on the real `static_site` the same payloads succeeded. Individual POST/PATCH/DELETE rule mutations return 405/404 — these are documented omissions in [ADR018](/docs/ADR018-render-parity.md), not findings. The supported contract is collection replacement via bulk PUT (`SetRoutes`/`SetHeaders` in `apps/service.go`, registered in `apps/rest.go`).
- **Bulk PUT works and is enforced:** canonical header and route sets were accepted, re-read identically, and live HTTP checks confirmed header injection, redirect, and rewrite behavior on the public URL.
- **Invalid rules refused atomically:** malformed route/header payloads returned named 400s and left the previously stored set untouched (verified by re-read after each refusal).
- **IDs stable, duplicates allowed:** re-PUT of the same set returned identical rule IDs. Duplicate rules are accepted with distinct occurrence IDs — contract behavior, since ADR018 derives rule IDs from service, content, and occurrence ordinal. Reordering preserved content IDs while priority followed the new index.
- **Cleanup semantics:** final empty PUTs (`[]`) cleared both header and route sets before deletion.

## Notes / non-findings

- Source-equals-destination rewrites (`same579`) and self-referencing redirects (`/loop579`) were accepted at write validation. No enforcement contradiction was observed (existing-file precedence applied), so these were not filed; write-time cycle/reflexive validation remains unprobed against the pinned Render contract.
- Priority is positional (index-derived) rather than an explicit field; matches the collection-replacement model.

## Unrun

- Header/route rules on a static site behind a custom domain.
- Interaction of rewrites with SPA fallback / 404-page precedence beyond existing-file checks.
- Rule-count and path-length ceilings.
