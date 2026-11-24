# w21 · m616 — A service's outbound IPs are a fixed, published set, as on Render: tenant egress leaves through Floating IPs, not through whichever node the instance runs on

**Worker:** worker21 **Goal:** an external allowlist built from `outboundIps` keeps admitting a service after the tenant pool adds, removes or replaces nodes, as a Render allowlist keeps admitting its region's ranges. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Correct ADR018's Service outbound IPs row and the 2025-09-24 changelog disposition; write the stable-egress ADR — **DONE** | 45m | — |
| [t002](t002.md) | Terraform two Hetzner Floating IPv4s in the tenant location and turn on Cilium's egress gateway | 1h | t001 |
| [t003](t003.md) | Egress gateway policy for tenant workloads, with the free-plan SMTP deny and the egress meter still correct | 1h | t002 |
| [t004](t004.md) | Failover: a lost gateway node's Floating IP moves to a standby, and affected TCP connections are reset | 1h30 | t003 |
| [t005](t005.md) | `outboundIps` answers the fixed set on REST, GraphQL, MCP and the dashboard | 45m | t003 |
| [t006](t006.md) | Production: egress from two nodes, and from a node added after the change, leaves from the set; a gateway failover recovers | 45m | t004, t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 40m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

## Definition of done

1. On production, `GET /v1/services/{id}/outbound-ips`, GraphQL `Service.outboundIps`, MCP `get_service_outbound_ips` and the dashboard card answer the same set before and after a tenant node is added or removed.
2. A service instance on any tenant node, including a node created after the change, reaches an external endpoint from an address in that set; so do a cron run and a pre-deploy Job.
3. Losing the gateway node restores egress from the same set without an operator, and the connections it carried are reset rather than left hanging.
4. Free-plan SMTP egress stays denied, and the egress meter still bills each service's outbound bytes.
5. ADR018's Service outbound IPs row and the 2025-09-24 changelog row describe what bex does, with the evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w21` (2026-11-23), filed by `/pmx them all to w21`, item 1. Found by listing Render's 227 docs pages (render.com/docs/llms.txt) against every `render.com/docs/<slug>` cited in `docs/`, `.pm/`, `lego/` and `dashboard/src`: `outbound-ip-addresses` and `outbound-connection-resets` were cited nowhere.
- **Render reference:** [outbound-ip-addresses](https://render.com/docs/outbound-ip-addresses), read 2026-11-23: "Render services send outbound traffic through specific sets of IP ranges depending on their region", "Each IP range uses CIDR notation", "Outbound IP ranges are shared across _all_ services in the same region"; the exception "For workspaces created before January 23, 2022, services in the Oregon region do _not_ use a fixed set of outbound IP addresses" shows that the default is a fixed set. [outbound-connection-resets](https://render.com/docs/outbound-connection-resets): rotation is "of in-use addresses in the default IP ranges", and Render "proactively resets affected TCP connections to public destinations". [Upcoming changes to outbound IP addresses](https://render.com/changelog/upcoming-changes-to-outbound-ip-addresses) (2025-09-24): new ranges from 2025-10-27, the original set retired "no later than December 1, 2025", so 33 and 68 days of notice.
- **bex today:** `Service.OutboundIPs` (`lego/backend/internal/apps/outboundips.go:49`) answers the `bex.co/pool=tenant` nodes' ExternalIPs, because "bex has no NAT gateway". ADR018's row marks it ✅ on the premise "Render's shared regional IPs drift the same way", which Render's pages above contradict, and `docs/render-artifacts/render-changelog.md` marks the 2025-09-24 entry `n/a`. On production on 2026-11-23, 8 of the 9 tenant nodes were less than 5 days old (read-only node listing), so the published set had almost entirely turned over within the week: an allowlist built from it a week earlier now refuses 8 of 9 nodes.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); hosting reliability (an external database's firewall is where most `outboundIps` readers put the list).
- **Expected outcome:** a Render user who allowlists a service's outbound IPs on Atlas, RDS or a partner API keeps working through bex's autoscaling and node rolls.
- **Why now:** signup has been open since 2026-11-21, so more external allowlists are being built from a list that changes without notice, and the ledger tells readers it is at parity.
- **Decision state.** The mechanism (Cilium egress gateway on Hetzner Floating IPv4s, a standby gateway node, a reassignment on loss) is this proposal's recommendation, not a user decision; it rests on ADR008 Render parity first. The set is shared by every tenant service, as Render's default ranges are; per-workspace dedicated outbound IPs stay a `.pm/DO_NOT_DO.md` non-goal and are not reopened. **Spend:** Floating IPs are billed monthly on the user's Hetzner account; creating them in t002 waits for the user's go-ahead, which is a gate on t002 only (t001 and the code in t003–t005 can ship before it).
- **Follow-up, not in scope:** announcing a change to the set ahead of time, as Render did with 33 days' notice, once there is a set to change.
- **Render parity:** included. `outboundIps` and the dashboard card change on REST, GraphQL, MCP and the UI.
