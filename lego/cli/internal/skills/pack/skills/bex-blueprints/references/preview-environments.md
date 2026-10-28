# Preview Environments

Pull request (PR) previews spin up temporary stacks from your Blueprint so each PR can be tested in isolation. This document summarizes Blueprint knobs and behavioral constraints.

## Top-level `previews`

```yaml
previews:
  generation: off          # default
  expireAfterDays: 3
```

### `previews.generation`

| Value | Behavior |
|-------|----------|
| `off` | No automatic preview creation from PRs (default). |
| `manual` | Previews are created on demand (Dashboard/workflow action—exact UX may vary by integration). |
| `automatic` | Previews are created for eligible PRs per bex’s Git integration rules. |

### `previews.expireAfterDays`

- Positive integer: automatically **delete** preview environments after **N** days to control cost and clutter.
- Omit or set per product/docs for “no expiry” behavior.

---

## Service previews vs. preview environments

Service-level `previews.generation` configures a service's independent pull request previews. It does **not** override the Blueprint's top-level preview-environment generation mode.

```yaml
services:
  - type: web
    name: api
    runtime: node
    # ...
    previews:
      generation: manual
```

The only supported service-level generation values are `manual` and `automatic`. Omit `previews.generation` to disable service previews. Do not set it to `off`, and do not use it to try to exclude a resource from preview environments.

---

## Preview compute plans

- For web services, private services, background workers, and cron jobs, set **`previews.plan`**. Despite sharing an object with the service-preview generation field, this field controls the compute plan used in preview environments.
- Set **`previews.numInstances`** to control the service's instance count in preview environments.
- For Key Value and Postgres instances, set **`previewPlan`**.
- For Postgres, `plan` and `previewPlan` must both use flexible compute plans or both use legacy compute plans.
- If no preview compute plan is specified, bex uses the base resource's compute plan.

Use `compute-plans.md` for current Plan IDs and availability rather than copying plan catalogs into this preview-specific reference.

```yaml
services:
  - type: web
    name: api
    plan: standard
    previews:
      plan: starter
  - type: keyvalue
    name: cache
    plan: standard
    previewPlan: starter
    ipAllowList: []
databases:
  - name: db
    plan: pro-4gb
    previewPlan: basic-1gb
```

---

## Database previews: `previewDiskSizeGB`

- Databases created for previews may use **`previewDiskSizeGB`** (and related preview fields per schema) so preview DB storage is smaller/cheaper than production.
- Keep disk large enough for migrations and seed data used in CI.

---

## `sync: false` env vars in previews

- Environment variables with **`sync: false`** are **excluded** from automatic preview configuration.
- **Users must set these manually** on preview services (Dashboard or automation) or the app may fail at runtime (missing secrets).

**Mitigation:** For previews, prefer secrets that can be generated per-preview, or use non-secret placeholders, or inject via your CI pipeline into preview env as allowed by your security model.

---

## Autoscaling in previews

- **Autoscaling is disabled** in preview environments.
- Instance counts follow **minimum instance** behavior (effectively pinned to the floor—commonly `scaling.minInstances` when autoscaling exists, or a single instance).
- Do not rely on preview traffic to test autoscale rules; test scaling in staging/production-like environments.

---

## PR workflow — how previews map to Git

1. A **pull request** is opened against the tracked branch (e.g. `main`).
2. bex uses the **PR head commit** (and Blueprint at that revision) to propose or create a preview environment according to the top-level `previews.generation` setting.
3. **Build filters** and **branch** settings on services still matter: misconfigured `branch` can break the expectation that previews track the PR (see `common-mistakes.md`).
4. When the PR closes or merges, previews are typically torn down per product rules; **`expireAfterDays`** enforces a maximum lifetime regardless.

---

## Checklist

- [ ] `previews.generation` matches team workflow (`off` / `manual` / `automatic`).
- [ ] `expireAfterDays` set if org needs automatic cleanup.
- [ ] Compute services use `previews.plan`; Key Value and Postgres use `previewPlan`.
- [ ] Postgres `previewPlan` is compatible with the primary `plan` family.
- [ ] `previewDiskSizeGB` (if used) sufficient for migrations.
- [ ] No reliance on `sync: false` secrets without a preview secret strategy.
- [ ] Any service-level `previews.generation` setting is intentional and treated separately from preview-environment generation.

Validate with `bex blueprints validate` and `https://render.com/schema/render.yaml.json`.
