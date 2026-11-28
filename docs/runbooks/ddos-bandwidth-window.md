# Runbook: take a DDoS attack's bandwidth off a workspace's bill

**What it is:** "As part of Render's DDoS protection, Render does not bill for bandwidth usage incurred from a DDoS attack" ([render.com/docs/faq](https://render.com/docs/faq), read 2026-11-27). bex keeps that promise through an **attack window**: a bex operator declares one for a workspace, and every bandwidth hour it overlaps leaves the bill. Only the control-plane API on bex-api's `:8091` has the verb, behind `BEX_CP_TOKEN`; no customer surface shows a window, as none of Render's does. Design: [ADR023 § Bandwidth served to a DDoS attack is not billed](../ADR023-usage-metering.md#bandwidth-served-to-a-ddos-attack-is-not-billed-w4m709). Board: `w4/m709`.

Declare the window as soon as the attack is recognised. bex sends an hour's bandwidth to Stripe 48 hours after the hour, so a window declared within two days of the attack keeps every hour it covers off the invoice by itself. A later window credits the hours Stripe already rated on the workspace's next invoice.

## 1. Recognise the attack

Any one of these is enough to declare a window; write which one in the `reason`.

- **The service's request rate.** In Grafana, the request rate of the service's routers (`sum(rate(traefik_router_requests_total{router=~"<namespace>-<app>-.*"}[5m]))`) jumps far above its usual level, and its request log in Loki (`{namespace="<namespace>", app="<app>", type="request"}`) shows the requests spread across many client addresses. A tenant's own traffic peak (a launch, a load test it ran) is not an attack: ask the customer when in doubt.
- **A Hetzner DDoS notice** naming the load balancer's address in the window.
- **The customer's report**, with the times and the service, checked against the request rate above.

Note the attack's start and end in UTC and, when it hit some services and not others, their ids (`srv-`, `dpg-`, `red-`). A window covers whole hours: one that starts at 03:15 takes the whole 03:00 hour.

## 2. Reach the control-plane API

From a shell with the production app cluster's kubeconfig (`bash scripts/fetch-app-kubeconfig.sh <path outside the repo>`), in a history-disabled shell so the token stays out of it:

```bash
kubectl -n bex-system port-forward svc/bex-api 18091:8091 &
export BEX_CP_URL=http://127.0.0.1:18091
export BEX_CP_TOKEN="$(kubectl -n bex-system get secret bex-control-plane -o jsonpath='{.data.token}' | base64 -d)"
WS=tea-…   # the workspace
```

Never print the token.

## 3. Declare, extend, withdraw, list

Declare (omit `serviceIds` to cover every service of the workspace):

```bash
curl -sS -X POST "$BEX_CP_URL/v1/tenants/$WS/egress-attack-windows" \
  -H "Authorization: Bearer $BEX_CP_TOKEN" -H 'Content-Type: application/json' \
  -d '{"startsAt":"2026-11-27T03:15:00Z","endsAt":"2026-11-27T09:00:00Z",
       "serviceIds":["srv-…"],"actor":"you@bex.co","reason":"router request spike from 4k sources"}'
```

The answer (`201`) is the window with its `eaw-` id. The verb refuses, writing nothing, with a `400` whose `error` names the rule:

| Refusal | Meaning |
| --- | --- |
| `startsAt must be RFC3339` / `endsAt must be RFC3339` | A time is not RFC3339 (`2026-11-27T03:15:00Z`). |
| `startsAt must be before endsAt` | The span is empty or reversed. |
| `a window lasts at most 7 days` | Declare a longer attack as consecutive windows. |
| `endsAt is more than 24 hours from now` | Declare an attack still under way up to now and extend it as it goes on. |
| `startsAt is more than 35 days ago` | Too old for the hourly rows to be relied on: credit it by hand in Stripe, as a negative invoice item on the workspace's Subscription. |
| `actor and reason are required` | Both are recorded on every change. |
| `… is not a service of workspace …` | A service id the workspace does not own and bex never metered for it. |

An unknown workspace or window answers `404`.

Extend a live window (either end; an extension only widens it):

```bash
curl -sS -X PATCH "$BEX_CP_URL/v1/tenants/$WS/egress-attack-windows/eaw-…" \
  -H "Authorization: Bearer $BEX_CP_TOKEN" -H 'Content-Type: application/json' \
  -d '{"endsAt":"2026-11-27T15:00:00Z","actor":"you@bex.co","reason":"attack still running"}'
```

To narrow a window or change its services, withdraw it and declare the new one. Withdraw (the row stays, with who withdrew it and why):

```bash
curl -sS -X DELETE "$BEX_CP_URL/v1/tenants/$WS/egress-attack-windows/eaw-…" \
  -H "Authorization: Bearer $BEX_CP_TOKEN" -H 'Content-Type: application/json' \
  -d '{"actor":"you@bex.co","reason":"the customer ran a load test"}'
```

A withdrawal puts the window's hours not yet sent to Stripe back on the bill. **It reverses nothing already done:** hours already sent as 0 stay at 0, and a credit already recorded stays on the invoice. Undo those by hand in Stripe if you must.

List the workspace's windows, withdrawn ones included, each with every declaration, extension and withdrawal and its actor and reason:

```bash
curl -sS "$BEX_CP_URL/v1/tenants/$WS/egress-attack-windows" -H "Authorization: Bearer $BEX_CP_TOKEN"
```

## 4. Check the result

- **The export rows.** `GET $BEX_CP_URL/v1/tenants/$WS/billing-export?start=<RFC3339>&end=<RFC3339>` lists each hour with `attackHour: true` inside the window. A pending row ships with `billedQuantity: 0`; an emitted one keeps what it billed and is in the credit's reach.
- **The usage read.** The workspace's `GET /v1/usage` (or the Billing page) no longer counts the window's bytes in `egress_bytes` or `includedBandwidth`, and the service's Outbound Bandwidth footer leaves them out too. The service's bandwidth chart still shows the traffic as served.
- **A credit.** For hours Stripe already had, the credit job runs hourly. Its log line reads `billing: credited <workspace> <cents> cents for <bytes> bytes of <YYYY-MM> attack bandwidth`, and the workspace's upcoming invoice in Stripe carries a negative line "Outbound bandwidth from a DDoS attack, not billed (_Month YYYY_)". It covers the attack's own billed bytes and the later hours the attack pushed above the included bandwidth. A credit under one cent waits until there is more to credit.
- **A free workspace spun down by the attack** resumes on the bandwidth sweeper's next pass (within five minutes) once the window takes the bytes out of its month.
