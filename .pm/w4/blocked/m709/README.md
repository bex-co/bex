# w4 · m709 — Bandwidth served to a DDoS attack is not billed, as on Render: an attack window a bex operator declares takes its hours off the invoice

**Worker:** worker4 **Goal:** a workspace whose service was flooded pays nothing for the bandwidth the flood drew, as Render promises: "As part of Render's DDoS protection, Render does not bill for bandwidth usage incurred from a DDoS attack" ([faq](https://render.com/docs/faq), "What does Render bill for?" › Outbound bandwidth, read 2026-11-27). **Status:** blocked (pin must carry `6e59881e00`; t001–t004, t006–t008 done; t005 has its dev-4 half, its production half and t009 wait for the pin)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | ADR023 states the rule; the ADR018 DDoS row and an operator runbook point at it — **DONE** | 25m | — |
| [t002](done/t002.md) | Attack windows: table, store and the control-plane verb that declares, extends and lists them — **DONE** | 50m | t001 |
| [t003](done/t003.md) | Every bandwidth read that bills or counts against the allowance leaves a window's hours out — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | Hours already sent to Stripe come back as a credit on the invoice — **DONE** | 45m | t003 |
| [t005](t005.md) | Live: a flooded service's hours leave the bill on dev-4, and the verb answers on production | 40m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 5h10m**.

## Decision (2026-11-27, `/loopx w4` worker, promoting w4/744)

How bex recognises attack traffic, picked by Render's behavior as the worker rules require (ADR008 § Render parity first), not a user decision:

- **Render** never bills an attack's bandwidth and does not say how it recognises one. Its faq, [ddos-protection](https://render.com/docs/ddos-protection) and [outbound-bandwidth](https://render.com/docs/outbound-bandwidth) pages (all read 2026-11-27) name no rule, form or support step. Its hosts sit behind Cloudflare (ADR018 Custom domains § Automatic DDoS protection), so a flood Cloudflare absorbs never reaches the origin that counts the bytes.
- **bex today** bills every byte Traefik answered (`egress_bytes`, `lego/backend/internal/egressquery/query.go`, [ADR023 § Complete outbound-bandwidth contract](../../../docs/ADR023-usage-metering.md#complete-outbound-bandwidth-contract-w8m15)). A volumetric flood that Hetzner's network filter drops never reaches Traefik and is already unbilled. An application-layer flood is served and billed, because nothing in front of tenant hosts mitigates it ([w21/126](../../w21/blocked/126.md)).
- **The rule bex adopts:** a bex operator declares an **attack window** for a workspace, optionally narrowed to some of its services, from the attack's evidence (a Hetzner DDoS notice, a request-rate spike from many sources on the service's routers, or the customer's report). Every bandwidth hour the window overlaps is left off the bill: it sends Stripe 0, uses none of the plan's included bandwidth, and counts toward no free-plan spin-down. An hour already sent is credited back (t004). The usage rows themselves keep the served bytes, so the window can be withdrawn and the evidence stays.
- **Why whole hours:** usage is recorded per hour (`usage_hourly`), and an attack's share of an hour cannot be told apart from the rest. Leaving the whole hour off errs toward the customer, and Render bills none of an attack's bandwidth.
- **Not gated on w21/126:** that decision (sign off the missing application-layer mitigation, or front tenant hosts with Cloudflare Enterprise) changes which floods reach the meter, not whether a recognised attack's bytes stay off the invoice. If Cloudflare fronts tenant hosts later, the flood it absorbs is unbilled with no window, and windows cover what still gets through. Recognising an attack automatically (an edge rate limit's drops, an anomaly alert) stays with w21/126 and is not part of this milestone.

## Definition of done

- An operator declares a window over the control-plane API (`POST /v1/tenants/{id}/…`, bearer-guarded, never on the public `:8090` mux) with a start, an end, an optional service list, an actor and a reason. It can be extended or withdrawn, and listing a workspace's windows shows each with its actor and reason.
- For every `egress_bytes` hour the window overlaps, whatever the resource kind (App, public Postgres, public Key Value):
  - the Stripe meter event carries 0, when the hour had not been sent yet;
  - the plan's included bandwidth is unchanged by it (`UsageBefore`), so later hours bill as if it never happened;
  - the free-plan bandwidth spin-down (`lego/backend/internal/bandwidth`) does not count it, and a workspace spun down only by those bytes resumes on the next sweep;
  - an hour already sent is credited on the workspace's next invoice, once per window, however often the credit runs.
- `GET /v1/usage`, GraphQL `usage`, MCP `get_usage`, the Billing page and a service's Outbound Bandwidth footer (`monthToDateBandwidth`) show the bandwidth that bills, with the window's bytes left out. The service's bandwidth chart keeps showing the traffic as served.
- Monthly compaction (`usage_monthly`) keeps a window's hours out of the month's billed total.
- [ADR023](../../../docs/ADR023-usage-metering.md) states the rule and the [ADR018 Custom domains DDoS row](../../../docs/ADR018/custom-domains.md) points at it. The row stays ◐ only for automatic mitigation (w21/126).
- On dev-4 a disposable service's served bytes inside a declared window bill 0 and leave the usage read. On production the verb declares, lists and withdraws a window on `bex-canary` (the QA user's workspace), and its bill reads as before afterwards.

## Source + Goal linkage

- **Source:** promoted from [w4/744](../done/744.md) on 2026-11-27 by `/loopx w4`. That note came from w4/m688 t002, a full read of Render's faq on 2026-11-27.
- **Evidence (`origin/main` `b0474cf448`):** nothing in the bandwidth billing path excludes anything but a billing-excluded workspace. That path is `billing/emitter.go` (`billedAboveAllowance` → `owedAbove`, rows sealed `DefaultSealHours` = 48 h after their hour), `store/billing.go` (`UsageBefore`), `store/usage.go` (`monthUsageByWorkspace`, used by `EgressBytesByWorkspace`, which unions `usage_hourly` with `usage_monthly`) and `bandwidth/bandwidth.go` (spins a workspace down at `u.Quantity >= included` and resumes it when that no longer holds). The control-plane mux already carries the billing operator verbs (`store/api.go`: `billing-override`, `billing-excluded`, `billing-export`).
- **Goal linkage:** Render parity (billing and invoicing), [ADR008](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a flood no longer grows a customer's bandwidth bill or spins down a free workspace's services, and the rule a support answer cites is written in ADR023.
- **Why now:** bex has no application-layer mitigation in front of tenant hosts (w21/126 is parked on the user), so a flood is billed today in every workspace. The 48-hour seal means a window declared within two days of an attack never reaches Stripe, so the operator verb is worth most before the first such incident.
- **Render parity:** included. The rule changes what REST, GraphQL, MCP and the dashboard report as billed bandwidth. Render exposes no attack window on any surface either, so there is no customer verb to match.
