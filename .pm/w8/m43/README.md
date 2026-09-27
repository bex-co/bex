# w8 · m43 — Postgres high availability runs on free and sub-1-CPU plans: gate it by plan like Render

**Worker:** worker8 **Goal:** `enableHighAvailability` is honored only on Postgres plans that support it (at least 1 CPU), on every write path, so a free database can never become a replicated multi-instance cluster for $0. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                          | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Gate HA by plan in the shared Postgres service: create, PATCH, and plan downgrade while HA is on               | 45m | —                |
| t002 | Blueprint: refuse `highAvailability: true` on a sub-1-CPU plan at validate and apply, with a location          | 30m | t001             |
| t003 | Dashboard: disable the HA control with a reason on unsupported plans; plan picker refuses a downgrade under HA | 40m | t001             |
| t004 | Audit existing HA databases on unsupported plans; record the transition, no silent conversion                  | 20m | t001             |
| t005 | Render parity                                                                                                  | 20m | t002, t003, t004 |
| t006 | Simplify                                                                                                       | 15m | t005             |
| t007 | Test coverage                                                                                                  | 30m | t006             |
| t008 | Closeout                                                                                                       | 15m | t007             |

## Definition of done

- `bex postgres update <free-dpg> --high-availability` exits non-zero with a 400 naming the plan requirement (at least 1 CPU, e.g. `standard` / `1c-2g` and above). The database keeps `highAvailabilityEnabled: false` and still runs one instance. `bex postgres create --plan free --high-availability` is refused the same way.
- On a supported plan, HA still enables. A downgrade to an unsupported plan while HA is on is refused with "disable high availability first", never silently switched off.
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
- **Expected outcome:** free and sub-1-CPU databases stay single-instance, and HA works exactly where Render offers it.
- **Why now:** every tenant can currently double their free Postgres capacity with one flag. The cost grows with adoption, and a later clamp on existing databases gets harder (t004).
- **Render parity included:** the fix changes REST/GraphQL/MCP/Blueprint/UI behavior.

## Unverified

- Whether the dashboard already hides the HA toggle on free plans (only CLI/REST was exercised).
- How metering bills the standby instance-seconds on a free plan.
- Whether `--read-replica` has the same gap. It was not exercised to avoid creating extra free replicas; t004 should check.
