# w33 · m436 — A Blueprint Key Value with no `plan` is created on `256mb` (Starter), as Render's spec says

**Worker:** worker33 **Goal:** a Blueprint that declares a Key Value instance without a `plan` creates the same size Render does (`256mb`, bex's `starter`), prices it and gates it as paid, and leaves an existing instance's plan alone. **Status:** blocked — release gate, clears by itself: t001–t007 are done and shipped in `fd6218884`. [t008](t008.md) replays the definition of done on production once a "pin platform images to" commit contains it (§ Blocked).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Blueprint parse: an omitted Key Value plan is `starter` on create, and unchanged on sync — **DONE** | 30m | — |
| [t002](done/t002.md) | The estimate, the payment gate, the plan output and the free-instance check follow the new default — **DONE** | 35m | t001 |
| [t003](done/t003.md) | An omitted plan on the other Key Value creates, checked against Render's pinned contracts — **DONE** | 35m | t001 |
| [t004](done/t004.md) | ADR030, ADR049 D5 and the ADR018 plan-ID row say what the default is; corpus fixtures — **DONE** | 20m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout (live replay on production) — **BLOCKED** (release gate) | 40m | t006, t007 |

## Definition of done

On production, in `bex-canary`, with owned fixtures deleted afterwards (by-id `GET` 404):

1. Validating a Blueprint with one `type: keyvalue` entry and no `plan` (for example `render-examples/sidekiq`'s `render.yaml`, unmodified) answers `valid: true`; its plan names the Key Value on `starter` and its estimated pricing carries a Starter line, not $0.
2. Applying it creates the instance on `starter` (REST `GET /v1/key-value/{id}`, GraphQL, MCP `get_key_value` and the dashboard agree), and the create counts as paid intent for the payment-method gate.
3. A Blueprint whose existing Key Value is on `free` and whose file still omits `plan` plans no change on sync and the instance stays on `free`.
4. An explicit `plan: free` still creates a free instance, and the free-instance limit still applies to it.
5. `cd lego/backend && go test ./...` and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for one aspect of render parity (Blueprints), 2026-11-16, item 2; claims re-verified against `bdf087c00` before filing (`/pmx them all to w33`).
- **Render reference:** render.com/docs/blueprint-spec § `plan`, read 2026-11-16: "If you omit this field: Render uses `0.5c-512mb` for a new web service, private service, background worker, or cron job. Render uses `256mb` for a new Key Value instance. Render retains the current compute plan for an existing service." Render's pinned OpenAPI requires `plan` on `keyValuePOSTInput`, so the default is a Blueprint rule.
- **bex today:** a plan-less service lands on `starter` (w1/122, user decision 2026-10-15) and a plan-less database on `basic-256mb` (`lego/backend/internal/apps/deploy.go:3489`). The Key Value parse applies no default (`deploy.go:3560`), so the CRD's `free` default wins (`lego/types/v1alpha1/keyvalue_types.go:53`), and `lego/backend/internal/apps/blueprint_pricing_test.go:263` asserts "a plan-less Key Value store stays free". Four of the 14 Key Value entries in `github.com/render-examples` omit `plan` (`git-scm.com`, `ioredis`, `saleor`, `sidekiq`; counted 2026-11-16).
- **Goal linkage:** ADR008 Render parity first; ADR030 (the plan-less default follows Render); ADR049 D5 (presence-aware sync); ADR018 row "Blueprint compute plan IDs".
- **Expected outcome:** one file yields the same Key Value size on bex and on Render, the review step shows its real price, and a second plan-less store in a workspace is no longer a free-limit question.
- **Why now:** Key Value is the one resource kind the w1/122 default change left out, and the difference decides what a customer is billed and how much memory they get.
- **Decision state:** following Render's default is this proposal's recommendation, resting on ADR008 Render parity first and on the reasoning the user accepted for services in w1/122. It moves a default from $0 to a paid plan and is not a recorded user decision for Key Value; ADR030 gets the sentence, marked as such, until the user signs it off. Keeping `free` would be a divergence that needs the user's sign-off in ADR018.
- **Render parity:** included: Blueprint validate, plan, preview and apply are tenant-facing on REST, GraphQL, MCP and the dashboard.

## Work record (t001–t007, 2026-11-16)

- **Parse (t001).** `parseKeyValue` (`lego/backend/internal/apps/deploy.go`) writes `blueprintKeyValueDefaultPlan` (`starter`) into an entry without `plan`, beside the database's `blueprintDatabaseDefaultPlan`. Both `type: keyvalue` and `type: redis` reach it, and so does a `create_new` copy, because the suffix is applied to the YAML before the parse. The presence bit is untouched: `ApplyBlueprintKeyValueSpec` still writes `plan` only when the file declares it, so a sync of an existing free store plans `noop` and writes nothing.
- **Review and gates (t002).** `blueprintEstimatedPricing` reads the parsed plan, and its own empty-means-free fallback is gone. `stackHasPaidPlan` and the preview's free-slot check already read `spec.Plan`, so a plan-less store is priced as Starter ($7.00 plus its 1 GB at $0.21), counts as paid intent, and never consults the free-instance limit. An explicit `plan: free` still does. Before this change a plan-less store skipped the free check and was then made free by the CRD default. That was a second free store in the workspace, and it is closed now. The plan's create action carries no plan field for any kind, so on REST, GraphQL and MCP the plan reaches the review through the estimate's line (`tier: starter`, `tierLabel: Starter`).
- **Other creates (t003).** None changed. REST `POST /v1/key-value` (Render requires `plan`), MCP `create_key_value` (Render's server defaults to `free`), GraphQL (bex-only) and the dashboard form (always sends a plan) keep the CRD default. The table is in [ADR021 §4](../../../../docs/ADR021-keyvalue-management.md#4-plans--renders-key-value-vocabulary).
- **Records (t004).** [ADR030 §8](../../../../docs/ADR030-pricing.md), [ADR049 D5](../../../../docs/ADR049-render-yaml-parity.md#d5--sync-is-presence-aware-and-field-specific) and the ADR018 "Blueprint compute plan IDs" row state `starter`. ADR030 and ADR018 mark it as following ADR008 and awaiting the user's sign-off. The four render-examples shapes (`git-scm.com`, `ioredis`, `saleor`, `sidekiq`; all `type: redis`) are inline cases in `TestRenderExamplesPlanLessKeyValuesArePricedAsStarter`.
- **Render parity (t005), read from the code; the live walk is t008.** Validate, preview, create and sync on REST, GraphQL and MCP, and the dashboard's review and Blueprint pages through GraphQL, all go through `ValidateBlueprint` / `DeployStack` over the same parse. So they agree on the plan and the estimate, and the created store reads `starter` wherever `spec.plan` is shown. Generate Blueprint used to leave `plan` out for a free store, so re-applying an exported file would have moved that store to Starter. It now always writes the plan (`generateKeyValueEntry`). One drift remains, shared with services and databases: the estimate and the paid-intent check price an existing resource whose file omits `plan` at the create default, not at the plan the sync keeps. It is filed as [w33/006](../../006.md).
- **Simplify (t006).** The two datastore defaults are now named constants; nothing else needed changing.
- **Tests (t007)** in `lego/backend/internal/apps/keyvalue_plan_default_test.go`, plus the updated pricing and agreement tests. With the change reverted, every new test fails except the sync test. The sync test checks behavior this milestone keeps, so instead it was confirmed to fail when `ApplyBlueprintKeyValueSpec` ignores the presence bit. `go test ./...` against the CI-pinned Postgres, OpenFGA and OpenBao, plus backend lint, pass. The dashboard is unchanged.

## Blocked (2026-11-16)

**Release gate (clears by itself; no decision needed).** Production runs the bex-api image of the newest `chore(deploy): pin platform images to <sha>` commit on `main`. That pin was `b2704b688742` when this milestone parked, and it does not contain `fd6218884`. Check:

```bash
git fetch origin main
pin=$(git log origin/main --grep='^chore(deploy): pin platform images to' -1 --format=%s | sed -E 's/.*images to ([0-9a-f]+).*/\1/')
git merge-base --is-ancestor fd6218884 "$pin" && echo "pinned: replay the definition of done"
```

Then t008 walks items 1–4 in `bex-canary`:

- validate `render-examples/sidekiq`'s `render.yaml` unmodified;
- apply a one-store Blueprint without `plan` and read the store on REST, GraphQL, MCP and the dashboard;
- re-sync a free store whose file omits `plan`;
- apply an explicit `plan: free` store.

Then delete the fixtures (by-id `GET` 404), record the conclusions here and close the milestone. The Starter fixture is billed while it lives, so delete it in the same run.
