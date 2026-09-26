# A Key Value restarting after a config change reports "creating", and "available" while it is down

Why: an operator who changes an existing store's eviction policy sees it labelled as being created from scratch (and "Available" for the first ~15s while every connection fails), where Render's own `databaseStatus` enum has `config_restart` for exactly this state.

Found live, `/qa-find-bugs` pass 159, 2026-09-25 (w4-targeted, `muse.env` credentials), journey 12, on throwaway `qa-20260925-kv` (`red-darktcjbdpcs73f5eha0`, Free, public; deleted the same pass).

## Repro (probes actually run)

1. Create a Free Key Value with Public access and wait for Available (about 45s).
2. Poll `redis-cli --tls --sni <host> -u <external URL> CONFIG GET maxmemory-policy` every 4s, and in parallel read the detail page's Status every 4s.
3. Change Maxmemory Policy (`allkeys-lru` ↔ `noeviction`) and save.

Observed timeline (UTC, second run):

| time         | redis-cli                     | dashboard Status |
| ------------ | ----------------------------- | ---------------- |
| 04:57:38     | `noeviction`                  | Available        |
| 04:57:43–:57 | `SSL_connect failed`          | **Available**    |
| 04:58:01–:17 | `SSL_connect failed` to :22   | **Creating**     |
| 04:58:21+    | `allkeys-lru` (data retained) | Available        |

The first run showed the same shape, with about 60s unreachable. The restart itself is correct and Render-parity: bex already emits `key_value_config_restart` because the operator folds the policy into the StatefulSet args (`lego/operator/internal/controller/keyvalue_controller.go:186-187`; `docs/render-artifacts/datastore-webhook-events.md:77`). The status word is what is wrong.

## Root cause

- `lego/backend/internal/keyvalue/service.go:225-234` `kvStatus` maps every non-Ready phase to `"creating"`. The operator sets `KVPhaseProvisioning` both on first create and on any later rollout or unready pod (`keyvalue_controller.go:846-850`, reason `Provisioning`/`PodUnready`), so a restart of a long-lived store is indistinguishable from creation. `kvView` (`service.go:241`) feeds REST (`rest.go:85`), GraphQL (`graphql.go:38`), and MCP alike.
- The stale "Available" window happens because Phase stays `Ready` until the operator observes the new generation. The Ready condition's `ObservedGeneration` then trails `kv.Generation`, and nothing reads that gap.
- Render's `databaseStatus` enum (pinned in `lego/backend/internal/api/openapi/render-public-api-1.json`) is `creating, available, unavailable, config_restart, suspended, maintenance_scheduled, maintenance_in_progress, recovery_failed, recovery_in_progress, unknown, updating_instance`.

## Fix (target behavior)

- Once a store has ever been Ready (for example, `Status.CredentialRevision != ""`, which the operator sets only on Ready), a non-Ready, non-Failed phase maps to **`config_restart`**. So does a `Ready` phase whose Ready condition `ObservedGeneration < Generation`. `creating` is reserved for a store that has never been Ready. Precedence stays `deleting` > `suspended` > the rest.
- Consumer: `dashboard/src/features/keyvalue/lib/status.ts` `STATUS_MAP` has no `config_restart` key, so without a matching entry the badge falls to "unknown". Add `config_restart` → a "Restarting" label (en + zh; `locale-parity.test.ts`) in the same change. The pinned Render CLI already knows the value.
- Copy: `key-value-maxmemory-policy-section.tsx` says only "The operator applies the new policy on the next reconcile". Name the brief restart, as Persistence Mode's "rolling update" copy should (a single replica has no zero-downtime roll).

## Blast radius / unverified

- `kvStatus` has 1 caller (`kvView`). `grep -rn 'kvStatus(' lego/backend` = 2 lines (the definition + `service.go:241`).
- **Unverified:** Postgres `dbStatus` (`lego/backend/internal/postgres/service.go:381-392`) has the same default-to-`creating` shape and also returns `upgrading`, which is not in Render's enum. Neither was probed this pass, so they are not folded into this fix. Instance-type and persistence-mode changes go through the same `kvStatus` path but were not exercised (instance type is a paid change).

Estimate: ~50m including backend table test + dashboard mapping test.
