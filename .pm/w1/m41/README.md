# w1 · m41 — Stable Traefik LB origin IP across rebuilds

**Worker:** worker1 **Goal:** the production Traefik load balancer's public IP survives cluster teardown/rebuild — a Terraform-owned Hetzner LB the CCM adopts by name — so DNS never points at an IP Hetzner has returned to the pool. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Design/capture: Terraform-owned LB + CCM adopt-by-name      | 30m | —          |
| t002 | Implement: Terraform LB + `load-balancer.hetzner.cloud/name` annotation | 45m | t001       |
| t003 | Prove rebuild-survival + prod adoption runbook              | 45m | t002       |
| t004 | Move the FUTURE-MAYBE entry to its Done section             | 15m | t003       |
| t005 | Simplify                                                     | 30m | t004       |
| t006 | Test coverage                                                | 45m | t004       |
| t007 | Closeout                                                     | 15m | t006       |

## Definition of done

Deleting and recreating the app cluster's Traefik Service (and, proven at least once, the cluster itself on the rebuild path) re-attaches the same named Hetzner LB and public IP; the adoption steps for the live prod LB are documented and executed (or scheduled with the operator's sign-off); `.pm/FUTURE-MAYBE.md`'s "Traefik LB survives cluster rebuilds" entry moves to Done.

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Traefik LB survives cluster rebuilds" — trigger ("the next planned rebuild/DR drill") read as fired by `w7/m29`'s executed ADR031 restore drills + `w1/m19`'s ongoing rebuild work; **user confirmed the trigger 2026-07-15** (round-12 materialization).
- **Goal linkage:** platform reliability (GOAL.md de-risking); protects every tenant's DNS at once.
- **Expected outcome:** the 2026-07-10 incident class (LB IP released to strangers while DNS still points at it) is structurally closed.
- **Why now:** rebuilds are an active reality on this substrate (m19/m19.1/m36 all rolled infrastructure); each one re-rolls the dice on the origin IP. **Render parity closing task omitted** — pure infra; no REST/GraphQL/MCP/UI surface.
