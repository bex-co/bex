# w6 · m517 — A store's proxy keeps its held clients until the store's own change rolls it

**Worker:** worker6 **Goal:** a platform deploy that changes `/kv-internal-proxy`, or the proxy pod template the operator renders, rolls no store's proxy. Each store's proxy keeps the template it runs until the store's own change rolls it: a plan change, enabling internal authentication, a credential rotation or a resume. That roll takes the current image and template at once. A code-reviewed override rolls every proxy at once for a security fix. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The proxy's pod template holds until the store's own change rolls it](t001.md) | 2h | — |
| t002 | [A security override rolls every proxy at once, and a guard makes each rendering change a decision](t002.md) | 45m | t001 |
| t003 | [Prove it on the local cluster](t003.md) | 1h30m | t002 |
| t004 | [Record it in ADR021 § 10 and the operator guide](t004.md) | 30m | t002 |
| t005 | [Ship and verify on production](t005.md) | 1h30m | t003, t004 |
| t006 | [Simplify](t006.md) | 30m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- A settled proxy Deployment's pod template stays byte-identical, and no new ReplicaSet appears, across reconcile passes of a manager whose `/kv-internal-proxy` hash and image differ and whose rendering adds a field. A test covers it.
- The store's next plan change, enabling of internal authentication, credential rotation or resume rolls its proxy **once**, onto the manager's image and the full current template. Enabling internal authentication still closes a held unauthenticated client within seconds, and new unauthenticated clients read `-NOAUTH` (the w6/m463 and w6/m512 checks).
- Bumping the override revision rolls every store's proxy once on the next pass, and never again until the next bump. A rendering change without a decision fails a test that tells its author to update the pinned rendering and decide whether to bump.
- A Deployment written before this milestone is adopted without a roll when its template already equals the rendering. Otherwise it rolls once, as today, and is held from then on.
- On the local cluster, a client held through `<id>:6379` keeps its `CLIENT ID` across an operator deploy that changes `/kv-internal-proxy` and the rendered template. The store's next plan change rolls the proxy onto the new binary.
- On production: the shipping deploy rolls no store's proxy and records every one. A `bex-canary` probe's held client keeps its `CLIENT ID` across a later deploy while its proxy runs a binary other than the manager's. Enabling internal authentication on the probe then rolls it once onto the manager's image.
- ADR021 § 10 states the rule, the inputs that roll a proxy, Render's basis, the override and when to bump it, the compatibility rule for held templates, and the gap that remains. `lego/operator/AGENTS.md` carries the author's rule.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/145](../done/145.md) on 2026-11-19 by `/loopx w6`. w6/m512 t006 filed it as the stricter variant ADR021 § 10 names.
- **Goal linkage:** ADR008 § Render parity first.
  - Render breaks a store's connections when the customer changes it ("Render restarts your Key Value instance", [Key Value](https://render.com/docs/key-value)). For its own infrastructure it does so only in service-affecting maintenance, which "usually occur[s] no more than once every three months". For a paid service, "Render always provides advance notice via email" and can usually reschedule it ([platform maintenance](https://render.com/docs/platform-maintenance), both read 2026-11-19).
  - bex closes every held client of every store, without notice, on each deploy that changes the proxy. That was 2 of the 11 production deploys on 2026-11-17, and w6/m512's drain only bounds the close at 30 s. No ADR018 row signs this off.
- **Expected outcome:** pools, pub/sub subscribers and long blocking commands held through a store's internal URL survive every platform release. They reconnect only when the customer changes the store, or on a deliberate, reviewed security roll.
- **Why now:** w6/m469 stopped image-only deploys from rolling proxies, and w6/m512 made a proxy-code roll drain. The proxy-code deploys that remain still close every held client of every store, and the proxy changed in each of m456, m463 and m512 within three days.
- **Render parity task omitted:** this is an operator-internal mechanism. No REST, GraphQL, MCP or dashboard field or message changes. t003 and t005 check the tenant-visible effect.

## Findings at promotion (2026-11-19, code at `2df9359a5`)

- **The premise holds.** `reconcileKeyValueInternalProxy` (`lego/operator/internal/controller/keyvalue_internal_proxy.go`) renders the whole pod template on every pass. `keyValueInternalProxyImage` keeps the running image only while its recorded `/kv-internal-proxy` hash equals the manager's. A differing binary takes the manager's image, so a deploy that changes either the binary or any rendered field rolls every store's proxy.
- **The hold must cover the whole template, not the image.** Each of this month's three binary changes also changed the rendering in the same commit:
  - `0eed45bcb` (m456): `BEX_KV_MAX_CONNECTIONS`, the TLS cert env and the memory formula;
  - `ef091b640` (m463): `BEX_KV_INTERNAL_AUTH`;
  - `d6d29f9a7` (m512): `BEX_KV_DRAIN_SECONDS` and `terminationGracePeriodSeconds`.
  - So an image-only deferral would have deferred none of these rolls.
- **The pattern exists.** `holdDatastorePriority` (`datastore_priority.go`) and `holdIsolationLabel` (`keyvalue_isolation.go`) already keep a platform change out of a template until something else rolls it, comparing with `templateEqualIgnoringIsolation`. This milestone generalises that to the whole proxy template. Its trigger is the store's own inputs, not "any other difference".
- **What the customer's change touches (the roll inputs):**
  - the plan: `BEX_KV_MAX_CONNECTIONS` and the memory limit;
  - `spec.internalAuth`: `BEX_KV_INTERNAL_AUTH=client`;
  - the credential revision: the `credentialRevisionAnnotation` and the auth Secret;
  - `intent.tlsForwarded()` and the TLS Secret name: making public an older store whose listener is static (a runtime-switch store rolls nothing, ADR021 § 3);
  - suspension: `replicas: 0`. A resume starts from no pod, so it may take any template for free.
  - A plan change, a credential rotation and a suspension also restart Valkey, which closes every upstream connection anyway (ADR021 § 10, w6/m512).
- **Interplay with the work in flight:**
  - The change stays in `internal/controller`. `go list -deps` shows neither SNI binary links it (they link `internal/sniproxy` and `lego/types/...`), and neither does `/kv-internal-proxy` (`internal/kvinternalauth` only). So it moves neither w6/m470's kept SNI pins nor w6/m515's window. Any `lego/types` edit would move both, so make none.
  - w6/m512's ship commit `d6d29f9a7` is not in production's pin yet: the newest pin, `a91e9aeb3` → `dff52c596d13`, does not carry it. Its roll onto the drain env must happen and be recorded by m512 t007 before this hold can keep any template. Otherwise the hold could keep the pre-m512 template and m512's check would never see its roll. The adoption rule in t001 also guarantees this: an unsettled template rolls once. t005 still waits for the record.
  - `TestANewOperatorImageChangesNoLongLivedTenantPodTemplate` (`operator_upgrade_test.go`, w6/m484) guards the image-only case for the store and its proxy. t007 extends its Key Value case to a changed binary and rendering.
- **What the hold subsumes:**
  - Once every proxy Deployment carries its inputs record, m469's image hold is dead code: `keyValueInternalProxyImage`, `carriesInternalProxyBinary`, `InternalProxyBinaryFromRegistry` and the `kv-internal-proxy-sha256` record. Held inputs keep the whole template, and changed inputs roll anyway.
  - `holdDatastorePriority` and `holdIsolationLabel` likewise become dead on this Deployment.
  - t006 removes what production's adoption has made dead. `registry.ImageFileSHA256` stays, because `cmd/proxy-pin` (w6/m470) uses it.
- **The ceiling, weighed:**
  - Render does not roll paid stores without notice, and bex has no maintenance-notice surface (`.pm/DO_NOT_DO.md` lists "maintenance runs" as a non-goal). So a time ceiling would bring back an unannounced close, rarer but still the divergence this removes.
  - The override is the only forced roll. It is a constant bumped in a reviewed commit, for a security fix or for a held template that can no longer work.
  - Render "might perform maintenance on a free service at any time, without advance notice" (same page). Rolling free stores at once would therefore be Render-consistent, but it was not chosen: one rule for every plan, and w6/145's done-when names no store rolling.

## Design constraints

- **Records go on the Deployment's own metadata**, never on the pod template, as m469's hash record does. Adding a key to the template would roll every proxy once.
- **One roll.** A customer change takes the current image and the current template in one template write.
- **The shipping deploy rolls nothing.** Adoption records a settled Deployment without changing its template. The commit changes neither `/kv-internal-proxy` nor the rendered template.
- **Held templates must keep working.** While any proxy may run an old template and binary, the operator must not rename or remove what one relies on: the `<id>-backend` Service and its ports, the auth and TLS Secrets it mounts, the pod labels its NetworkPolicy and the internal-auth cut select (`reconcileKeyValueInternalProxyPolicy`, w6/m512 t004), and the `--check` readiness command. A change that breaks this must bump the override.
- **No authority gained.** The proxy pod gets no service account token and no new Secret, and the operator gets no new RBAC.
- **No `lego/types`, `cmd/kv-internal-proxy` or `internal/kvinternalauth` edits** (see § Findings, interplay).
