# w6 · m127 — A free Key Value created in the dashboard is silently non-durable, on a plan that provisions a 1 GB disk — and `persistenceMode` can never be changed afterwards

**Worker:** worker6 **Goal:** a free Key Value is as durable as its plan actually allows, the UI stops asserting a hardware constraint bex does not have, and persistence is changeable after create like every other Key Value setting **Status:** done

## Resolution (2026-08-27)

- **The form no longer forces `off` on Free, and the false claim is gone (t001).** `dashboard/src/routes/keyvalue.new.tsx` dropped the `isFree`/`effectivePersistence` special-casing and both `disabled={isFree}` locks; the persistence default is `journal-snapshot`, matching the API create path (which defaults via the CRD's `+kubebuilder:default=journal-snapshot`, as observed live). The `keyvalue.fieldPersistenceFreeHint` string ("The Free plan has no persistent disk…") is deleted from **en and zh**.
- **`persistenceMode` is updatable post-create on all three surfaces (t002).** `KeyValuePatch` gained a `PersistenceMode *string` field with `validate`/`apply` mirroring `maxmemoryPolicy` and a shared `persistenceModeKnown` check reused by create; REST `PATCH /v1/key-value/{id}`, GraphQL `setKeyValuePersistenceMode`, and MCP `update_key_value(persistenceMode:)` all route through the shared `UpdateKeyValue`, so create and update can never diverge. `maxmemoryPolicy` post-create update is untouched and still green.
- **Existing `off` stores are not force-migrated (t003).** The PATCH path is the deliberate remedy — one call reaches `journal_snapshot` with no recreation and no lost id — because a persistence change rolls the pod and `off` is a legitimate deliberate choice on paid plans; a silent fleet-wide durability change is the opposite failure. Recorded in ADR021 §2.
- **Ledger corrected (t004).** `ADR018` no longer gives "no persistent disk" as the reason and now documents the post-create `persistenceMode` surface + the free-tier divergence; the `:17` re-baseline note is qualified (covered at create, not update, until w6/m127). ADR021 §2 records the whole decision.
- **Tests (t006):** REST rescue-scenario (`off` free store → `journal_snapshot` in place), GraphQL `setKeyValuePersistenceMode`, MCP `update_key_value(persistenceMode:)`, underscore normalization, unknown-value 400/error, and nil = unchanged; the dashboard create test now asserts Free is durable-by-default and the persistence control is unlocked. Backend `internal/keyvalue` + `internal/api` and dashboard suites pass.

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Stop forcing persistence off on bex's free plan, and correct the claim that explains it    | 45m | —          | — **DONE** |
| t002 | Make `persistenceMode` updatable post-create across REST, GraphQL and MCP                  | 55m | —          | — **DONE** |
| t003 | Decide what happens to free stores already created on `off`                                | 35m | t002       | — **DONE** |
| t004 | Render parity                                                                               | 25m | t001, t002, t003 | — **DONE** |
| t005 | Simplify                                                                                    | 20m | t004       | — **DONE** |
| t006 | Test coverage                                                                               | 40m | t004       | — **DONE** |
| t007 | Closeout                                                                                    | 15m | t005, t006 | — **DONE** |

## Definition of done

- **The two creation paths agree.** Creating a free Key Value **through the dashboard form** produces a store whose `GET /v1/key-value/{id}` reports the same `options.persistenceMode` as one created through `POST /v1/key-value` on the same plan. Today the API path yields `journal_snapshot` (**measured**) and the form sends `off` (**read from `keyvalue.new.tsx:108,122`, not observed** — so this bullet is a verification step, not a re-check).
- **No bex surface claims the Free plan has no persistent disk.** `tiers.yaml` gives free `storageGB: 1`. Grep the locales — **en and zh** — and confirm the claim is gone or corrected.
- **Persistence is changeable after create.** `PATCH /v1/key-value/{id}` accepts `persistenceMode` and the change shows on a subsequent `GET`; the same works through GraphQL and MCP `update_key_value`. Today all three ignore it — `KeyValuePatch` has no such field.
- **An existing free store on `off` can reach `journal_snapshot`** without being recreated.
- **`maxmemoryPolicy` still updates post-create** exactly as `w7/m45` shipped it — correct today, and precisely what a careless refactor of this patch path would break.
- **The ledger stops repeating the false premise.** `docs/ADR018-render-parity.md:150` no longer gives "no persistent disk" as bex's reason, and its Key Value row states the true post-create update surface.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **61st run**, 2026-08-27, journey 12 (Key Value). Workspace `tea-d98210cbbpdc73dcrkvg`.

  **Observed live** — created a free Key Value through REST and read it back:

  ```
  POST /v1/key-value {"name":"qa-20260827-kv","ownerId":"tea-…","plan":"free"}
    -> 201, red-da8b33vm2e9c73ft68eg
  GET  /v1/key-value/red-da8b33vm2e9c73ft68eg
    -> status "available", plan "free", version "8",
       options: {"maxmemoryPolicy":"allkeys_lru","persistenceMode":"journal_snapshot"}
  ```

  An API-created **free** store runs with journal+snapshot persistence. Fixture deleted afterwards (`deleteKeyValue` → true, `GET` → 404).

- **The dashboard does the opposite, and says why.** `dashboard/src/routes/keyvalue.new.tsx:104-108`:

  ```js
  // The Free plan has no persistent disk, so persistence is forced Off and both
  // settings lock — Render's exact behavior (docs/render-artifacts/key-value.md).
  const isFree = plan === FREE_PLAN;
  const effectivePersistence = isFree ? "off" : persistenceMode;
  ```

  `:122` sends it (`persistenceMode: effectivePersistence`), `:200` and `:228` disable both Selects (`disabled={isFree}`), and `:243` shows `keyvalue.fieldPersistenceFreeHint` — **"The Free plan has no persistent disk, so persistence is off."** (`dashboard/src/features/keyvalue/locales/en.ts:190`).

- **The premise is false for bex, and its own tier catalog says so.** `lego/types/tiers/tiers.yaml`, valkey section:

  ```yaml
  # ... (Render's KV free tier is 25MB / no persistence; bex's is more generous
  # at 128Mi.) ...
  tiers:
    - id: free
      cpu: 100m
      memory: 128Mi
      storageGB: 1        # bex's free Key Value HAS a 1 GB persistent disk
      instances: 1
  ```

  The catalog explicitly records that bex diverges from Render here. The form copied Render's constraint ("Render's exact behavior") without noticing that the thing it depends on — no persistent disk — is not true of bex.

- **What `off` actually does.** `lego/operator/internal/controller/keyvalue_controller.go:173-180`, `valkeyArgs`:

  ```go
  case "off":
      // No AOF and no RDB save points — a pure in-memory cache.
      args = append(args, "--appendonly", "no", "--save", "")
  ...
  default: // journal-snapshot and "": AOF + RDB
      args = append(args, "--appendonly", "yes")
  ```

  So a dashboard-created free store is a pure in-memory cache whose data does not survive a pod restart, while an API-created one on the identical plan journals to a disk that exists either way. **Two creation paths, opposite durability, same plan, no warning that they differ.**

- **And it cannot be undone — `persistenceMode` is create-only on every surface.**
  - `lego/backend/internal/keyvalue/service.go:683-694` — `KeyValuePatch` carries `Name`, `Plan`, `MaxmemoryPolicy`, `IPAllowList`. No `PersistenceMode`.
  - `lego/backend/internal/keyvalue/rest.go:336-341` — `PATCH /v1/key-value/{id}` decodes only `name`, `plan`, `maxmemoryPolicy`, `ipAllowList`, `dryRun`.
  - `lego/backend/internal/keyvalue/mcp.go:163` — `patch := KeyValuePatch{Name, Plan, MaxmemoryPolicy, IPAllowList}`.

  Render's pinned `keyValuePATCHInput` lists **five** properties: `name`, `plan`, `maxmemoryPolicy`, `persistenceMode`, `ipAllowList`. bex supports four. Render lets you change persistence after create; bex does not — which is what turns a bad default into a permanent one.

- **The ledger records the wrong rationale.** `docs/ADR018-render-parity.md:150` describes the create form's dropdowns as "Render-consistent, locked to Render's forced values on the Free plan — no persistent disk", repeating the false premise; and it documents "**Updatable post-create (w7/m45):** `maxmemoryPolicy` now mutates after create via REST PATCH … GraphQL … MCP" with **no** equivalent sentence for `persistenceMode`. The w2/m79 re-baseline at `:17` lists "Key Value `persistenceMode`" among rows "re-verified as unchanged where bex already covered the surface" — true at create, not at update.

- **Goal linkage:** [docs/ADR021-keyvalue-management.md](../../../docs/ADR021-keyvalue-management.md) and [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md); indirectly [ADR030](../../../docs/ADR030-pricing.md), since the free tier's 1 GB disk is provisioned whether or not persistence uses it.

- **Expected outcome:** a free Key Value is as durable as its plan actually allows, and persistence is changeable after create.

- **Why now:** this silently costs users data durability **by default**, on the plan most likely to hold a first experiment, and it is invisible — the control is disabled with an explanation that reads as authoritative platform fact. It also cannot be self-corrected, so every affected store stays affected.

- **Precedent — extend, do not re-litigate.** `w7/m45` made `maxmemoryPolicy` updatable post-create through REST/GraphQL/MCP on the shared `UpdateKeyValue`; `t002` is the same change for its sibling field and should reuse that shape. `w5/011` added both create-time dropdowns and introduced the free-plan lock. **Neither is a regression** — the lock was written deliberately, from a premise about Render that does not hold for bex.

- **Render parity:** included (t004). `persistenceMode` is on Render's PATCH input, so adding it **closes** a divergence rather than opening one; and the free-tier behaviour is a place where bex deliberately differs from Render (more generous), which must be **recorded as a divergence** rather than silently reverted to Render's shape.

- **Blast radius:** `KeyValuePatch` has 2 write entry points that must move together (REST `handleUpdateKeyValue`, MCP `update_key_value`) plus GraphQL. `UpdateKeyValue`/`PreviewUpdateKeyValue` share `validate()` so create and update can never diverge — keep that property. Operator side is `valkeyArgs` (`keyvalue_controller.go:171-190`), whose `default:` branch deliberately covers both `"journal-snapshot"` and `""` so legacy stores reconcile byte-identically; new handling must not disturb that.

- **Adjacent classes:** a **paid** store (unlocked today, unaffected); a store created **before** this change (`t003`); `persistenceMode: "snapshot"` (RDB only — the middle option, which must keep working); and the CRD's hyphenated form versus Render's underscore wire form, which `renderToCRD`/`crdToRender` (`service.go:189-190`) already normalize and which the new patch path must route through identically.

- **Unverified this run — carried as work, not presented as observation:** **I did not create a Key Value through the dashboard form** — the `off` outcome is read from `keyvalue.new.tsx:108,122`; whether any **existing** production free store is actually sitting on `off` (the workspace's two pre-existing stores were not inspected for plan or persistence); whether **GraphQL `createKeyValue`** applies the same free-plan forcing as the REST path; and what a persistence-mode change does to **live data** in each direction, which `t002` must establish rather than assume.
