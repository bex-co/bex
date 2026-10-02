# w8 · m43 — Postgres high availability runs on free and sub-1-CPU plans: gate it by plan like Render

**Worker:** worker8 **Goal:** `enableHighAvailability` is honored only on Postgres plans that support it (at least 1 CPU), on every write path, so a free database can never become a replicated multi-instance cluster for $0. **Status:** done

## Tasks (in order)

| id   | title                                                                                                          | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Gate HA by plan in the shared Postgres service: create, PATCH, and plan downgrade while HA is on — **DONE**               | 45m | —                |
| t002 | Blueprint: refuse `highAvailability: true` on a sub-1-CPU plan at validate and apply, with a location — **DONE**          | 30m | t001             |
| t003 | Dashboard: disable the HA control with a reason on unsupported plans; plan picker refuses a downgrade under HA — **DONE** | 40m | t001             |
| t004 | Audit existing HA databases on unsupported plans; record the transition, no silent conversion — **DONE**                  | 20m | t001             |
| t005 | Render parity — **DONE**                                                                                                  | 20m | t002, t003, t004 |
| t006 | Simplify — **DONE**                                                                                                       | 15m | t005             |
| t007 | Test coverage — **DONE**                                                                                                  | 30m | t006             |
| t008 | Closeout — **DONE**                                                                                                       | 15m | t007             |

## Definition of done

- `bex postgres update <free-dpg> --high-availability` exits non-zero with a 400 naming the plan requirement (at least 1 CPU, e.g. `standard` / `1c-2g` and above). The database keeps `highAvailabilityEnabled: false` and still runs one instance. `bex postgres create --plan free --high-availability` is refused the same way.
- The current catalog has no HA-capable Postgres plan: all published plans refuse enabling HA. Adding a priced ≥1-CPU plan is deferred. Tests with an explicitly supplied supported tier keep the positive predicate path covered; downgrading an existing HA database to an unsupported plan is refused with "disable high availability first", never silently switched off.
- REST, GraphQL, MCP, Blueprint and the dashboard give the same answer and the same message (one shared predicate from the tier catalog).
- Any existing databases in production with HA on an unsupported plan are listed, and their handling is decided and recorded, not silently changed.

## Evidence (2026-09-26, `/qa-find-bugs-cli` sweep 11, production at `dbf24a217`)

Released `bex v0.2.1` (pin v2.27.0), human device login, workspace `bex-canary`. Fixture: `qa-20260926-eb8d39-pg` (`dpg-darlq2q9slkc73beqtr0`, since deleted), `--plan free --version 16`.

```text
$ bex postgres update dpg-darlq2q9slkc73beqtr0 --high-availability --confirm -o json      # exit 0
{ "plan": "free", "highAvailabilityEnabled": true, "status": "available", … }
# about 1 min later, GET (highAvailabilityEnabled is the operator's OBSERVED state, i.e. ≥2 ready instances, service.go:103-107):
$ bex postgres get dpg-darlq2q9slkc73beqtr0 -o json   →  plan free · status available · highAvailabilityEnabled true
```

- **Pinned contract:** `bex postgres create/update --help` says "Enable high availability (available for plans with at least 1 CPU)". The pinned client's own test names the plans without HA: `free`, `0.1c-256mb`, `0.5c-1g`, `basic-256mb`, `basic-1gb` (`render-oss/cli` `pkg/postgres/options_test.go:27-35`, `plansWithoutHighAvailability`). The client does not enforce this at runtime; Render's API does. bex's server is the only gate, and it has none.
- **bex policy:** ADR030 §6 caps the free compute plan at one running instance because "N free instances would deliver N× the capacity for $0". Free Postgres HA does exactly that for datastores: a primary plus a standby for $0.

## Mechanism

- `lego/backend/internal/postgres/service.go:638` copies `req.EnableHighAvailability` into `Spec.HighAvailability` on create, and `:1029-1031` copies the PATCH value, with no plan check. The plan is canonicalized and validated nearby (`:599`, `:874-875` `tiers.Postgres.ByID`), but CPU is never consulted.
- The tier catalog already has the data: `lego/types/tiers/tiers.yaml` `postgres:` gives `free`/`basic-256mb` `cpu: 100m` and `basic-1gb` `cpu: 500m`. The predicate is "cpu ≥ 1".
- bex's own tests encode the wrong expectation: `postgres/ha_test.go:35,76` and `apps/blueprint_roundtrip_test.go:91` enable HA on `basic-1gb`, a plan Render excludes. t001/t002 must move those fixtures to a supported plan.
- Blueprint: `databases[].highAvailability` is accepted (`apps/blueprint_compiler.go:448`) and exported (`blueprint_generate.go:497`) without a plan check.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 11 (w8), 2026-09-26.
- **Goal linkage:** Render-compatible managed Postgres (ADR006, ADR018 "HA · failover · read replicas" row) and the free-tier capacity rule (ADR030 §6).
- **Expected outcome:** free and sub-1-CPU databases stay single-instance. HA remains unavailable until bex introduces a priced, supported tier.
- **Why now:** every tenant can currently double their free Postgres capacity with one flag. The cost grows with adoption, and a later clamp on existing databases gets harder (t004).
- **Render parity included:** the fix changes REST/GraphQL/MCP/Blueprint/UI behavior.

## Unverified

- Whether the dashboard already hides the HA toggle on free plans (only CLI/REST was exercised).
- How metering bills the standby instance-seconds on a free plan.
- Whether `--read-replica` has the same gap. It was not exercised to avoid creating extra free replicas; t004 should check.

## Historical blockers (2026-09-26; resolved 2026-10-02)

t001–t007 are done (gate in the shared service, Blueprint, dashboard, audit, parity, tests). t008 cannot close:

1. **User decision: there is no ≥1-CPU Postgres plan.** bex ships `free` (100m), `basic-256mb` (100m) and `basic-1gb` (500m), so the DoD line "on a supported plan, HA still enables" has no plan to hold on, and HA is refused everywhere. Either add a ≥1-CPU rung (a `tiers.yaml` postgres entry + a `pricing.yaml` rate + the Stripe catalog, i.e. a pricing call), which re-enables HA with no code change, or accept "no HA until then" and amend this DoD line.
2. **Live closeout** after this ships to production: `bex postgres update <free-dpg> --high-availability` and `bex postgres create --plan free --high-availability` exit non-zero with the 400. Needs the deploy to land (see `blocked/m42`) and a logged-in CLI (`bex login`).

## Live re-verification (2026-09-27, `/qa-find-bugs-cli` sweep 51)

Holds on production `4a0422577`: a new free Postgres (`dpg-dasglvi1pbgc73a24j5g`, deleted), `bex postgres update <dpg> --high-availability --confirm` → exit 1, `400 (POSTGRES_HA_PLAN_UNSUPPORTED): high availability requires a Postgres plan with at least 1 CPU; plan "free" has 100m CPU`. `postgres get` afterwards shows `highAvailabilityEnabled: false`. The ≥1-CPU positive leg still waits on the plan decision in § Blocked.

## Closeout (2026-10-02)

The user accepted the review's recommended sequence, including enforcement-only scope: **no HA in the current catalog**. No new plan, price, or Stripe product is introduced. A supported-plan launch is deferred product work, not a condition for this enforcement milestone.

Using an isolated `bex v0.2.1` device login in `bex-canary`, free Postgres create with `--high-availability` and update with the same flag both returned `400 POSTGRES_HA_PLAN_UNSUPPORTED`, naming the 1-CPU requirement and `free`'s 100m CPU. The update fixture `dpg-davmjcede41s73canrc0` remained `highAvailabilityEnabled: false` and was deleted. A fresh read-only production audit found **3 databases, 0 configured or observed with HA**. The prior shared-service, Blueprint, dashboard and downgrade tests remain the cross-surface evidence; sibling free-plan flags remain separate in `w8/035`.


**Cleanup verified:** both owned web-service fixtures and the free Postgres fixture were deleted; all four baseline services remained. The isolated CLI credential was revoked, and the temporary local test containers were removed.
