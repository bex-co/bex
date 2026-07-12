# w4 · m4 — Authorization: OpenFGA over the auth substrate

**Worker:** worker4 **Goal:** bex-api decides not just _who_ is calling (m1–m3: Kratos identities, Hydra API keys) but _what they may touch_: OpenFGA (Zanzibar-style ReBAC) runs on the cluster against its own CNPG database, a checked-in authorization model defines tenant-scoped permissions, and bex-api consults it through a Core seam — with nil-checker behavior identical to today so nothing breaks before the flip. **Status:** done (2026-07-06; E2E green on the local mock — seeded bootstrap allowed, tuple-less key 403, env-unset regression allow-all; /simplify applied: shared ttlCache + doJSON across the Ory/FGA clients, reflection sweep guarding every Core verb, stdio transport exempted from authz wiring, model.fga↔model.json drift gate with the fga CLI in CI, fixpoint normalize)

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------- | --- | ---------- |
| t001 | `openfga-db` CNPG cluster in `charts/auth-dbs` + local sizing — **DONE** | 25m | — (w4/m3)  |
| t002 | Argo Application for OpenFGA (pinned chart, values, cluster-internal) — **DONE** | 35m | t001       |
| t003 | Authorization model in git + idempotent store/model apply script — **DONE** | 40m | t002       |
| t004 | `Checker` seam in bex-api Core + enforcement in the three adapters — **DONE** | 45m | t003       |
| t005 | Seed tuples at deploy (bootstrap → admin) + E2E allow/deny on mock — **DONE** | 35m | t004       |
| t006 | Docs: authorization section in `docs/ADR012-auth.md` + env tables — **DONE** | 25m | t005       |
| t007 | Simplify — run `/simplify` over the code this milestone changed — **DONE** | 20m | t006       |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE** | 30m | t006       |

## Definition of done

On the local mock cluster: OpenFGA pods healthy against `openfga-db`; the checked-in model applied idempotently; with `BEX_OPENFGA_URL` set, the seeded `bex-bootstrap` identity passes all verbs while a freshly minted key with no tuples gets 403 on manage verbs (list/mint/revoke keys, suspend/delete resources) and OpenFGA-unreachable fails closed (503); with the env unset, behavior is byte-for-byte today's allow-all (`make test` green unchanged); model + seam covered by unit tests with a fake checker/FGA server; docs updated.

## Source + Goal linkage

- **Source:** promoted from inbox note `w4/001` (user request 2026-07-06: "add openfga integration to this system for w4").
- **Goal linkage:** roadmap #1 (multi-tenant control plane) and pillars 3–4 — per-client credentials (m3) are only half of multi-tenant security; tenants also need their resources isolated by _permission_, not just identity.
- **Expected outcome:** authorization decisions externalized to a queryable relation store: an agent's key can be scoped to one tenant's Apps instead of the whole platform.
- **Why now:** m3 put `api.IdentityFrom` (the subject) into every request context — the exact hook a checker needs; landing the seam + single-default-tenant model now means w1/m2's tenants/accounts tables plug into an existing enforcement path instead of retrofitting one.

## E2E invocation (t005)

On the local mock cluster (auth substrate + OpenFGA deployed per docs/ADR012-auth.md, App CRD via `cd operator && make install`):

```
KUBECONFIG=$PWD/infra/local/bex.kubeconfig bash scripts/auth-e2e.sh
```
