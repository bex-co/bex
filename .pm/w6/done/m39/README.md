# w6 · m39 — Control-plane identity for the namespace pruner

**Worker:** worker6 **Goal:** two bex-api control planes can share one cluster without deleting each other's tenant namespaces **or App CRs** — parallel multi-worker `dev-N` development stops being serialized, and a destructive namespace delete stops being keyed on the wrong scope **Status:** done

## Tasks (in order)

| id   | title                                                                                | est | depends_on  |
| ---- | ------------------------------------------------------------------------------------ | --- | ----------- |
| t001 | Add a control-plane instance identity stamped on every managed namespace — **DONE**    | 30m | —           |
| t002 | Scope `pruneOrphans` to its own identity; settle the unlabeled-legacy case — **DONE**  | 35m | w6/m39/t001 |
| t003 | Wire the identity through every `dev-N` harness — **DONE**                             | 25m | w6/m39/t002 |
| t007 | Extend the identity guard to the App-CR projector — **DONE**                           | 40m | w6/m39/t002 |
| t004 | Simplify the code this milestone changed — **DONE**                                    | 20m | w6/m39/t007 |
| t005 | Test coverage: two identities on one cluster prune only their own — **DONE**           | 30m | w6/m39/t007 |
| t006 | Closeout — **DONE**                                                                    | 10m | w6/m39/t005 |

## Definition of done

Two control planes configured with distinct identities run against the same cluster and **neither deletes the other's `tea-*` namespaces or App CRs** — proved by a test that fails against today's code. A control plane running under the production identity still reclaims its genuine orphans, **including pre-existing namespaces that carry no identity label** (no regression in the existing prune behavior). `bash .pm/w6/dev-6/up.sh` and its sibling `dev-N` harnesses set the identity, and no `.pm/*/dev-*/README.md` still carries the "at most one dev-N control plane at a time" restriction.

## Source + Goal linkage

- **Source:** [`.pm/w3/017.md`](../../w3/017.md) — found by `w3/m78`'s live leg 2026-08-08, re-verified against HEAD by `/pm-brainstorm` 2026-08-17: `lego/backend/internal/store/namespaces.go:279` `pruneOrphans` lists **every** namespace carrying `LabelManagedBy` and deletes any whose workspace is absent from *its own* database, with no control-plane instance identity anywhere in the selector.
- **Goal linkage:** [`GOAL.md`](../../GOAL.md) #5 (multi tenant) — the pruner is tenant-lifecycle machinery, and its blast radius is currently "every control plane sharing a cluster". [`docs/ADR043-tenant-namespace-isolation.md`](../../../docs/ADR043-tenant-namespace-isolation.md) owns the `NamespaceReconciler` this scopes.
- **Expected outcome:** the `.pm/wN/dev-N` isolation design — which predates the ADR043 pruner and is silently broken by it — works as documented again: every workstream can raise its own control plane concurrently.
- **Why now:** it silently serializes the entire multi-worker development model across 11 workstreams, and it is a **destructive namespace delete keyed on the wrong scope** — the same class of mistake is one config error away from being pointed at production. Observed live in both directions (the dev-5 session's bex-api pruned a freshly provisioned `tea-…` namespace within 60s, and symmetrically). `t002` is a genuine design call rather than a one-liner: unlabeled legacy namespaces must stay prunable under the production identity while never being cross-prunable from a `dev-N`.
- **Render parity task omitted:** this is an internal control-plane mechanism plus dev tooling — a new operator-facing env var and a namespace label selector. No REST, GraphQL, MCP, or dashboard surface changes.

## Implementation notes

- **The one-at-a-time restriction was never actually written down.** `t003` expected to *remove* it from `.pm/*/dev-*/README.md`, but `w3/017.md` only said to "note this in `.pm/*/dev-*/README.md` when touched next" and that never happened — a repo-wide grep found the restriction in no harness README and no workstream README. So the task added the positive guarantee instead: each of the ten `dev-N` READMEs now documents its own `BEX_CP_IDENTITY`, why the cluster-scoped prune made concurrent stacks unsafe, and that running several at once is now safe.
- **The D6 admission policy admits the new label.** [`bex-api-namespace-admission.yaml`](../../../deploy/gitops/base/bex-api-namespace-admission.yaml) asserts specific label **values** and never a closed label **set**, so adding `app.bex.co/control-plane` to a namespace the reconciler already owns is admitted. Checked before shipping: a policy enumerating the exact label set would have failed every production namespace write the moment this landed.
- **Production needs no configuration change.** `BEX_CP_*` is supplied out of band (no `BEX_CP_DB_URI` appears in any checked-in manifest), and unset `BEX_CP_IDENTITY` means `production` — which, under the unlabeled-is-mine rule, keeps reclaiming pre-existing orphans from the very first pass.
