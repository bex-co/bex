# Render artifact — Maintenance mode

**Captured:** render.com/docs/maintenance-mode (docs-fallback fetch, 2026-07-15) + the pinned OpenAPI field-grep from the m37 brainstorm round (`maintenanceMode: {enabled: boolean, uri: string}` on `webServiceDetails`, POST/PATCH).

## What Render ships

- **Status code:** every request to a service in maintenance mode gets `503 Service Unavailable`.
- **Default page:** when `uri` is empty, Render serves its own built-in maintenance page. Docs don't describe its markup — bex is free to design its own.
- **`uri` semantics:** an **absolute URL to an external page**, fetched and served (not a redirect — the visitor's address bar stays on the service's own host). Render explicitly forbids pointing `uri` at the service that's in maintenance mode itself ("must not be a URL of the service in maintenance mode") and recommends a static site instead — the obvious reason: the service is intentionally unreachable, so self-referencing would be a fetch that can never succeed. If the fetch of a custom `uri` fails, Render returns _that_ failure to the visitor rather than silently falling back to the default page.
- **Host + domain scope:** applies to all public traffic — custom domains and the platform `onrender.com` subdomain alike (no per-host opt-out).
- **Service type scope:** web services only ("available only for paid web services" in Render's copy — the paid qualifier doesn't carry over to bex, which has no plan gate here). Not available for background workers, cron jobs, private services, or static sites.
- **Toggle location:** the service's Settings page, a switch in a "Maintenance Mode" section.
- **Interaction with other states:** undocumented by Render. bex defines its own (below) since the DoD requires it.

## bex parity decisions

| Decision | bex |
| --- | --- |
| Status code | `503`, matching Render |
| Default page | bex's own minimal branded page (no Render markup to mirror) |
| `uri` semantics | Fetched and served by the maintenance responder (not a redirect), matching Render. Must be an absolute `http(s)://` URL; validated at the API layer (t004/t005) — empty is valid (⇒ default page), garbage is a named 400. bex does not special-case "points at the app's own host" — a self-referencing `uri` is left to fail its own fetch and surface that failure, same as Render's documented behavior for any other unreachable `uri`. |
| Fetch failure | The responder returns the fetch error to the visitor (502) rather than silently falling back to the default page — matches Render |
| Host scope | Every host the App serves (platform subdomain + `spec.hosts` custom domains) — no per-host opt-out, matching Render |
| Type scope | `web_service` only. Non-web types are rejected at the API layer with a named error (`t005`); the CRD field itself is untyped like other optional `AppSpec` fields (consistent with `Autoscaling`, `SubdomainPolicy` etc. — type-scoping lives in bex-api, not in the CRD) |
| Responder mechanism | Reuse the activator (`lego/operator/cmd/activator`) — it already resolves App-by-host and serves a content-negotiated interstitial (HTML for browsers, JSON for API clients) for the auto-sleep wake case; maintenance mode is a second interstitial state on the same responder, not a third component. (The activator was previously built but never wired into `config/default`'s kustomization — both auto-sleep and maintenance mode were unroutable until this milestone fixed that, plus a hardcoded double-`bex-` name-prefix bug shared with the static-server component.) |
| SSRF protection on `uri` fetch | The activator's fetch client validates the resolved IP at dial time (`net.Dialer.Control`), blocking loopback/private/link-local/unspecified ranges (covers cloud-metadata `169.254.169.254`) and disabling redirect-following — a tenant-supplied `uri` can't be used to reach cluster-internal services from the activator's ServiceAccount-bearing pod. Render's own docs don't address this (SSRF isn't Render's problem to solve for its own infra the way it is for bex's shared activator). The identical, pre-existing gap on bex's outbound-webhook path was found but left unfixed (out of this milestone's scope) — filed in docs/ADR028-security-review.md's follow-up register |
| Suspend interaction | **Suspend wins.** A suspended App's Ingress is untouched by maintenance-mode routing (mirrors today's `Suspended` early-return in `reconcileKubernetes`, which already runs before any Ingress-backend decision) |
| Auto-sleep interaction | Maintenance mode **suppresses auto-hibernation** — `shouldAutoHibernate` returns `false` while `maintenanceMode.enabled`. Rationale: the DoD promises pods "keep running untouched" for the duration maintenance is enabled, not just at the toggle moment; letting the idle timer scale to 0 underneath an enabled maintenance window would mean disabling it wakes to a cold start, contradicting "without suspending it." If the App was already auto-hibernated when maintenance is enabled, it un-hibernates (pods scale back up) since the auto-hibernate condition no longer holds |
| Deploy interaction | Deploys proceed normally while `maintenanceMode.enabled` — the pre-deploy gate and Deployment rollout are unaffected by the maintenance flag; the maintenance page persists across the deploy (Ingress keeps pointing at the responder) until the flag is explicitly disabled |
| Dashboard | Settings section toggle + optional custom-page URL field, confirm dialog on enable (matches the suspend row-action precedent — a destructive-to-visibility action); unmissable banner on the service detail header while enabled |
