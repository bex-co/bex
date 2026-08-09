# w7 · m79 — Surface unsatisfiable service dependencies instead of letting them time out silently

**Worker:** worker7 **Goal:** when a service cannot start or cannot be reached because something it depends on is missing — an unreadable linked-datastore Secret, a blocked network path, an absent public route — the user is told which one, instead of watching a health check fail with no explanation. **Status:** done 2026-08-09

## Tasks (in order)

| id   | title                                                                | est | depends_on               |
| ---- | -------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Detect + condition an unresolvable env dependency (missing Secret) — **DONE**    | 45m | —                        |
| t002 | Project the reason onto the deploy + service-event surfaces — **DONE**           | 45m | w7/m79/t001              |
| t003 | Surface "exposed but unroutable" as its own condition — **DONE**                 | 35m | w7/m79/t001              |
| t004 | Render parity: failure reasons + event surfaces — **DONE**                       | 30m | w7/m79/t002, w7/m79/t003 |
| t005 | Simplify the code this milestone changed — **DONE**                              | 25m | w7/m79/t004              |
| t006 | Test coverage for the shipped signals — **DONE**                                 | 35m | w7/m79/t004              |
| t007 | Closeout — **DONE**                                                              | 15m | w7/m79/t005, w7/m79/t006 |

> **Premise corrected mid-milestone.** t003 was written believing production had a routing *bug*. `w7/m78/t001` disproved that: nothing fails — production runs `BEX_BASE_DOMAIN` unset by deliberate security decision (`w7/m54`), so a service with no custom domain correctly gets no Ingress and no URL. m78 folded in here, and t003 was rewritten to surface a *configuration state* with an actionable cause rather than to report a fault. That correction is the milestone's most useful outcome: the behavior was right all along, and only the silence was wrong.

## Definition of done

Given a web service whose linked-datastore Secret does not exist in its namespace, the platform reports **that specific reason** — not a generic health-check failure — on the App status, in the deploy's failure reason, and on the surfaces a user actually reads (REST/GraphQL/MCP and the dashboard). The same holds for a service marked exposed that has no route.

Each signal is pinned by a test asserting the reason is specific and correct, and asserting it does **not** fire when the dependency is satisfied.

## Source + Goal linkage

- **Source:** production bug report 2026-08-08, §8 item 3 ("Surface failures. The deploy showed only 'Update Failed' with a healthy pod; the missing-secret / network-drop / missing-ingress conditions had no user-visible signal"). Independent of `w7/m77`/`w7/m78`, which remove the specific causes; this milestone makes the class visible if it ever recurs.
- **Goal linkage:** observability (`docs/ADR010-observability.md`) and the service-event lifecycle facts (`docs/ADR052-notifications.md`, whose feed already carries `service_event_facts`). Also directly serves the AI-native pillars: an agent operating bex through MCP cannot diagnose an unexplained health-check timeout any better than a human can.
- **Expected outcome:** the incident's defining property — "each missing step is invisible until the pod hits it" — stops being true. A user or agent reading the deploy learns which dependency is unsatisfied on the first failure rather than the third.
- **Why now:** the 2026-08-08 incident cost hours of one-crash-at-a-time reconstruction precisely because there was no signal, and the blast radius of `w7/m77`'s root cause is still unknown for the same reason — no one can tell how many tenants are affected without inspecting each. Building the signal now also means `m77`'s live cutover has something to verify against.
- **Render parity task included:** yes — deploy failure reasons and service events are Render-compatible REST/GraphQL/MCP surfaces with a dashboard presentation.

## Notes

Scope discipline: this is about **reporting** unsatisfied dependencies, not repairing them. Do not add auto-healing (creating missing Secrets, opening network paths) here — `w7/m77` removes the causes; auto-repair would mask a future one.

## Outcome (2026-08-09)

Both halves of the DoD hold, and both turned out to be about **carrying a diagnosis the platform already had** rather than producing a new one:

- **Unresolvable dependency.** `stuckPodMessage` recognised crash loops and image-pull failures but not `CreateContainerConfigError` — the one state where the pod never starts, so neither existing diagnosis applies. One case added; the reason then flows through `failureReasonFor` into `deploys.failure_reason` and out to REST, GraphQL, MCP, the dashboard deploy header, and (newly) the failure email. Before this, that path ended in the literal string "the deploy did not become healthy within the health-gate window" while the operator held the exact missing object.
- **No public address.** A new `PublicRouting` condition on the CRD contract, surfaced as `publicRoutingNotice` across REST/GraphQL/MCP and rendered in the service-detail header **in the slot the URL would occupy** — previously empty, which is exactly how "will never be reachable" came to look like "still starting".

Three things worth carrying forward:

1. **The negatives were the hard part.** Five by-design exclusions for the routing condition, failure-only gating for the email, and a classification test in both directions for the failure list. A reporting feature fails by crying wolf, and a signal people ignore is worse than the silence it replaced.
2. **`lint` caught what the tests could not** — the dashboard's `ServiceView` is a hand-maintained type separate from the generated GraphQL types, so the component compiled against a field its prop type lacked while vitest, which does not typecheck, passed clean.
3. **No migration was needed.** `service_event_facts` has CHECKed closed vocabularies for both fact type and reason code; the deploy's free-text `failure_reason` already reached every surface the DoD names, so the vocabulary stayed closed and the change stayed additive.

Verification: backend and operator suites 0 failures, `make lint-backend` 0 issues, dashboard services 344/344 with no lint errors outside the pre-existing stale-`node_modules` agent-sessions cascade. Anti-tautology confirmed on both operator changes.
