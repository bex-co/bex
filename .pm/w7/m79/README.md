# w7 · m79 — Surface unsatisfiable service dependencies instead of letting them time out silently

**Worker:** worker7 **Goal:** when a service cannot start or cannot be reached because something it depends on is missing — an unreadable linked-datastore Secret, a blocked network path, an absent public route — the user is told which one, instead of watching a health check fail with no explanation. **Status:** todo

## Tasks (in order)

| id   | title                                                                | est | depends_on               |
| ---- | -------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Detect + condition an unresolvable env dependency (missing Secret)    | 45m | —                        |
| t002 | Project the reason onto the deploy + service-event surfaces           | 45m | w7/m79/t001              |
| t003 | Surface "exposed but unroutable" as its own condition                 | 35m | w7/m79/t001              |
| t004 | Render parity: failure reasons + event surfaces                       | 30m | w7/m79/t002, w7/m79/t003 |
| t005 | Simplify the code this milestone changed                              | 25m | w7/m79/t004              |
| t006 | Test coverage for the shipped signals                                 | 35m | w7/m79/t004              |
| t007 | Closeout                                                              | 15m | w7/m79/t005, w7/m79/t006 |

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
