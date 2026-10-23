# w8 · m130 — Free web services cannot receive private-network traffic, as on Render

**Worker:** worker8 **Goal:** A free web service is reachable only through its public `onbex.co` URL (and custom domains), as on Render. Other services in the workspace can no longer reach it at its internal address. Paid web and private services keep their internal address. **Status:** blocked — t001, t002, t003, t004, t005, t006 done 2026-10-23; GitOps syncs deny-free-plan-private-ingress and the release pipeline deploys the operator, the backend and the dashboard; QA runs t007: free web A and free probe B — B's wget to A's internal address fails and A still serves its public URL; upgrade A and B's wget succeeds, downgrade blocks it again; paid web/private controls stay reachable; delete fixtures by ID with 404 proofs

## Tasks (in order)

| id   | title                                                                       | est | depends_on                 |
| ---- | --------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Platform policy: deny tenant-to-free-web ingress, keep edge/activator/probes | 60m | —                          |
| t002 — **DONE** | Plan changes: upgrade opens the internal address, downgrade closes it        | 30m | w8/m130/t001               |
| t003 — **DONE** | Surfaces: internal address, Blueprint `fromService` and Connect copy on Free | 45m | w8/m130/t001               |
| t004 — **DONE** | Render parity: ADR018/ADR041/ADR022 and every surface agree                  | 20m | w8/m130/t002, w8/m130/t003 |
| t005 — **DONE** | Simplify the changed code                                                   | 15m | w8/m130/t004               |
| t006 — **DONE** | Test coverage: free vs paid ingress, plan changes, edge paths untouched     | 30m | w8/m130/t004               |
| t007 | Close out the milestone                                                     | 30m | w8/m130/t005, w8/m130/t006 |

## Definition of done

Live on production after GitOps syncs the platform policy and the release pipeline deploys the operator, the backend and the dashboard. Owned fixtures are deleted by ID with 404 proofs:

- A free web service A (`docker.io/ealen/echo-server:0.9.2`) and a free probe web service B in the same workspace. B's loop `wget -T 5 http://<A-slug>:80/privprobe` fails, and A's app logs show no `/privprobe` request. `curl https://<A-slug>.onbex.co/` still returns 200, and A still sleeps and wakes through the public activator.
- Upgrading A to a paid plan makes B's `wget` succeed after the rollout; downgrading A blocks it again.
- A paid private service and a paid web service keep receiving private traffic from B (control).
- REST/GraphQL/MCP, Blueprint validation and the dashboard Connect menu describe a free web service's internal address the same way (see t003).

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 8004, 2026-10-22/23 UTC: bex v0.3.3 (`bex-cli/v0.3.3` darwin-arm64), pin Render CLI v2.27.0 `a764810a7682`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login, non-TTY JSON. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). [render.com/docs/free](https://render.com/docs/free) lists, among what Free web services do not support: "Receiving private network traffic". [ADR018](../../../docs/ADR018-render-parity.md)'s Internal address row (line 103) records no free-plan divergence and no user sign-off. w4/252 (`done/252.md`, "bex deliberately exposes and serves its internal address while awake (ADR041)") predates the 2026-10-17 parity rule. Precedent: w8/m127 restricted free egress to match the same Render page.
- **Expected outcome:** free behavior matches Render's. A free web service cannot act as an always-on internal backend for paid services, and tenants get one documented rule.
- **Why now:** major. This is a customer-visible parity divergence and a free-tier pricing hole. The free-plan pod label (`app.bex.co/free-plan`, w8/m127) already exists, so the policy needs no new operator labeling. Render parity task included: REST/GraphQL/MCP and dashboard surfaces show the internal address.

## Evidence (sweep 8004)

Fixtures, all owned by this run and deleted by ID: A = `srv-dbdaqcvvlr1c73fj19ug` (`qa-20261022-8004-echo`, free, echo-server, `PORT=80`), B = `srv-dbdbdla8ohec73f9fuj0` (`qa-20261022-8004-probe`, free, `docker.io/library/busybox:1.37`, `PORT=10000`).

B was created with `bex services create --type web_service --name qa-20261022-8004-probe --image docker.io/library/busybox:1.37 --plan free --env-var PORT=10000 --confirm -o json`. The pinned CLI refuses `--start-command` on an image service, so its Docker command was set with `PATCH /v1/services/srv-dbdbdla8ohec73f9fuj0` → 200:

```json
{"serviceDetails":{"envSpecificDetails":{"dockerCommand":"/bin/sh -c 'mkdir -p /tmp/w; echo ok > /tmp/w/index.html; (while true; do if wget -q -T 5 -O /tmp/r http://qa-20261022-8004-echo:80/privprobe-8004; then echo PRIVOK; else echo PRIVFAIL; fi; sleep 15; done) & exec httpd -f -p 10000 -h /tmp/w'"}}}
```

`bex deploys create srv-dbdbdla8ohec73f9fuj0 --wait --confirm -o text` → `Deploy dep-dbdbdn28ohec73f9fukg succeeded`, exit 0. Then:

```text
$ bex logs -r srv-dbdbdla8ohec73f9fuj0 --type app --limit 20 -o text | grep PRIV
2026-10-23 01:23:39  PRIVOK
2026-10-23 01:23:24  PRIVOK
2026-10-23 01:23:20  PRIVOK
2026-10-23 01:23:09  PRIVOK
2026-10-23 01:23:05  PRIVOK
$ bex logs -r srv-dbdaqcvvlr1c73fj19ug --type app --text privprobe-8004 --limit 3 -o json | grep -o '"originalUrl[^,]*,\|"user-agent[^,]*,'
"originalUrl\":\"/privprobe-8004\",
"user-agent\":\"Wget\",
```

A's create response read back `serviceDetails.plan: "free"`, and `bex services update` reads `internalAddress: "<slug>:80"` on a free web service.

## Root cause

- The per-App NetworkPolicy (`lego/operator/internal/controller/app_controller.go:6564` `reconcileNetworkPolicy`) allows same-workspace ingress regardless of plan. No platform policy denies ingress to `app.bex.co/free-plan: "true"` pods. The only free-plan policy is egress (`deploy/gitops/base/tenant-node-egress.yaml:126-146`, `deny-free-plan-smtp-egress`).
- The slug-named ClusterIP Service (ADR041, w9/m57) is created for every web App, free included.

## Fix (target)

- t001: a `CiliumClusterwideNetworkPolicy` beside `deny-free-plan-smtp-egress` that selects `app.bex.co/free-plan: "true"` web pods (`app.bex.co/type` or the equivalent web selector; workers/private/cron have no free plan) with `ingressDeny` `fromEndpoints` of every workspace-labeled tenant pod (`app.bex.co/workspace` Exists, any namespace). Traefik, the activator, kubelet probes (`host`) and platform scrapers carry no workspace label and stay allowed. Add a `scripts/gitops-validate.sh` guard as m127 did. Verify on the pinned Cilium that `ingressDeny` from endpoints beats the per-App allow.
- t002: the label already follows plan changes (m127 t002). Prove upgrade/downgrade flip reachability without a manual restart.
- t003: decide each surface's shape on Free, the parity-sufficient choice being: keep `internalAddress` readable (Render derives it from the slug and does not hide it), but the dashboard Connect → Internal section on a free web service says "Free web services can't receive private network traffic; upgrade to use the internal address". Blueprint `fromService` `host`/`hostport` pointing at a free web service: validate refuses with the same sentence, in every workspace.

## Blast radius

- Every free web service in every workspace. Paid tenants that today call a free web service internally will break. Before rollout, t001 inventories live free web Apps that receive same-workspace traffic (Hubble flow query or access counts) and the release notes name the change. No allowlist: the rule ships to every workspace at once (parity rule).
- Not affected: public edge, custom domains, activator wake, health checks, Postgres/Key Value (they are not web Apps), paid web/private services.

## Unverified

- Render's exact wire/UX on a free service's internal address (no Render account): whether its API omits or keeps the field, and the dashboard copy. The target above keeps the field and refuses Blueprint wiring.
- Whether Render drops or refuses the connection (timeout vs refused); bex's choice (Cilium drop) is a timeout.

## Dedupe

Searched `.pm` open and done, ADR018, DO_NOT_DO for "private network", "receive private", "internal address" + free, ADR041: only w4/252 (copy fix that kept the behavior). No open item.

## Partial live evidence (2026-10-23 11:13–11:18 UTC, platform pin `29d63e1f7bea`, bex v0.3.3, `bex-canary`, `/qa-find-bugs-cli` sweep d4e8)

Free web A `srv-dbdk2l1f0c2s738fsdqg` (echo-server) and free busybox probe B `srv-dbdk2lr17lic73al6ne0` looping `wget -T 5 http://qa-20261023-d4e8-echo:80/privprobe-d4e8`: B logged 5× `PRIVFAIL`, no `PRIVOK`; `curl https://qa-20261023-d4e8-echo.onbex.co/` → 200. Not covered: upgrade/downgrade flip, paid controls, sleep/wake through the activator, Blueprint/dashboard copy (t003). DoD (t007) still open.
