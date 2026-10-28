# Common Blueprint Mistakes

Symptoms, causes, and fixes for frequent `render.yaml` errors.

## 1. Setting `branch` while using preview environments

**Mistake:** A global or service-level `branch` is set (e.g. `main`) while PR previews are enabled.

**Effect:** Previews are meant to build **pull request** refs. Forcing a single branch can make preview builds ignore the PR commit you intend to test, undermining PR workflows.

**Fix:** Use default branch settings appropriate for production services; let previews use PR head refs. Avoid hardcoding production `branch` on services that must track arbitrary PRs. Validate preview behavior in the Dashboard after changes.

---

## 2. `buildFilter` sync — omitting `paths` clears filters

**Mistake:** Syncing a Blueprint that includes `buildFilter` but **omits** `paths` or `ignoredPaths` keys.

**Effect:** Missing keys can be interpreted as “set to empty,” **replacing** existing path filters with empty lists—triggering builds on every unrelated commit or breaking intended ignore rules.

**Fix:** Always specify the **full** desired `paths` and `ignoredPaths` arrays when using `buildFilter`. Treat filters as replace, not patch.

---

## 3. `readReplicas` — empty list or name drift

**Mistake A:** Setting `readReplicas: []` to “leave unchanged.”

**Effect:** An **empty list destroys all read replicas**.

**Mistake B:** Renaming replica identifiers in YAML without understanding reconcile behavior.

**Effect:** Unmatched replicas may be **removed** and new ones **created**, causing churn or brief disconnects.

**Fix:** Treat `readReplicas` as authoritative. Only include entries you want to exist. Prefer additive changes with care and validate in a non-production environment first.

---

## 4. Deprecated fields

| Deprecated | Use instead |
|------------|-------------|
| `env` | `runtime` |
| `redis` (service type) | `keyvalue` (alias `redis` if still accepted—prefer `keyvalue` in new files) |
| `autoDeploy` | `autoDeployTrigger` |
| Top-level `previewsEnabled: true` | Top-level `previews.generation: automatic` |
| Service-level `pullRequestPreviewsEnabled: true` | Service-level `previews.generation: automatic`; omit `generation` to disable service previews |

**Fix:** Migrate to current keys; run schema validation against `https://render.com/schema/render.yaml.json`.

---

## 5. Same resource defined in multiple places

**Mistake:** Defining a service or database at the **root** and again under a **project environment**, or listing duplicates across environments.

**Effect:** Confusing reconcile behavior, name collisions, or unintended duplicate infrastructure.

**Fix:** Each logical resource should live in **exactly one** scope: root `services` / `databases` **or** a single `projects[].environments[]` block—not both.

---

## 6. Using the wrong preview-plan field

**Mistake:** Setting `previewPlan` on a compute service, setting `previews.plan` on Key Value or Postgres, or mixing flexible and legacy Postgres compute plans between `plan` and `previewPlan`.

**Effect:** Preview deploy failures or plan validation errors.

**Fix:** Use `previews.plan` for web, private, worker, and cron services. Use `previewPlan` for Key Value and Postgres, and keep Postgres plans in compatible instance families.

---

## 7. `fromService` / `fromDatabase` to non-existent names

**Mistake:** Typo in `name` under `fromService` / `fromDatabase`, or reference before resource is defined in scope.

**Effect:** Validation failures or broken deploys with missing env vars.

**Fix:** Keep a single naming table; grep for `name:` and cross-check references. Run `bex blueprints validate`.

---

## 8. Forgetting `ipAllowList` on Key Value

**Mistake:** Provisioning `keyvalue` without proper network restrictions.

**Effect:** Security exposure or service creation errors (depending on account defaults and product requirements).

**Fix:** Always set **`ipAllowList`** to explicit CIDRs appropriate for your private services and regions.

---

## 9. Hardcoding connection strings

**Mistake:** Pasting `postgres://...` or `redis://...` into `value:`.

**Effect:** Secrets in Git, drift across environments, rotation pain, preview breakage.

**Fix:** Use **`fromDatabase`** and **`fromService`** with `connectionString` or discrete fields; use **`generateValue`** for app secrets.

---

## Quick prevention

1. `bex blueprints validate` (CLI v2.7.0+).
2. JSON Schema in the IDE: `https://render.com/schema/render.yaml.json`.
3. Diff previews against production wiring before merging.
