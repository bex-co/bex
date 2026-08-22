# w7 · m77 — Finish ADR043: namespace managed Postgres/Key Value under `<ws>` and repair every broken datastore link

**Worker:** worker7 **Goal:** a service created today can actually reach the managed Postgres/Key Value it declares via `fromDatabase`/`fromService` — the link works on create, with no hand-copied Secrets, hand-written network policy, or hand-patched hostnames. **Status:** done — **t007 (live hetzner-prod cutover) was executed 2026-08-21**: all four datastores are in `<ws>`, all three forums serve real content, every hand-made artifact is gone, and the workspace `ResourceQuota` is actually charged (closing `w3/010`).

> **Blocker re-check 2026-08-17 (during `/loop-worker w7`).** Two of t007's three original blockers are now **resolved**: the code **is** shipped (`docs/runbooks/datastore-namespace-cutover.md` and the code commits are in `main`), and production cluster credentials **are** available (admin kubeconfig via `scripts/fetch-app-kubeconfig.sh` + `.env`). The remaining blocker is the third and it is unchanged: **explicit authorization to migrate live tenant data**, since the procedure takes a write outage per tenant on forums serving real content.
>
> Live state re-confirmed the same day — the datastores are still in `default`: `dpg-d9nqg95cavls73fp8m20`, `dpg-d9rrkoc4h4mc73edurp0`, `dpg-d9rs3ee0ccis738kc7c0`, `red-d9p49kdrtmes73c34ovg`.
>
> **Deliberately deferred out of the autonomous drain loop by user decision**, so the loop proceeded to m82. The reasoning: an irreversible production data migration with user-visible downtime wants an attended session, not an unattended one. This is a scheduling decision, not a de-prioritization — m77 remains a live production defect and every affected tenant stays broken until t007 runs (ADR043 D8.5: the code fixes new resources only).

## Tasks (in order)

| id   | title                                                                        | est | depends_on                     |
| ---- | ---------------------------------------------------------------------------- | --- | ------------------------------ |
| t001 | Settle the target namespace contract + live cutover plan (ADR043 amendment) — **DONE** | 45m | — |
| t002 | Regression harness: pin the three broken legs as failing tests — **DONE** | 40m | w7/m77/t001 |
| t003 | Move Database/KeyValue CRs + every reader into the tenant namespace (bex-api) — **DONE** | 60m | w7/m77/t002 |
| t012 | Operator: move tenant-namespace Secret access off the cached client — **DONE** | 45m | w7/m77/t003 |
| t013 | Hosting allow set: admit CNPG/Valkey control, scrape, and proxy paths — **DONE** (live legs verify in t007) | 45m | w7/m77/t003 |
| t004 | Operator: per-tenant Barman ObjectStore + backup CronJobs follow the namespace — **DONE** | 45m | w7/m77/t012 |
| t005 | Connection-string correctness after co-location (internal · external · pooler) — **DONE** | 35m | w7/m77/t003 |
| t006 | Charge the namespace ResourceQuota for datastores (closes `w3/010`) — **DONE** | 30m | w7/m77/t003 |
| t007 | Live cutover on hetzner-prod + retire the manual workarounds — **DONE** | 60m | w7/m77/t004, w7/m77/t005, w7/m77/t013 |
| t008 | Render parity sweep: datastore link + connection surfaces — **DONE** | 30m | w7/m77/t006, w7/m77/t007 |
| t009 | Simplify the code this milestone changed — **DONE** | 30m | w7/m77/t008 |
| t010 | Test coverage for the shipped behavior — **DONE** | 40m | w7/m77/t008 |
| t011 | Closeout — **DONE** | 15m | w7/m77/t009, w7/m77/t010 |

> **t012/t013 added by t001** (2026-08-08). The enumeration found two blockers the milestone brief did not anticipate, both of which would have failed only at runtime inside a tenant namespace: the operator's Secret informer is scoped to one namespace and **cannot** be widened (its ClusterRole omits Secrets, so a cluster-wide informer stops the entire cache — App controller included — from starting), and the tenant allow set grants none of the in-cluster paths CNPG and Valkey need. See [ADR043](../../../docs/ADR043-tenant-namespace-isolation.md) D8.2 and D8.3.

> **Blocked: t007 — live cutover on hetzner-prod.** Needs three things this session does not have: (1) the code shipped, which per repo rule only `/ship` may do; (2) production cluster credentials; (3) explicit authorization to migrate live tenant data — `tianpan-forum`, `blockeden-forum`, and `beancount-forum` are serving real Discourse content, and the procedure takes a write outage per tenant. The runbook, its rollback per step, and its rehearse-first requirement are ready in [`docs/runbooks/datastore-namespace-cutover.md`](../../../docs/runbooks/datastore-namespace-cutover.md). **Until t007 runs, existing affected tenants stay broken** — the code fixes new resources only (ADR043 D8.5).

## Definition of done

On a workspace whose namespace is `<ws>`, applying a Blueprint that declares a web service plus a linked Postgres and Key Value produces a **running, connected** service with no manual intervention:

- the datastore Secrets are readable by the pod (no `CreateContainerConfigError`),
- the pod reaches Postgres `:5432` and Valkey `:6379` (no connection timeout),
- the hostname in the injected env resolves from the pod (no `could not translate host name`),
- `count/databases.app.bex.co` and `count/keyvalues.app.bex.co` are actually charged against the workspace's `ResourceQuota`,
- and the three legs are pinned by tests that fail against the pre-fix code.

On hetzner-prod, the existing `default`-namespace datastores (`beancount-forum`, `tianpan-forum`, `blockeden-forum` and siblings) are migrated or explicitly recorded as legacy-supported, and every hand-made workaround artifact (copied Secrets, hand-written `CiliumNetworkPolicy`, hand-patched `host` key, hand-made Ingress) is removed with the forums still serving.

## Source + Goal linkage

- **Source:** production bug report 2026-08-08 (Blueprint auto-deploy of `tianpan-forum` + `blockeden-forum`, workspace `tea-d98210cbbpdc73dcrkvg`, cluster `hetzner-prod`), triaged against the code the same day. **Supersedes `.pm/w3/010.md`**, which recorded the identical namespace split but scoped it to the quota-enforcement consequence only; the secret/DNS/network consequence was never traced. w3/010 lists two options and explicitly says to promote "as part of any future work that already touches Postgres/KeyValue namespace placement" — this is that work, and it forces option 1.
- **Goal linkage:** the Render-alternative core (`docs/ADR008-vision.md`). `fromDatabase`/`fromService` is the Blueprint contract's central wiring primitive (ADR049-render-yaml-parity) and pillar 4's deploy-from-chat depends on it; a stack that cannot reach its own database is not a Render alternative. Also completes `docs/ADR043-tenant-namespace-isolation.md:38`, whose stated contract is "App / Database / KeyValue land in `<ws>`" — only the App half shipped (`aca23a05`, `db7d95fc`).
- **Expected outcome:** every managed-datastore link created after this milestone works on first deploy. The three failure legs collapse into one fix: co-locating the datastore with the App makes `secretKeyRef` resolvable, `allow-same-namespace` sufficient, and the CNPG short-name host correct again — the same reason pre-migration tenants like `beancount-forum` silently worked.
- **Why now:** this is a **live production defect with no workaround short of hand-reconstructing the reconcile per tenant**, and it silently affects every service created since the ADR043 migration. It also has no user-visible signal (see m79), so the blast radius is unknown until each tenant crashes. The quota gap `w3/010` accepted as an interim also stays open until this lands.
- **Render parity task included:** yes — the change moves the namespace of a user-facing resource and touches the connection-info/URL surfaces exposed over REST, GraphQL, and MCP.
