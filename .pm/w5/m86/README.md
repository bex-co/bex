# w5 · m86 — Platform observability UI: Grafana at obs.bex.co (ADR088)

**Worker:** worker5 **Goal:** operators read the existing Prometheus/Loki backends through a provisioned Grafana at `obs.bex.co`, signing in with their bex identity, gated by ops-workspace membership — no kubectl port-forwards, no separate password store. **Status:** t001–t009 done; t010 closeout open — code complete and verified locally, pending `/ship` + post-merge live `obs.bex.co` verification via Argo CD

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | GitOps base: Grafana Application in `monitoring` with provisioned datasources — **DONE** | 45m | —          |
| t002 | Overlays: prod `obs.bex.co` ingress + OIDC values; local admin-only shape — **DONE**   | 30m | t001       |
| t003 | Hydra client: bootstrap the obs client in `auth-bootstrap-client.sh` — **DONE**        | 30m | —          |
| t004 | bex-api: internal ops-role verb (`BEX_OPS_WORKSPACE` / `BEX_OPS_ROLE_TOKEN`) — **DONE** | 45m | —          |
| t005 | Consent acceptor: ops-gated client class (`OAUTH_OPS_CLIENTS`) + id_token claims — **DONE** | 45m | t004       |
| t006 | Ops-workspace guards: refuse delete/suspend; exempt invite seat/plan gate — **DONE**   | 30m | t004       |
| t007 | E2E + docs: allow/deny consent flow verification, env inventories — **DONE**           | 30m | t002, t003, t005 |
| t008 | Simplify — `/simplify` over the code this milestone changed — **DONE**                 | 30m | t006, t007 |
| t009 | Test coverage — meaningful tests for gate, guards, and claims — **DONE** (mutation spot-checks: non-member→ungated flip fails 2 consent tests; bearer-compare inversion fails 2 opsrole tests) | 30m | t006, t007 |
| t010 | Closeout                                                                               | 15m | t009       |

## Definition of done

All platform suites green (`make test`, backend `go test ./...`, `make lint`, `dashboard yarn test`). The GitOps tree gains a Grafana Application in `monitoring` (datasources + starter dashboards provisioned as code) that `scripts/gitops-validate.sh` accepts; prod overlay carries `obs.bex.co` + OIDC config, local overlay runs admin-only. The consent acceptor rejects a non-ops-workspace identity for the obs client with `access_denied` and stamps `email`/`name`/`ops_role` claims for a member (unit-tested both paths); the bex-api ops-role verb answers only with the static bearer; delete/suspend of the pinned ops workspace is refused and invite seat-gating exempts it (tested). An e2e script proves the allow and deny consent paths against the throwaway Ory stack. Live `obs.bex.co` rollout lands via Argo CD after ship — verify post-merge and record in this README.

## Verification record (2026-09-07)

- **Suites:** backend `go test ./...` exit 0 + `make lint-backend` 0 issues; operator `make test` exit 0; dashboard 3,026 tests + `yarn typecheck` pass; cli green; `scripts/gitops-validate.sh` PASS (0 FAIL); all overlays render.
- **E2E (`scripts/auth-obs-e2e.sh`, throwaway Hydra v26.2.0 + Kratos + in-memory OpenFGA, real bootstrap script → real bex-api with **no CP DB** → real dashboard consent route):** ALLOW leg — headless ops-gated consent accepts an ops-workspace admin, id_token + `/userinfo` carry `ops_role=GrafanaAdmin`/email/name; DENY legs — an authenticated customer with no tuple AND a `billing`-role member both bounce with `access_denied`, no code ever issued; verb probe — wrong bearer 401, member answer matches the t004 wire contract. 4 green runs total (3 by the t007 track, 1 after the t008 bootstrap-script refactor, proving behavior preservation).
- **Mutation spot-checks (t009):** flipping the gate's non-member deny to ungated fails 2 consent tests; inverting the opsrole bearer compare fails 2 verb tests.
- **Store-gated tests** (`opsguard_pg_test.go`: seat-cap exemption, account-deletion blocked disposition) skip without local Postgres and run in CI's real-Postgres job.

### Operational notes

- Secrets flow: `scripts/auth-bootstrap-client.sh` (needs `BEX_OBS_OAUTH_CLIENT_SECRET`) → `scripts/obs-secrets.sh` (installs `bex-system/bex-ops`, `dashboard/bex-ops`, `monitoring/grafana-oauth`, `monitoring/grafana-admin`; reuses existing values, `ROTATE=1` to rotate, rolls only changed consumers).
- The billing lifecycle also has an operator-owned exclusion path (`SetBillingException`): marking the ops workspace billing-excluded prevents dunning from ever scheduling enforcement against it; the `OPS_WORKSPACE_PROTECTED` refusal in `billing.KubernetesEnforcer.Enforce` is a DB-state-independent backstop, so a delinquent ops workspace retries loudly instead of converging — intended.
- Declined `/simplify` findings (recorded): `core.BearerGate` promotion (would touch the pre-existing CP-API auth path for a mutually-cross-referenced 6-line pair) and `startControlPlaneServer`'s 11-param signature (pre-existing shape; a dedicated cleanup).
- Adjacent finding filed as `w5/052`: `name-conflict-e2e.sh`'s Hydra v2.2.0 pin cannot pass the bootstrap lifespan assert (needs v26.x).
- **t010 closeout blocked on:** the user's live sign-in confirmation only. Shipped 2026-09-07 (`25e8cbc9f` + `4eea10f20` pipefail fix; the CI-only `TestAccountDeletionOpsWorkspaceBlockedPG` failure — `NOT (subject = ANY(NULL))` filtering every member — was fixed by `42d4532a3` COALESCE). **Live verification 2026-09-08:** prod Hydra has `bex-obs` (idempotent bootstrap run converged all five clients); all four Secrets installed via `obs-secrets.sh` (ops workspace `tea-d98210cbbpdc73dcrkvg`); `bex-charts/grafana` published public; Argo `grafana` Synced/Healthy, pod Running; `https://obs.bex.co/login` → 200; `/login/generic_oauth` → 302 to `oauth.bex.co/oauth2/auth` with `client_id=bex-obs` + PKCE S256 challenge.

## Source + Goal linkage

- **Source:** [docs/ADR088-platform-observability-ui.md](../../../docs/ADR088-platform-observability-ui.md) (accepted 2026-09-07), from the platform-monitoring discussion of 2026-09-07 (Grafana placement → hostname → AuthN/AuthZ decisions).
- **Goal linkage:** platform reliability/operations underpin every ADR008 hosting pillar — the monitoring backends (Prometheus/Loki/Alertmanager) already run as platform GitOps; this makes them usable by humans, with access control reusing ADR012/ADR024 machinery instead of a parallel credential system.
- **Expected outcome:** an operator with a live dashboard session opens `obs.bex.co` and lands in Grafana with a role mapped from their ops-workspace role; a customer identity is denied at consent server-side. Dashboards and datasources are git-provisioned, zero click-ops state.
- **Why now:** the backends have run headless since w4/m88-era work — operators debug with port-forwards and raw PromQL today; agent-session and billing incidents (w5/m80–m85 lineage) repeatedly needed exactly these platform views. The auth building blocks (consent acceptor, bootstrap script, OpenFGA roles) are all shipped and stable, so the marginal cost is one gate branch plus GitOps manifests.
- **Render parity omitted:** no tenant-facing REST/GraphQL/MCP/UI surface changes — the ops-role verb is server-only and in-cluster, Grafana is platform infrastructure, and ADR010's customer metrics/logs surfaces are untouched.
