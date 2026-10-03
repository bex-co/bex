# w4 · m160 — Revalidate static files across deployments

**Worker:** worker4 **Goal:** returning visitors receive republished files at unchanged URLs **Status:** blocked — t001/t002/t004/t005 done 2026-10-03; t003 hosted parity and t006 closeout await deploy + QA

## Tasks (in order)

| id   | title                                            | est | depends_on |
| ---- | ------------------------------------------------ | --- | ---------- |
| t001 — **DONE** | Audit static response caching and rollout limits | 20m | — |
| t002 — **DONE** | Default static responses to revalidation         | 30m | t001 |
| t003 | Render parity                                    | 20m | t002       |
| t004 — **DONE** | Simplify                                         | 10m | t003 |
| t005 — **DONE** | Test coverage                                    | 30m | t003 |
| t006 | Closeout                                         | 15m | t004, t005 |

## Definition of done

Use a new qa-prefixed Free static site from public bex-co/bex, main, root `examples/static-site`, publish `.`, empty build command, auto-deploy Off, with no custom cache header. Start with a fresh browser cache under the fixed server policy.

- Fetch `/render.yaml` normally in a same-origin browser page, change Root Directory through Settings to `examples/hello-go`, confirm Save changes and wait for the new published revision. The next normal fetch of the same URL returns the hello-go manifest, matching an independent no-store fetch, without changing the URL or clearing the browser cache.
- Seed a separate stable query URL while hello-go is active, restore `examples/static-site`, wait for publication and reload the visitor page. A normal fetch of that same query URL returns the static-site manifest. GET and HEAD success responses without a custom override carry `public, max-age=0, must-revalidate`; the default HTML response remains revalidating.
- Save an explicit `/render.yaml` Cache-Control rule through Headers, reload and confirm its value survives in UI/REST/GraphQL/MCP and the public response. The observed revalidation override continues to fetch the new bytes after another root-directory republish. Already cached legacy immutable entries are explicitly documented as unaffected until eviction/expiry or a new asset URL/forced reload; do not claim server-side recall.
- Delete the owned fixture through the dashboard; its API/public URL return not-found and its App/owned tenant and build artifacts disappear. Revoke the QA session.

Custom domains, actual JS/CSS application execution, Firefox/Safari, explicit immutable override replay, rewrite cache policy and origin failures were not exercised live in sweep 43; t001/t005 own those regression checks, not claims of prior live success.

## Source + Goal linkage

- **Source:** user-requested continuous `$qa-find-bugs`, sweep 43, 2026-10-02 America/Los_Angeles (2026-10-03 UTC), muse.env, explicitly scheduled in w4. [Complete finding and durable probes](finding.md). Major stale-content defect; six tasks, 125m.
- **Goal linkage:** ADR008 reliable hosting; ADR029 atomic static publication and ADR018 static-site parity.
- **Expected outcome:** a successful static republish reaches returning visitors fetching unchanged URLs, while the server continues to reuse immutable revision objects.
- **Why now:** two live publish transitions served new bytes to uncached callers while the browser retained the previous successful revision; the explicit revalidation policy passed the same transition.
- **Render parity included:** public file delivery and existing header controls are tenant-facing. No new API/schema is needed. Preserve the DO_NOT_DO static CDN cache-purge exclusion; this changes response freshness, not origin-cache invalidation.
