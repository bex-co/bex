# w33 · m436 — A Blueprint Key Value with no `plan` is created on `256mb` (Starter), as Render's spec says

**Worker:** worker33 **Goal:** a Blueprint that declares a Key Value instance without a `plan` creates the same size Render does (`256mb`, bex's `starter`), prices it and gates it as paid, and leaves an existing instance's plan alone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Blueprint parse: an omitted Key Value plan is `starter` on create, and unchanged on sync | 30m | — |
| t002 | The estimate, the payment gate, the plan output and the free-instance check follow the new default | 35m | t001 |
| t003 | An omitted plan on the other Key Value creates, checked against Render's pinned contracts | 35m | t001 |
| t004 | ADR030, ADR049 D5 and the ADR018 plan-ID row say what the default is; corpus fixtures | 20m | t002, t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout (live replay on production) | 40m | t006, t007 |

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
