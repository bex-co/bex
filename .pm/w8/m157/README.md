# w8 · m157 — An emptied inbound IP list blocks a service, as on Render; an untouched service reads the allow-all rule

**Worker:** worker8 **Goal:** on a web service or a static site an explicitly empty inbound IP list blocks every public request, a list nobody has changed reads Render's allow-all default, and a write that omits the field changes nothing, on REST, GraphQL, MCP, Blueprints and the dashboard alike. **Status:** in progress (t001–t007 done and shipped; t008, the live replay on production, waits for the deploy that carries it)

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | One rule for the service layer: empty is saved as deny-all, allow-all is saved as no rule — **DONE**   | 45m | —          |
| t002 | Every writer follows it, and the plan gate treats an empty list as a restriction — **DONE**            | 45m | t001       |
| t003 | Every reader follows it: `[]` when blocked, the allow-all rule when untouched — **DONE**               | 30m | t001       |
| t004 | Dashboard: removing the last rule warns that all traffic is blocked; reset writes allow-all — **DONE** | 30m | t003       |
| t005 | Render parity — **DONE**                                                                               | 15m | t002, t003 |
| t006 | Simplify — **DONE**                                                                                    | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                               | 20m | t005       |
| t008 | Closeout (live replay on production)                                                                   | 30m | t007       |

## Definition of done

1. A web service with the rule `203.0.113.0/24`, then `PATCH {"serviceDetails":{"ipAllowList":[]}}`: `GET` reads `"ipAllowList": []`, and a public request answers 403 and keeps answering 403.
2. `PATCH` with a `0.0.0.0/0` rule: public requests answer 200 within seconds, over IPv4 and IPv6.
3. A new web service reads the allow-all rule and answers 200.
4. A `PATCH` without `ipAllowList` leaves a blocked service blocked.
5. A Blueprint with `ipAllowList: []` on a web service blocks it. The same Blueprint with the field removed keeps the rules it had.
6. A static site behaves the same in 1 to 4.
7. GraphQL `service { ipAllowListEntries }` and MCP `get_service` read the same three states as REST.
8. The dashboard's Networking editor shows the allow-all rule on an untouched service, and warns before saving an empty list.
9. No service that is open today is closed by the deploy that carries this.
10. Fixtures deleted afterwards.

## Design (decided 2026-11-11, w8 drain)

- **The empty state is saved as a rule nobody matches.** An explicitly empty list is stored as the single entry `255.255.255.255/32` (`core.DenyAllCIDR`), the placeholder the environment layer already projects for its own empty list (w4/m28). The operator needs no change and no new field: it already turns a non-empty list into the `-ip-allow` middleware. A new spec marker was the alternative; it was not taken because a marker the cluster's CRD does not know yet is dropped silently and the service stays open, while a stored rule fails closed.
- **Allow-all is saved as no rule.** A list holding only `0.0.0.0/0` and/or `::/0` stores nothing, so no middleware exists and both address families are admitted. Stored as written, a lone `0.0.0.0/0` (Render's default rule) would refuse every IPv6 client.
- **Reads.** A blocked service reads `[]`. A routable service with no stored rule reads `0.0.0.0/0` and `::/0`, the pair Postgres and Key Value read when public with no saved rules (`core.UnrestrictedAllowList`, w4/m315). Writing back what was read keeps access as it is.
- **Existing services are untouched.** A service stored with no rule stays open, whether it never had one or was emptied before this ships (finding item 7). Nothing is migrated.
- **Render's API, confirmed from Render's own Terraform provider** (`render-oss/terraform-provider-render`, `internal/provider/webservice/resource/internal/update.go`, read 2026-11-11): "In plan with empty list -> send empty array (block all)", "Not in state and not in plan -> send nil (don't update)", and the API default is `0.0.0.0/0` with the description `everywhere` (`internal/provider/common/ipcidr.go`, and a recorded response in its test cassettes). That settles two of the finding's unverified points for web services and static sites.

## Shipped 2026-11-11 (t001–t007)

- **Backend.** `core.ServiceAllowListToSpec` and `core.ServiceAllowList` are the one save and the one read; REST create and PATCH, GraphQL create and `setServiceIpAllowList`, MCP create and `update_service`, and Blueprint apply and export go through them. The plan gate (`requireServiceInboundRules`) now judges the spec a write would save, and an empty list counts as a restriction everywhere (`RestrictsInbound` lost its `emptyDeniesAll` switch: the service layer was its only `false`).
- **Dashboard.** An untouched service lists the allow-all rule. Removing the last rule shows "No rules: all inbound traffic to this service is blocked…" before Save. Below Scale the button reads "Allow all traffic" and writes the allow-all rule; it used to be "Clear all rules" and wrote an empty list.
- **Operator.** No change; a test pins that the saved placeholder becomes the service's middleware.
- **Found by review and fixed before commit:**
  - GraphQL `setServiceIpAllowList(id)` with neither `entries` nor `cidrs` was read as an empty list and would have blocked the service. It is refused now.
  - GraphQL `ipAllowList` and `ipAllowListEntries` read `[]` for a worker, cron job or private service, which now means blocked. They read `null` there.
  - The dashboard showed the blocked warning while the service was still loading.
  - On create, an empty list compared equal to "no list yet" and passed the plan gate below Scale (caught by the new test).

## Left as it is

- **A lone `0.0.0.0/0` saved before this shipped** is still enforced as written, so it refuses IPv6 sources, as it did. Any write of the allow-all rule replaces it with no rule; the dashboard offers that below Scale too. Not migrated, so that no existing service changes behaviour on deploy.
- **A Blueprint apply with no field-presence record** copies the whole desired spec, the list included (`applyCreateToSpec`). Every production path builds the record from the compiled manifest; only tests pass none.
- **The read default differs from Render in shape** (two ranges with empty descriptions, against Render's `0.0.0.0/0` / `everywhere`). ADR018's row records it as not signed off.
- **The pinned CLI cannot send an empty list** (`--ip-allow-list ""` sends nothing), as the finding notes. That is the client's; REST, GraphQL, MCP and Blueprints can.

## Source + Goal linkage

- **Source:** inbox note `w8/269` (promoted 2026-11-11), `/qa-find-bugs-cli` w8 loop sweep `bcc7`. Its finding follows.
- **Goal linkage:** Render parity ("If you delete all rules, Render blocks all inbound traffic to the affected service(s)!"), and a security control that fails open.
- **Expected outcome:** the documented way to take a service off the public internet works on bex.
- **Why now:** major. Taken ahead of the queue's order because the control fails open.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweep `bcc7`)

Why: Render gives a service one default rule, `0.0.0.0/0`, and treats a service whose rules were all deleted as closed: "Render blocks all inbound traffic". Its Blueprint spec says the same for `ipAllowList: []`. bex has no default rule and reads an empty list as "no rule". Sending `[]` to a locked-down service removes the restriction, and the service answers anyone within seconds. A user who follows Render's documented way to take a service off the public internet gets a fully public service. bex already uses Render's meaning for environments (w4/m28) and for Postgres and Key Value (w4/m315). The service layer is the one place where empty means open.

**Severity:** major (a security control fails open; it needs a Scale workspace to reach). **Attribution:** server (backend apps, operator ingress projection) and dashboard. **Estimate:** ~3h, milestone-sized. **Source:** `/qa-find-bugs-cli` w8 loop sweep `bcc7`, 2026-11-11 UTC, released `bex v0.5.0` (Render CLI v2.35.0), `https://api.bex.co/v1/`, production pin `f609163ada8e`, workspace `bex-canary` (plan `scale`), QA human device login.

### Evidence

A free image-backed web service. Times are UTC. The caller's address is outside `203.0.113.0/24`.

```text
$ bex services create --name qa-20261111-bcc7-ip --type web_service --image docker.io/traefik/whoami:v1.10.1 \
    --plan free --env-var WHOAMI_PORT_NUMBER=10000 -o json --confirm
$ bex services update <srv> --ip-allow-list cidr=203.0.113.0/24,description=qa-bcc7 -o json --confirm

19:38:29.2  saved; serviceDetails.ipAllowList = [{"cidrBlock":"203.0.113.0/24","description":"qa-bcc7"}]
19:38:44    GET https://qa-20261111-bcc7-ip.onbex.co/  → 403 Forbidden

PATCH /v1/services/<srv>  {"serviceDetails":{"ipAllowList":[]}}
19:38:44.4  → 200; the response has no ipAllowList key
19:38:45    GET → 403
19:38:47    GET → 200, and 200 on every poll to 19:39:08
```

- `GET /v1/services/<srv>` afterwards has no `ipAllowList` key. GraphQL `service { ipAllowList ipAllowListEntries }` reads `[]` and `[]`.
- The event feed records `ip_allow_list_changed` at 19:38:44.
- A service that never had a rule (`qa-20261111-bcc7-up`) also returns no `ipAllowList` key, so "never set" and "emptied" read the same.
- The pinned CLI cannot send the empty list. `bex services update <srv> --ip-allow-list ""` answers "Error: at least one update flag must be provided" and sends nothing. The request above is REST with the CLI's token.

Matching Render in the same sweep:

- A rule change took effect in 5 to 12 seconds (Render: "within a few seconds").
- A disallowed address got `403 Forbidden`, and the blocked requests appear in the request logs with status 403.
- A hibernated Free service answered a disallowed address 403 in 0.5 s and stayed hibernated.
- A `0.0.0.0/0` rule opened the service again.

### Expected

Render, [Inbound IP rules](https://render.com/docs/inbound-ip-rules):

- "If you haven't made any changes yet, you'll see a single default rule: `0.0.0.0/0` (allow all IPs)."
- "If you delete all rules, Render blocks all inbound traffic to the affected service(s)!"

Render, [Blueprint spec](https://render.com/docs/blueprint-spec), Inbound IP rules:

- "To block all external connections, set `ipAllowList` to an empty list:"
- When the field is omitted: "Render allows all IPs for a new service." and "Render retains the current IP ranges for an existing service."

So on bex, for a web service or a static site:

- An empty list blocks every request from the public internet with 403. Private-network traffic is not affected.
- A service nobody has changed reads the allow-all default, not an empty list.
- A write that omits the field changes nothing.

### Root cause

1. An empty write is stored as nothing. `SetIPAllowListEntries` sets `spec.ipAllowListEntries` to nil for an empty list (`lego/types/v1alpha1/ipallowlist_types.go:77-84`), and the field is `omitempty` (`lego/types/v1alpha1/app_types.go:981`). "Never set" and "explicitly empty" are one state.
2. The operator creates the service's `-ip-allow` Traefik middleware only for a non-empty list and deletes it otherwise (`reconcileIPAllowListMiddleware`, `lego/operator/internal/controller/app_controller.go:4166-4188`).
3. The environment layer already has Render's meaning. An explicitly empty environment list is projected as `core.DenyAllCIDR`, `255.255.255.255/32` (`lego/backend/internal/core/cidr.go:213-217`, `EnvironmentLayerCIDRs` `:246-249`), and a new environment is seeded with the allow-all pair (`DefaultEnvironmentAllowList`, `:233`). The service layer was left out on purpose: "on a service an empty list is no rule at all" (`RestrictsInbound`, `lego/backend/internal/core/plan_gate.go:156-167`), and `requireServiceInboundRules` passes `emptyDeniesAll: false` (`lego/backend/internal/apps/settings_check.go:115-121`).
4. Every writer and reader follows that:
   - REST decodes `[]` as a replace with nothing (`decodeIPAllowList`, `lego/backend/internal/apps/rest.go:554-565`) and leaves the key out of responses when the list is empty (`lego/backend/internal/apps/render.go:554-556`).
   - Blueprint apply copies the list the same way (`lego/backend/internal/apps/blueprint_plan.go:275-277`), and Blueprint export leaves an empty list out (`lego/backend/internal/apps/blueprint_generate.go:446-448`).
5. ADR018's row says "Empty list = open to all (Render's default)". Render's default is the `0.0.0.0/0` rule.

### Fix

1. Keep the explicit empty state on the App. `omitempty` drops an empty slice, so it needs a marker in the spec, set by an empty write and cleared by any non-empty one.
2. The operator projects `core.DenyAllCIDR` into the `-ip-allow` middleware for a marked service, as the environment layer does.
3. Reads (REST, GraphQL, MCP, Blueprint export): a marked service reads `[]`. A routable service with no rule and no marker reads the allow-all default, the way w4/m315 made Postgres and Key Value read what they admit.
4. Writes: `[]` marks the service on REST create and PATCH, GraphQL `setServiceIpAllowList`, MCP `update_service` and Blueprint apply. An omitted field changes nothing. A list holding only the allow-all pair stores no rule and no marker.
5. Plan gate: an empty list on a service now restricts, so `requireServiceInboundRules` passes `emptyDeniesAll: true`. Below Scale, clearing a legacy rule means writing the allow-all default.
6. Dashboard: the Networking editor's clear action writes the allow-all rule. Deleting the last rule warns that all traffic will be blocked before it saves.
7. Existing services: a stored empty list has no marker and stays open, so no service changes behaviour when this ships.
8. ADR018: correct the row's sentence about the empty list.

### Acceptance

- A service with the rule `203.0.113.0/24`, then `PATCH {"serviceDetails":{"ipAllowList":[]}}`: `GET` reads `"ipAllowList": []`, a public request answers 403, and a request from another service over the private network is still served.
- `PATCH` with a `0.0.0.0/0` rule: public requests answer 200 within seconds.
- A new web service reads the allow-all default and answers 200.
- A `PATCH` without `ipAllowList` leaves a blocked service blocked.
- A Blueprint with `ipAllowList: []` on a web service blocks it. The same Blueprint with the field removed keeps the rules it had.
- Below Scale, `[]` on a service with no rule is refused with the Scale sentence, and the allow-all rule is accepted.
- A hibernated Free service with an empty list answers 403 and stays hibernated.
- A static site behaves the same as a web service in each case above.
- Tests: the operator emits the deny-all middleware for a marked App; REST conformance covers the three read states (default, rules, empty).

### Unverified

- Render's side is from its docs; no Render service was run. The API reference describes neither a default value nor an empty array for `ipAllowList`. The Blueprint sentence sits in the general "Inbound IP rules" section, which covers web services, static sites, Postgres and Key Value, and does not name a resource type.
- What Render's REST API returns for a service nobody has changed: the default rule, or no field.
- On bex only the REST write was run. GraphQL, MCP, Blueprint apply and the dashboard editor were read in code. A static site was not run.
- Whether any production service has been emptied after holding a rule. Those services stay open under fix item 7, and their owners may believe otherwise.

### Dedupe

w7/m32 and w9/m56 (done) built the service allowlist. w4/m28 (done) gave environments the empty-means-deny-all rule and the seeded default. w8/m137 (done) added the Scale plan gate. w4/m315 (done) made Postgres and Key Value reads report what the endpoint admits. w1/m150 and w1/m171 (done) cover which address is matched and Cloudflare-proxied hosts. None changes what an empty list means on a service.

### Cleanup

The service was deleted; `GET /v1/services/<id>` answers 404.
