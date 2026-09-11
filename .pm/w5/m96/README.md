# w5 · m96 — Production canary fixture: authorize, first green and red runs, classify

**Worker:** worker5 **Goal:** the first-party `bex-canary` workspace, its free hello-go web service, and its scoped API key exist in production and are wired into the repository as the `BEX_CANARY_*` secret and variables, so the four credentialed synthetic probes shipped by `w3/m83` stop soft-skipping and each has a recorded green run and a recorded red-path proof. **Status:** BLOCKED on t001 — attempted 2026-09-10 with authorization and refused by the production payment gate (PAYMENT_REQUIRED on the free plan; `BEX_REQUIRE_PAYMENT_METHOD=all`). Nothing was created. Needs a browser Stripe card step or a policy decision — see § t001 attempted

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Provision the fixture: `bex-canary` workspace (billing-excluded), hello-go free web service, scoped API key | 30m | —          |
| t002 | Wire the repository: fixture ids as variables, key via `.env` + `scripts/gh-secrets.sh`                    | 15m | t001       |
| t003 | Dispatch the three credentialed workflows once; record green run ids in ADR088 and the m83 evidence        | 30m | t002       |
| t004 | Red-path proof per credentialed probe: break, confirm the issue opens, restore, confirm it closes           | 30m | t003       |
| t005 | Classify the canary workspace as `canary` for product analytics; confirm the ops boards exclude it         | 15m | t003       |
| t006 | Simplify                                                                                                  | 15m | t004, t005 |
| t007 | Test coverage                                                                                             | 20m | t004, t005 |
| t008 | Closeout                                                                                                  | 10m | t007       |

## Definition of done

- `gh variable list` shows `BEX_CANARY_WORKSPACE_ID`, `BEX_CANARY_SERVICE_ID`, `BEX_CANARY_URL`; `gh secret list` shows `BEX_CANARY_API_KEY`; `.env` holds the key (never printed or committed).
- The `tenant-view-liveness` job (Production edge liveness), `deploy-canary` (weekly), and `isolation-matrix` (weekly) each have a recorded green `workflow_dispatch` run id with **no** `::notice::` soft-skip line, and a recorded red-path run that opened its tracking issue followed by a green run that closed it.
- ADR088 § "Scheduled probes and the canary fixture" has no `_owed_` cell (except the optional static-site repo, explicitly left unset); `.pm/w3/done/m83/README.md` Evidence is annotated with the run ids.
- The canary workspace is `billing_excluded` (ADR040 § Mode A) and is classified `canary` in `product_analytics_audiences` — or, if migration 0114 is not yet live in production, its id is recorded in `docs/runbooks/product-analytics.md` as pending classification.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 4. `docs/ADR088-platform-observability-ui.md` § canary fixture table lists five settings as `_owed_`; `.pm/w3/done/m83/README.md` Evidence: "no `BEX_CANARY_*` secret/vars exist on the repo yet … creating the first-party `bex-canary` workspace, minting the key, and recording green + red-path `workflow_dispatch` runs is an authorized operator step"; `docs/ADR008-vision.md` § What's next → Ops hardening.
- **Authorization — BLOCKED, and this supersedes the note previously here.** An earlier draft of this milestone claimed that materializing it through `/pm` constituted authorization for t001. That was an inference, and it is wrong: [ADR088](../../../docs/ADR088-platform-observability-ui.md) line 146 states that the owed operator steps "provision real first-party production resources, so they are an authorized human action, **not something a scheduled job or an agent may do**". Repository policy is explicit, so t001 needs the user to either run it personally and hand over the four ids, or say plainly in-session that an agent should run it under their production credentials. Everything from t002 onward is ordinary work once the ids and key exist. Access was checked rather than assumed: the GitHub token can write repository variables and secrets, but `GET https://api.bex.co/v1/owners` answers 401 unauthenticated, so creating the workspace, service and key would mean acting as the operator against production.

- **Facts verified 2026-09-10:** `gh variable list` / `gh secret list` contain no `CANARY` entries; `.github/workflows/deploy-canary.yml:78` still prints the soft-skip notice; `.env.example:476` already declares `BEX_CANARY_API_KEY`; the variable/secret split is documented in `ssh-edge-liveness.yml:388-401`.
- **Goal linkage:** pillar 2 (agent-readable state has to be true) and ADR088's falsifiable-green principle; the probes exist precisely to end the "guard that exists and does not run" failure pattern (`w6/m131`, `w6/m132`).
- **Expected outcome:** silent regressions of deploy-from-git, tenant read surfaces, and tenant/sandbox isolation open GitHub issues within one probe interval, with a proven red path so a green run is evidence, not decoration.
- **Why now:** the probes shipped 2026-09-09 and have skipped on every schedule since; every skipped run is the exact failure the milestone was built to end.
- **Render parity omitted:** pure platform operations; no REST/GraphQL/MCP/dashboard change.
- **Anti-goals:** `#CI-RUNNERS` / `#RUNNER-HOSTS` respected — credentialed jobs stay on the existing `bex-production` pool; nothing changes on the runner hosts.

## t001 attempted 2026-09-10 — blocked by the production payment gate

Authorization was granted in-session (`/loop-worker w5 m96 m97 m98`, naming m96 after the blocker was raised), so t001 was attempted under the operator's own production credentials. It is blocked by something authorization cannot clear.

**What happened.** `createWorkspace(name: "bex-canary", plan: "hobby")` against `https://api.bex.co/graphql` was refused server-side:

```
PAYMENT_REQUIRED — Payment information is required for paid plans.
Call create_billing_checkout_session to add a payment method, then retry.
```

**Nothing was created.** The workspace list is unchanged (`bex`, `tian-personal`); no `bex-canary`, no Stripe Customer, no partial attempt. The refusal happens before any write.

**Why it blocks.** The message says "paid plans", but the request was for `hobby` — the free plan. Production therefore runs `BEX_REQUIRE_PAYMENT_METHOD` in **`all`** mode, which per [the backend env reference](../../../lego/backend/CLAUDE.md) "includes free + agent-sessions". So the canary fixture cannot be created without first binding a payment method through Stripe Checkout — an interactive browser flow that an agent cannot and should not complete. This is an access limit, not a permission one.

**This also corrects an assumption in ADR088.** Its "owed operator steps" read as though the operator can simply create the workspace and deploy the service. Since `w4/m90` made a payment method mandatory for workspace creation under the `all` policy, even the deliberately-free canary now has to pass that gate. Whoever schedules this work next should know that before starting.

**What would clear it** (a decision for the user, not a default an agent should pick):

1. **Create `bex-canary` in the dashboard**, completing the Stripe card step in a browser, then hand over the four ids. t002 onward is then ordinary work and needs nothing further.
2. **Temporarily set the policy to `paid`** so free-plan creation skips the gate, create the canary, restore the setting. Cheapest, but it changes a production billing control for the duration and wants a deliberate choice.
3. **Reuse an existing first-party workspace** as the canary. Avoids the gate entirely but abandons ADR088's isolated-fixture intent and its `billing_excluded` premise; recorded for completeness, not recommended.

**Verified reachable, so only the gate is missing:** the production control plane answers `kubectl` (needed for the `billing_excluded` flag, which GraphQL does not expose), the GraphQL surface authenticates with the operator's CLI token, and the GitHub token can write repository variables and secrets. Every step after workspace creation is unblocked.
