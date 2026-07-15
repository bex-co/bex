# w3 · m14 — Live deploy following: land on the deploy, watch it build

**Worker:** worker3 **Goal:** Render's live deploy loop: creating a service or triggering a Manual Deploy lands the browser on the in-flight deploy's page, where build-log lines stream live and the status flips without a refresh. **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Capture Render's live deploy-following behavior → `docs/render-artifacts/`                            | 30m | —                      |
| t002 | Backend: extend `FollowLogs`/SSE to `type=build` — stream the `bld-<name>-gen-<N>` Job pod live       | 45m | —                      |
| t003 | Deploy detail page: in-flight auto-refresh + build pane switches to the live tail                     | 45m | t002                   |
| t004 | Create wizard + Manual Deploy land on the in-flight deploy's detail page                              | 30m | —                      |
| t005 | Render parity — same tail semantics across REST SSE/GraphQL/MCP where exposed; compare Render; refresh ADR018 | 20m | t003, t004             |
| t006 | Simplify — `/simplify` over the code this milestone changed                                           | 20m | t005                   |
| t007 | Test coverage — tail routing/honesty + dashboard in-flight behavior                                   | 30m | t005                   |
| t008 | Closeout — DoD met → move milestone to `done/`                                                        | 10m | t007                   |

## Definition of done

Creating a git-sourced service in a real browser lands on its first deploy's detail page, where build-log lines appear live while the build runs and the deploy status flips to live without a manual refresh; Manual Deploy behaves the same; `type=build` on `GET /v1/logs/subscribe` streams the running build Job's logs and keeps the w3/m8 honesty rules (what can't be served is refused by name, never silently widened).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` rounds 8–9. Verified 2026-07-15: the SSE live tail covers app pods only (`type=build` is store-query-only), the deploy detail page's build pane is a Loki history query (w5/m29), and `services.new.tsx:503` navigates to the service page, not the in-flight deploy. All ingredients shipped: w9/m1 page, w7/m28 build logs, w2/m5+m10 deploy verbs.
- **Goal linkage:** Render deploy-UX parity (pillar 1); GOAL.md #2 observability — the live half of the logs surface is w3's own `internal/logs`.
- **Expected outcome:** the deploy loop feels like Render's: no polling the checklist of panes to see whether a build is doing anything.
- **Why now:** the last ingredient (build logs in the store, w7/m28) landed 2026-07-14; w3's queue is empty. (The round-8 "deploy started" toggle task is dropped — `w3/005` was disposed in the interim.)
- **Render parity closing task: included** — REST SSE surface + dashboard UI change.
