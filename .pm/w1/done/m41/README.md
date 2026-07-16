# w1 · m41 — Stable Traefik LB origin IP across rebuilds

**Worker:** worker1 **Goal:** the production Traefik load balancer's public IP survives cluster teardown/rebuild — a Terraform-owned Hetzner LB the CCM adopts by name — so DNS never points at an IP Hetzner has returned to the pool. **Status:** done (2026-07-15)

## Tasks (in order)

| id   | title                                                       | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Design/capture: Terraform-owned LB + CCM adopt-by-name — **DONE** | 30m | —          |
| t002 | Implement: Terraform LB + `load-balancer.hetzner.cloud/name` annotation — **DONE** | 45m | t001       |
| t003 | Prove rebuild-survival + prod adoption runbook — **DONE**    | 45m | t002       |
| t004 | Move the FUTURE-MAYBE entry to its Done section — **DONE**   | 15m | t003       |
| t005 | Simplify — **DONE**                                         | 30m | t004       |
| t006 | Test coverage — **DONE**                                    | 45m | t004       |
| t007 | Closeout — **DONE**                                         | 15m | t006       |

## Definition of done

Deleting and recreating the app cluster's Traefik Service (and, proven at least once, the cluster itself on the rebuild path) re-attaches the same named Hetzner LB and public IP; the adoption steps for the live prod LB are documented and executed (or scheduled with the operator's sign-off); `.pm/FUTURE-MAYBE.md`'s "Traefik LB survives cluster rebuilds" entry moves to Done.

## Closeout evidence

- Version-pinned source inspection and the deployed image both identified hcloud CCM `v1.33.0`: name fallback imports pre-existing LBs and `EnsureLoadBalancerDeleted` skips API-protected objects. ADR002 records the ownership split, first-adoption flow, rollback, and the pinned version's stale Service-UID-label nuance.
- An isolated real-Hetzner proof created protected temporary LB `7208582`, then two wholly distinct kind clusters (`m41-a` and `m41-b`) reconciled it from distinct Service UIDs. Both received the same LB ID and IPv4; a matching-UID Service deletion logged `ignored: load balancer deletion protected`. Both clusters and the temporary LB were removed.
- Commit `4dd753af` shipped the declaration. Main workflow [`29468203706`](https://github.com/bex-co/bex/actions/runs/29468203706) imported the existing production LB `7115248`, changed only `delete_protection: false -> true` in place, and reported `0 added, 0 destroyed`.
- The live production proof deleted `traefik/Service/traefik`: Argo recreated it in about six seconds with UID `1810f9d3-f820-4579-a222-0759c69562a2` → `1fa1ac1b-be2d-4cd2-9b9f-9a7ce5aa96b7`; CCM logged the protected-delete skip and name adoption. LB ID `7115248`, IPv4 `49.12.20.236`, IPv6 `2a01:4f8:c01e:3d1f::1`, all ports/targets, and the API origin's HTTP 200 remained stable; Argo returned `Synced:Healthy`.
- Manual drift run [`29468296349`](https://github.com/bex-co/bex/actions/runs/29468296349) finished with `No changes. Your infrastructure matches the configuration.` Terraform 1.10.5 `fmt`/`validate` and the full GitOps renderer passed locally; main's docs, GitOps, and infra workflows passed.
- Simplification audit kept the durable Terraform resource deliberately narrow: Terraform owns identity/protection and leaves labels, targets, listeners, and network attachment computed for CCM. The permanent first-adoption guard is also the safe remote-state recovery path; no one-off ID is committed.

## Source + Goal linkage

- **Source:** `.pm/FUTURE-MAYBE.md` "Traefik LB survives cluster rebuilds" — trigger ("the next planned rebuild/DR drill") read as fired by `w7/m29`'s executed ADR031 restore drills + `w1/m19`'s ongoing rebuild work; **user confirmed the trigger 2026-07-15** (round-12 materialization).
- **Goal linkage:** platform reliability (GOAL.md de-risking); protects every tenant's DNS at once.
- **Expected outcome:** the 2026-07-10 incident class (LB IP released to strangers while DNS still points at it) is structurally closed.
- **Why now:** rebuilds are an active reality on this substrate (m19/m19.1/m36 all rolled infrastructure); each one re-rolls the dice on the origin IP. **Render parity closing task omitted** — pure infra; no REST/GraphQL/MCP/UI surface.
