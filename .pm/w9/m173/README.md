# w9 · m173 — CI stability: deploys stop stalling on an overloaded self-hosted runner host

**Worker:** worker9 **Goal:** main deploys land within one run of a push again; image builds, SBOM and CVE steps finish inside their budgets on the existing runner host. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Diagnose the 2026-11-05 deploy failures and measure the runner host | 45m | — |
| t002 | Bound build concurrency and thread use | 60m | t001 |
| t003 | SBOM and CVE steps reuse work and fit their budgets | 60m | t001 |
| t004 | Deploy latency watchdog reports stalls early | 30m | t002, t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 45m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- No deploy fails from thread exhaustion or SBOM timeouts across 3 consecutive main pushes; an unchanged image skips SBOM regeneration; a deploy gap over the threshold raises an alert.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** platform reliability: fixes reach customers; ADR008 operations.
- **Expected outcome:** pushed fixes deploy within one run instead of waiting hours (m168 waited ~10 h).
- **Why now:** repeated stalls 2026-11-04/05 delayed production fixes; other sessions are already patching symptoms piecemeal.
- **Parity:** omitted — CI/infra only, no tenant surface.
