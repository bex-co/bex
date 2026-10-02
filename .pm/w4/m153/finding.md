# Finding — enabling journaling replays an older Key Value dataset

**Severity: major.** A settings save completes, the store becomes Available and accepts TLS commands, but data acknowledged and saved under Snapshot only disappears when Journal + Snapshot is enabled. The counter rolls back as well. This is a missing persistence conversion, not a failed connection or ordinary unsaved-snapshot risk.

## Fixture and two reproductions

Production `https://dashboard.bex.co/keyvalue/red-davps56de41s73canv6g`, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`, 2026-10-02. The own fixture was **Free**, public, named `qa-20261002-kv-durable-r15`; Valkey **8.1.9**, image `valkey/valkey:8-alpine@sha256:a038175878d66b9d274fbf8be73c0305e93798b83917647f167e18cef3c71eec`. It began with the form's Journal + Snapshot default and allkeys-lru. No paid change, production data from another store, pod deletion, data-file edit or restore operation was involved.

Identity was stable throughout: KeyValue UID `e801efcb-5ac8-4e8d-aab7-368ae4c85b65`, StatefulSet UID `324d3ad5-a864-4e85-93a2-19a576feae28`, PVC UID `d429740c-90eb-4dd8-9121-0708d9d7c154`. Pod UIDs changed on the intended setting rollouts.

1. Create the Free store with public access. Reveal and copy its external URL; display and copied value matched the API. Connect over `rediss://` using system CA verification and SNI. Credentials stayed in runtime memory/environment and were never recorded.
2. Write `qa:r15:marker=qa-r15-persist`, `qa:r15:expiring=qa-r15-ttl EX 900`, and increment `qa:r15:counter` to 1.
3. In Persistence Mode, change Journal + Snapshot → Snapshot only. After restart, the baseline values remain and the TTL decreases normally. Runtime `CONFIG GET appendonly` returns `no`.
4. Write `qa:r15:snapshot-new=qa-r15-written-in-snapshot` and increment the counter to 2; read both back. Freshly reload the detail page, edit Persistence Mode and save Journal + Snapshot.
5. After Available, TLS PING succeeds but `snapshot-new` is absent and the counter is **1**, not 2. The older marker and still-unexpired key remain. Runtime `appendonly` is `yes`.
6. **Second fresh-page reproduction with a persisted positive control:** reload and return to Snapshot only. Write `qa:r15:snapshot-two=qa-r15-written-before-save`, set the counter to **23**, and issue `SAVE` → OK. Change Maxmemory Policy to `noeviction`, causing an ordinary restart while keeping Snapshot only. The new marker and 23 **survive** that restart. Reload the page again, save Journal + Snapshot, wait for Available, and read again: `EXISTS snapshot-two` becomes **0**, and the counter reverts to **1**.

### Dated results

| UTC time | Action / observed result |
| --- | --- |
| 12:05:40.972 | Create accepted, status creating, default journal-snapshot |
| 12:07:33.071829 | Initial TLS probes: marker present, counter 1, TTL 897, appendonly yes |
| 12:08:15.147 | Save snapshot, GraphQL 200 |
| 12:08:58.037905 | Ordinary journal→snapshot control passes: marker/counter retained, TTL 812, appendonly no |
| 12:09:41.505064 | New snapshot-only marker reads back; counter 2, TTL 769 |
| 12:09:44.470 | Fresh-page save journal-snapshot, GraphQL 200 |
| 12:10:47.338368 | First loss: new marker absent, counter 1, original marker retained, TTL 703, appendonly yes |
| 12:11:45.879 | Fresh-page save snapshot, GraphQL 200 |
| 12:13:10.421960 | Second marker and counter 23 written; explicit SAVE OK, marker EXISTS 1, TTL 560 |
| 12:13:46.414 | Save noeviction, GraphQL 200; persistence stays snapshot |
| 12:14:48.717656 | Hard control after ordinary restart: second marker EXISTS 1, value correct, counter 23, TTL 462, appendonly no, policy noeviction |
| 12:15:11.752 | Fresh-page save journal-snapshot, GraphQL 200 |
| 12:16:03.278680 | Second loss: second marker EXISTS 0, counter 1, older marker retained, TTL 387, appendonly yes |
| 12:16:23.164–.697 | REST, GraphQL and MCP all report available / journal_snapshot / noeviction |

Expected: switching between durable modes preserves the current acknowledged keyspace, including non-expiring values, counter updates and original expiration deadlines. A failed conversion must retain the current safe workload/data and expose an actionable transition/failure, not publish a stale dataset as completed. Changes to/from Off have a distinct deliberate data-loss contract; neither reproduction used Off.

The UI currently says “Saving restarts the store with its data kept,” then returns to Available with Journal + Snapshot selected. The screenshot supports that copy/state claim; the complete command responses below prove the loss. The strongest control includes both explicit `SAVE` and an intervening real restart, so unsaved snapshot writes, TTL expiration, memory eviction and a client cache do not explain the result. Redis CLI opens a fresh verified TLS connection for every command; the second loss occurred under `noeviction` and the missing marker had no TTL.

No pre-cleanup browser console warnings/errors were reported. The own-resource 404 after deletion is expected. A first copy attempt used the wrong automation label (“Copy External URL”); it was corrected to the actual “Copy External Key Value URL” and passed. That locator timeout is not a product finding.

## Root cause: desired flags skip a required data handoff

1. **UI and API producer:** `dashboard/src/features/keyvalue/hooks/use-set-key-value-persistence-mode.ts:30–57` submits the selected mode and refetches; the section at `components/key-value-persistence-mode-section.tsx:29–85` and copy at `locales/en.ts:461–475` promise a data-preserving restart. The complete captured writes show the intended setting, including the reverse transition.
2. **Shared spec mutation:** `lego/backend/internal/keyvalue/service.go:768–775,814–836,858–907` validates the enum, normalizes underscore/hyphen spellings, writes `Spec.PersistenceMode`, and records a config restart. REST `rest.go:358–376`, GraphQL `graphql.go:235–237` and MCP `mcp.go:170–182` converge here. They do not convert the running dataset.
3. **Consumer that performs the destructive transition:** `lego/operator/internal/controller/keyvalue_controller.go:177–187` maps snapshot to `--appendonly no` and journal-snapshot to `--appendonly yes`. `applyValkeyPodSpec:666–718` puts those flags in the pod spec; `applyKeyValueStatefulSet:620–658` reapplies the template and `reconcileKeyValueWorkload:609–615` writes it before the status path at `803–846`. There is no persistence handoff in that path. It rolls the pod over the same PVC while the old journal files remain.
4. **Actual dependency opened, not guessed:** controller-runtime **v0.23.3**, `pkg/controller/controllerutil/controllerutil.go:321`, fetches the object, applies the mutation and calls Update when changed; it supplies no datastore migration. More directly, the exact live Valkey **8.1.9** startup path [server.c, loadDataFromDisk](https://github.com/valkey-io/valkey/blob/8.1.9/src/server.c#L6190) loads the AOF manifest when AOF is enabled; the RDB-load branch is the alternative, not a newer-file fallback. Its [aof.c loader](https://github.com/valkey-io/valkey/blob/8.1.9/src/aof.c#L1513) follows the existing base and incremental journal files.
5. **Live filesystem/log control matches that mechanism:** after the first loss, `/data/dump.rdb` was dated 12:09, while `appendonly.aof.1.incr.aof` was dated 12:07 and the manifest/base 12:06. The new pod logged loading that base and incremental AOF. After the ordinary Snapshot-only policy restart, the log instead reported loading four keys from the RDB on disk. After the second journal enable, it again logged the old AOF load. The second startup transcript is retained below.
6. **Positive control and existing implementation precedent:** Valkey's [persistence guide](https://valkey.io/topics/persistence/#how-i-can-switch-to-aof-if-im-currently-using-dumprdb-snapshots) requires enabling AOF on the live dataset and completing its rewrite before restarting. The versioned [startAppendOnly](https://github.com/valkey-io/valkey/blob/8.1.9/src/aof.c#L888) enters a rewrite state rather than treating old AOF files as current. This repository's `scripts/restore-keyvalue.sh:253–302` already verifies an RDB-loaded marker, enables AOF on the running throwaway target, waits/rewrites, restarts with AOF enabled and verifies again. Ordinary settings never call that recovery script. Reuse the transition principle, not its throwaway-only orchestration or an insufficient “rewrite not running” check by itself.

### Proposed target and layer

Fix this in the **operator's managed Key Value transition**, so API, Blueprint and direct CR writers share it. Before a snapshot→journal desired spec can replace the current working pod, convert the current authoritative dataset to a complete journal, verify completion, and only then roll into the durable desired configuration. An authenticated live `CONFIG SET appendonly yes` followed by bounded verification of enabled state, no scheduled/in-progress rewrite and successful rewrite completion is the existing upstream/repo precedent. An equivalent safe staged conversion is acceptable if it proves the same data/ordering guarantees; merely deleting the old AOF or choosing a file by modification time is not sufficient.

The handoff must be idempotent across reconcile retries, desired-mode changes and controller/process restarts. A conversion failure must not discard the old authoritative dataset or mark the transition complete. Keep credentials from the owned auth Secret out of args/logs/artifacts; keep the operator DB-free and preserve `operator → types ← backend`.

**The type can express a transition, but currently records no applied persistence mode:** `lego/types/v1alpha1/keyvalue_types.go:87–98` permits the three desired modes; status at `145–218` has Pending/Provisioning/Ready/Failed, generation and Conditions. Use a persisted, generation-aware transition/applied-mode record only if needed; add its type/codegen coherently rather than inventing an unregistered phase. Existing Provisioning/config_restart and a condition reason can represent pending conversion. Do not confuse desired mode accepted by PATCH with safe runtime completion.

Before settling, retain the existing page and current dataset while showing a truthful operation state. Keep existing protected-environment confirmation, tenant authorization and validation behavior. Unauthenticated/forbidden/not-found/timeout responses must keep their existing meanings; this is not a reason to disclose credentials or resource existence.

## Blast radius and aliases

The fix is **global for managed Key Value persistence transitions**, not a dashboard-only check.

- Exhaustive non-test Go search found **one production caller of `valkeyArgs`** (`applyValkeyPodSpec:676`); each of `applyValkeyPodSpec`, `applyKeyValueStatefulSet` and `reconcileKeyValueWorkload` likewise has one production caller in this controller. This single projection covers all three Key Value plans and supported major versions 7 and 8, including legacy empty mode (journal behavior).
- Existing-resource persistence has **two application-owned spec write paths**: `KeyValuePatch.apply` (`keyvalue/service.go:835–836`) and `ApplyBlueprintKeyValueSpec` (`apps/blueprint_plan.go:428–429`). The latter has **four non-test callers**: two planning probes at `blueprint_state_plan.go:238,243`, a change probe at `deploy.go:3025`, and the live writer at `deploy.go:3123` which patches the CR. Initial create constructors are `keyvalue/service.go:470` and `apps/deploy.go:2524`; direct Kubernetes/desired-YAML changes reach the same reconciler.
- The patch writer has **three persistence-setting public adapters**: PATCH `/v1/key-value/{id}`, GraphQL `setKeyValuePersistenceMode`, and MCP `update_key_value(persistenceMode:)`. The dashboard calls that GraphQL setter; the pinned CLI uses REST. REST/MCP writes and Blueprint/direct-CR updates were not repeated live in this hunt.
- UI canonical `/keyvalue/<id>` and Render-shaped `/r/<id>` alias share this resource. `journal_snapshot` and `journal-snapshot` inputs normalize to the same CRD mode. Omitted/nil mode remains unchanged; malformed modes remain validation errors. No separate Redis-named mutation is registered in the inspected keyvalue adapter files.
- Seven resource families: **Key Value affected**. **Postgres, web, static, cron, worker and private services** do not use this KeyValue projection. Their generic API/helper behaviors must stay unchanged; this finding supplies no evidence of data loss in them.
- Adjacent cases to verify: snapshot→journal with no prior AOF; stale base/incremental/manifest variants; journal→snapshot control; repeated same-mode saves; legacy empty mode; all transitions involving Off with honest destructive semantics and no old-data resurrection; suspended stores; controller interruption; write activity during rewrite; exhausted disk/failed rewrite; and plan/version/eviction changes sharing the workload projection. Paid/version-7/failure cases belong in isolated local tests, not purchases or destructive experiments on others' stores.

## Dedupe and prior acceptance

Searched open and done board entries for AOF/appendonly, persistence loss, stale journals and snapshot transitions; scanned **42 open/blocked milestone READMEs**; reread DO_NOT_DO; reviewed the latest 40 dashboard/lego code commits and targeted PersistenceMode history. No open equivalent or anti-goal match. Post-create mutation landed in **821bc3685**; no later normal-settings conversion was found. Analysis HEAD: **dcce61131**. The observed headless Service is still the old readiness behavior, but the same unsafe persistence-flag projection remains on main, so deploying w1/m166 alone does not solve this data issue.

**Classification:** uncovered runtime correctness gap in w6/done/m127's post-create persistence work and w4/done/066's UI control, not a recurrence of the old forced-Off creation default or missing setter. The complete older milestone DoD is dispositioned here:

| w6/m127 DoD | Result / boundary in this hunt |
| --- | --- |
| Dashboard and REST free create defaults agree | Dashboard create sent journal-snapshot and runtime AOF was on. REST create was not repeated in sweep 15; retain the original create-path regression checks. |
| No surface claims Free has no disk | Current create/detail copy advertises durability; source search of en/zh and create route found none of the old claim/key. Free durability remains an intentional bex divergence. |
| Persistence updates on REST/GraphQL/MCP | GraphQL writes and all three reads pass for desired mode; REST/MCP write adapters were source-traced. Accepted settings do not prove data survived their runtime application — this is the uncovered gap. |
| Existing Off store reaches journal without recreation | Not exercised; Off's data-loss semantics and no historical-data resurrection belong to t003. Do not claim old Off data was made durable by this fix. |
| Maxmemory still updates | Passed live: noeviction selected, runtime confirmed, snapshot marker and counter 23 survived its ordinary restart. |
| ADR018 corrects the false hardware premise and update surface | ADR021 and ADR018 retain free persistence and post-create settings. Update their runtime-transition explanation after implementation without reviving the false free-plan constraint. |

The older **t002** also explicitly required documenting data consequences in each direction. Its outcome records only flag derivation and pod rollout; the current “data kept” copy is contradicted by the captured durable→durable transition. Preserve its shared validation, dry-run behavior and underscore/hyphen acceptance; those auxiliary cases were not all replayed live.

**Distinct related work:**

- w7/done/m68, w7/done/m69 t004 and w8/done/m30 t006 handle **throwaway backup restores**, including RDB→AOF conversion and marker verification. They are a working mechanism precedent; their backup/restore DoD is not re-run or declared regressed here.
- w4/blocked/m137 / w1/blocked/m166 cover lifecycle labels and available-but-unreachable windows. This hunt's client is reachable and authenticates **against the wrong old keyspace**. A stronger readiness PING cannot detect it.
- w4/m149 owns leftover Key Value TLS Secrets. That known residue recurred during cleanup; no duplicate filing is created.

**Render comparison:** [Render's current Key Value documentation](https://render.com/docs/key-value#data-persistence) distinguishes Journal + Snapshot, Snapshot only and Off, describes restart on mode change, and warns about loss when switching to or from Off. It also excludes persistence on Render Free; bex deliberately provides it. No authenticated Render instance was mutated, and no paid plan was purchased. The second reproduction explicitly persisted the snapshot and survived an ordinary restart, so this is not an assertion that snapshot mode protects unsaved writes from arbitrary failures.

**Unverified:** all public write adapters beyond dashboard GraphQL, Blueprint/direct CR transitions, version 7, initially-snapshot stores with no AOF, Off transitions, large/concurrent datasets, rewrite failures, crash recovery and suspend/resume races. The proposed safe conversion is researched from primary source and existing restore code; it was not manually applied to production in this hunt.

## Complete data-plane probes

The external client used `redis-cli --tls --sni <owned-host> --cacert /etc/ssl/cert.pem -h <owned-host> -p 6379 --raw --user default`; password supplied only through runtime `REDISCLI_AUTH`, not argv. Each listed command was a separate process. Blank raw GET output was further disambiguated by EXISTS=0 in reproduction two. These complete command/result objects contain only own fixture data, not connection secrets.

### initial — 2026-10-02T12:07:33.071829+00:00

```json
{
  "label": "initial",
  "at": "2026-10-02T12:07:33.071829+00:00",
  "mode": "init",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["SET", "qa:r15:marker", "qa-r15-persist"],
      "exit": 0,
      "stdout": "OK\n",
      "stderr": ""
    },
    {
      "command": ["SET", "qa:r15:expiring", "qa-r15-ttl", "EX", "900"],
      "exit": 0,
      "stdout": "OK\n",
      "stderr": ""
    },
    {
      "command": ["INCR", "qa:r15:counter"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "897\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nyes\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nallkeys-lru\n",
      "stderr": ""
    }
  ]
}
```

### after-snapshot — 2026-10-02T12:08:58.037905+00:00

```json
{
  "label": "after-snapshot",
  "at": "2026-10-02T12:08:58.037905+00:00",
  "mode": "after-snapshot",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "812\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nno\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nallkeys-lru\n",
      "stderr": ""
    }
  ]
}
```

### snapshot-write — 2026-10-02T12:09:41.505064+00:00

```json
{
  "label": "snapshot-write",
  "at": "2026-10-02T12:09:41.505064+00:00",
  "mode": "snapshot-write",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["SET", "qa:r15:snapshot-new", "qa-r15-written-in-snapshot"],
      "exit": 0,
      "stdout": "OK\n",
      "stderr": ""
    },
    {
      "command": ["INCR", "qa:r15:counter"],
      "exit": 0,
      "stdout": "2\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "qa-r15-written-in-snapshot\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "769\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "2\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nno\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nallkeys-lru\n",
      "stderr": ""
    }
  ]
}
```

### after-journal-return — 2026-10-02T12:10:47.338368+00:00

```json
{
  "label": "after-journal-return",
  "at": "2026-10-02T12:10:47.338368+00:00",
  "mode": "after-journal-return",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "703\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nyes\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nallkeys-lru\n",
      "stderr": ""
    }
  ]
}
```

### second-snapshot-baseline — 2026-10-02T12:12:44.995639+00:00

```json
{
  "label": "second-snapshot-baseline",
  "at": "2026-10-02T12:12:44.995639+00:00",
  "mode": "second-snapshot-baseline",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "585\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nno\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nallkeys-lru\n",
      "stderr": ""
    }
  ]
}
```

### second-snapshot-write-and-save — 2026-10-02T12:13:10.421960+00:00

```json
{
  "label": "second-snapshot-write-and-save",
  "at": "2026-10-02T12:13:10.421960+00:00",
  "mode": "snapshot-write-two",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["SET", "qa:r15:snapshot-two", "qa-r15-written-before-save"],
      "exit": 0,
      "stdout": "OK\n",
      "stderr": ""
    },
    {
      "command": ["SET", "qa:r15:counter", "23"],
      "exit": 0,
      "stdout": "OK\n",
      "stderr": ""
    },
    {
      "command": ["SAVE"],
      "exit": 0,
      "stdout": "OK\n",
      "stderr": ""
    },
    {
      "command": ["EXISTS", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "0\n",
      "stderr": ""
    },
    {
      "command": ["EXISTS", "qa:r15:snapshot-two"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-two"],
      "exit": 0,
      "stdout": "qa-r15-written-before-save\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "560\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "23\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nno\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nallkeys-lru\n",
      "stderr": ""
    }
  ]
}
```

### snapshot-ordinary-restart-control — 2026-10-02T12:14:48.717656+00:00

```json
{
  "label": "snapshot-ordinary-restart-control",
  "at": "2026-10-02T12:14:48.717656+00:00",
  "mode": "after-policy-control",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["EXISTS", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "0\n",
      "stderr": ""
    },
    {
      "command": ["EXISTS", "qa:r15:snapshot-two"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-two"],
      "exit": 0,
      "stdout": "qa-r15-written-before-save\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "462\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "23\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nno\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nnoeviction\n",
      "stderr": ""
    }
  ]
}
```

### after-second-journal-return — 2026-10-02T12:16:03.278680+00:00

```json
{
  "label": "after-second-journal-return",
  "at": "2026-10-02T12:16:03.278680+00:00",
  "mode": "after-second-journal-return",
  "host": "red-davps56de41s73canv6g.kv.bex.co",
  "tls": true,
  "trust": "/etc/ssl/cert.pem",
  "probes": [
    {
      "command": ["PING"],
      "exit": 0,
      "stdout": "PONG\n",
      "stderr": ""
    },
    {
      "command": ["EXISTS", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "0\n",
      "stderr": ""
    },
    {
      "command": ["EXISTS", "qa:r15:snapshot-two"],
      "exit": 0,
      "stdout": "0\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-two"],
      "exit": 0,
      "stdout": "\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:snapshot-new"],
      "exit": 0,
      "stdout": "\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:marker"],
      "exit": 0,
      "stdout": "qa-r15-persist\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "qa-r15-ttl\n",
      "stderr": ""
    },
    {
      "command": ["TTL", "qa:r15:expiring"],
      "exit": 0,
      "stdout": "387\n",
      "stderr": ""
    },
    {
      "command": ["GET", "qa:r15:counter"],
      "exit": 0,
      "stdout": "1\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "appendonly"],
      "exit": 0,
      "stdout": "appendonly\nyes\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "save"],
      "exit": 0,
      "stdout": "save\n3600 1 300 100 60 10000\n",
      "stderr": ""
    },
    {
      "command": ["CONFIG", "GET", "maxmemory-policy"],
      "exit": 0,
      "stdout": "maxmemory-policy\nnoeviction\n",
      "stderr": ""
    }
  ]
}
```

## Complete API requests and responses

Authenticated page fetches used credentials: include. GraphQL requests below are the actual UI batches, Content-Type application/json. REST/MCP reads demonstrate desired mode/status agreement, not data correctness.

### CreateKeyValue — 2026-10-02T12:05:40.972Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
[
  {
    "operationName": "CreateKeyValue",
    "variables": {
      "name": "qa-20261002-kv-durable-r15",
      "ownerId": "tea-d98210cbbpdc73dcrkvg",
      "plan": "free",
      "public": true,
      "maxmemoryPolicy": "allkeys-lru",
      "persistenceMode": "journal-snapshot"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation CreateKeyValue($name: String!, $ownerId: String, $environmentId: String, $plan: String, $version: String, $storageGB: Int, $public: Boolean, $maxmemoryPolicy: String, $persistenceMode: String) {\n  createKeyValue(\n    name: $name\n    ownerId: $ownerId\n    environmentId: $environmentId\n    plan: $plan\n    version: $version\n    storageGB: $storageGB\n    public: $public\n    maxmemoryPolicy: $maxmemoryPolicy\n    persistenceMode: $persistenceMode\n  ) {\n    id\n    name\n    plan\n    status\n    projectId\n    environmentId\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "createKeyValue": {
        "__typename": "KeyValue",
        "environmentId": null,
        "id": "red-davps56de41s73canv6g",
        "name": "qa-20261002-kv-durable-r15",
        "plan": "free",
        "projectId": null,
        "status": "creating"
      }
    }
  }
]
```

### SetKeyValuePersistenceMode — 2026-10-02T12:08:15.147Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
[
  {
    "operationName": "SetKeyValuePersistenceMode",
    "variables": {
      "id": "red-davps56de41s73canv6g",
      "persistenceMode": "snapshot"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetKeyValuePersistenceMode($id: String!, $persistenceMode: String!) {\n  setKeyValuePersistenceMode(id: $id, persistenceMode: $persistenceMode) {\n    id\n    persistenceMode\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "setKeyValuePersistenceMode": {
        "__typename": "KeyValue",
        "id": "red-davps56de41s73canv6g",
        "persistenceMode": "snapshot"
      }
    }
  }
]
```

### SetKeyValuePersistenceMode — 2026-10-02T12:09:44.470Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
[
  {
    "operationName": "SetKeyValuePersistenceMode",
    "variables": {
      "id": "red-davps56de41s73canv6g",
      "persistenceMode": "journal-snapshot"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetKeyValuePersistenceMode($id: String!, $persistenceMode: String!) {\n  setKeyValuePersistenceMode(id: $id, persistenceMode: $persistenceMode) {\n    id\n    persistenceMode\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "setKeyValuePersistenceMode": {
        "__typename": "KeyValue",
        "id": "red-davps56de41s73canv6g",
        "persistenceMode": "journal_snapshot"
      }
    }
  }
]
```

### SetKeyValuePersistenceMode — 2026-10-02T12:11:45.879Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
[
  {
    "operationName": "SetKeyValuePersistenceMode",
    "variables": {
      "id": "red-davps56de41s73canv6g",
      "persistenceMode": "snapshot"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetKeyValuePersistenceMode($id: String!, $persistenceMode: String!) {\n  setKeyValuePersistenceMode(id: $id, persistenceMode: $persistenceMode) {\n    id\n    persistenceMode\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "setKeyValuePersistenceMode": {
        "__typename": "KeyValue",
        "id": "red-davps56de41s73canv6g",
        "persistenceMode": "snapshot"
      }
    }
  }
]
```

### SetKeyValueMaxmemoryPolicy — 2026-10-02T12:13:46.414Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
[
  {
    "operationName": "SetKeyValueMaxmemoryPolicy",
    "variables": {
      "id": "red-davps56de41s73canv6g",
      "maxmemoryPolicy": "noeviction"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetKeyValueMaxmemoryPolicy($id: String!, $maxmemoryPolicy: String!) {\n  setKeyValueMaxmemoryPolicy(id: $id, maxmemoryPolicy: $maxmemoryPolicy) {\n    id\n    maxmemoryPolicy\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "setKeyValueMaxmemoryPolicy": {
        "__typename": "KeyValue",
        "id": "red-davps56de41s73canv6g",
        "maxmemoryPolicy": "noeviction"
      }
    }
  }
]
```

### SetKeyValuePersistenceMode — 2026-10-02T12:15:11.752Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
[
  {
    "operationName": "SetKeyValuePersistenceMode",
    "variables": {
      "id": "red-davps56de41s73canv6g",
      "persistenceMode": "journal-snapshot"
    },
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "mutation SetKeyValuePersistenceMode($id: String!, $persistenceMode: String!) {\n  setKeyValuePersistenceMode(id: $id, persistenceMode: $persistenceMode) {\n    id\n    persistenceMode\n    __typename\n  }\n}"
  }
]
```

Complete response:

```json
[
  {
    "data": {
      "setKeyValuePersistenceMode": {
        "__typename": "KeyValue",
        "id": "red-davps56de41s73canv6g",
        "persistenceMode": "journal_snapshot"
      }
    }
  }
]
```

### REST after data loss — 2026-10-02T12:16:23.164Z

GET `https://api.bex.co/v1/key-value/red-davps56de41s73canv6g`, HTTP 200.

No request body.

Complete response:

```json
{
  "id": "red-davps56de41s73canv6g",
  "name": "qa-20261002-kv-durable-r15",
  "plan": "free",
  "status": "available",
  "suspended": "not_suspended",
  "createdAt": "2026-10-02T12:05:40Z",
  "updatedAt": "2026-10-02T12:15:31Z",
  "owner": {
    "id": "tea-d98210cbbpdc73dcrkvg",
    "name": "bex",
    "email": "puncsky@gmail.com",
    "type": "team"
  },
  "region": "fsn1",
  "dashboardUrl": "https://dashboard.bex.co/r/red-davps56de41s73canv6g",
  "version": "8",
  "options": {
    "maxmemoryPolicy": "noeviction",
    "persistenceMode": "journal_snapshot"
  },
  "ipAllowList": [],
  "externalHost": "red-davps56de41s73canv6g.kv.bex.co",
  "public": true
}
```

### GraphQL after data loss — 2026-10-02T12:16:23.458Z

POST `https://api.bex.co/graphql`, HTTP 200.

Request:

```json
{
  "query": "query QAPersistence($id:String!){keyValue(id:$id){id name status plan maxmemoryPolicy persistenceMode}}",
  "variables": {
    "id": "red-davps56de41s73canv6g"
  }
}
```

Complete response:

```json
{
  "data": {
    "keyValue": {
      "id": "red-davps56de41s73canv6g",
      "maxmemoryPolicy": "noeviction",
      "name": "qa-20261002-kv-durable-r15",
      "persistenceMode": "journal_snapshot",
      "plan": "free",
      "status": "available"
    }
  }
}
```

### MCP after data loss — 2026-10-02T12:16:23.697Z

POST `https://api.bex.co/mcp`, HTTP 200. Content-Type application/json; Accept application/json, text/event-stream.

Request:

```json
{
  "jsonrpc": "2.0",
  "id": 151,
  "method": "tools/call",
  "params": {
    "name": "get_key_value",
    "arguments": {
      "keyValueId": "red-davps56de41s73canv6g"
    }
  }
}
```

Complete response:

```text
event: message
data: {"jsonrpc":"2.0","id":151,"result":{"content":[{"type":"text","text":"{\"createdAt\":\"2026-10-02T12:05:40Z\",\"dashboardUrl\":\"https://dashboard.bex.co/r/red-davps56de41s73canv6g\",\"externalHost\":\"red-davps56de41s73canv6g.kv.bex.co\",\"id\":\"red-davps56de41s73canv6g\",\"ipAllowList\":[],\"maxmemoryPolicy\":\"noeviction\",\"name\":\"qa-20261002-kv-durable-r15\",\"ownerId\":\"tea-d98210cbbpdc73dcrkvg\",\"persistenceMode\":\"journal_snapshot\",\"plan\":\"free\",\"public\":true,\"region\":\"fsn1\",\"status\":\"available\",\"suspended\":\"not_suspended\",\"updatedAt\":\"2026-10-02T12:15:31Z\",\"version\":\"8\"}"}],"structuredContent":{"createdAt":"2026-10-02T12:05:40Z","dashboardUrl":"https://dashboard.bex.co/r/red-davps56de41s73canv6g","externalHost":"red-davps56de41s73canv6g.kv.bex.co","id":"red-davps56de41s73canv6g","ipAllowList":[],"maxmemoryPolicy":"noeviction","name":"qa-20261002-kv-durable-r15","ownerId":"tea-d98210cbbpdc73dcrkvg","persistenceMode":"journal_snapshot","plan":"free","public":true,"region":"fsn1","status":"available","suspended":"not_suspended","updatedAt":"2026-10-02T12:15:31Z","version":"8"}}}
```

## Startup proof and cleanup

Ordinary Snapshot-only policy restart: Pod UID `1f92360f-9f07-48c3-a007-7f32e0cda940`, created 12:13:47Z, logged at 12:13:58.929Z that four keys loaded from the RDB and the database loaded from disk. The marker and counter 23 read back over TLS afterwards.

Second journal restart: Pod UID `9f040908-c95f-4208-baab-0383a41d1019`, created 12:15:13Z. The captured startup evidence below records a 540-second-old base AOF and loads the base plus incremental AOF before reporting ready. The observed values match the old three-key baseline; the saved snapshot marker is missing and the counter has reverted. The exact runtime PING/GET/EXISTS responses above establish the tenant-visible result.

```json
{
  "at": "2026-10-02T12:16:06.157216+00:00",
  "podUID": "9f040908-c95f-4208-baab-0383a41d1019",
  "podCreatedAt": "2026-10-02T12:15:13Z",
  "logs": [
    "1:M 02 Oct 2026 12:15:20.274 * Configuration loaded",
    "1:M 02 Oct 2026 12:15:20.572 * Reading RDB base file on AOF loading...",
    "1:M 02 Oct 2026 12:15:20.572 * Loading RDB produced by Valkey version 8.1.9",
    "1:M 02 Oct 2026 12:15:20.572 * RDB age 540 seconds",
    "1:M 02 Oct 2026 12:15:20.572 * RDB memory usage when created 0.84 Mb",
    "1:M 02 Oct 2026 12:15:20.572 * RDB is base AOF",
    "1:M 02 Oct 2026 12:15:20.572 * Done loading RDB, keys loaded: 0, keys expired: 0.",
    "1:M 02 Oct 2026 12:15:20.572 * DB loaded from base file appendonly.aof.1.base.rdb: 0.002 seconds",
    "1:M 02 Oct 2026 12:15:20.579 * DB loaded from incr file appendonly.aof.1.incr.aof: 0.006 seconds",
    "1:M 02 Oct 2026 12:15:20.579 * DB loaded from append only file: 0.009 seconds",
    "1:M 02 Oct 2026 12:15:20.579 * Opening AOF incr file appendonly.aof.1.incr.aof on server start",
    "1:M 02 Oct 2026 12:15:20.579 * Ready to accept connections tcp",
    "1:M 02 Oct 2026 12:15:20.579 * Ready to accept connections tls"
  ]
}
```

Supplementary paths were checked on disk (gitignored; durable probes are inline above):

- `.playwright-mcp/qa-kv-r15-journal-data-loss.png` — Available, Journal + Snapshot and the “data kept” claim, inspected.
- `.playwright-mcp/qa-r15-evidence.json` — complete UI mutations, API reads, credential-free client probes and observations.
- `.playwright-mcp/qa-r15-kv-before.json` — original workload/PVC identities, Valkey image digest and Ready state.
- `.playwright-mcp/qa-r15-second-journal-load.json` — second AOF startup transcript.
- `.playwright-mcp/qa-r15-final-cleanup.json` — empty fixture inventory after known TLS-residue cleanup.

Rename at 12:17:13.777Z changed only the display name to `qa-20261002-kv-renamed-r15`; a fresh-page read at 12:17:30.371Z found all connection fields unchanged. Delete through its exact confirmation succeeded at 12:17:46.996Z. GET `/v1/key-value/red-davps56de41s73canv6g` at 12:17:47.708Z returned HTTP 404 with complete body:

```json
{
  "error": "not found",
  "id": "not_found",
  "message": "not found"
}
```

Overview no longer listed it. Workloads, Certificate, PVC and owned auth/connection Secrets disappeared. The known ownerless TLS Secret `red-davps56de41s73canv6g-kv-tls` (UID `a27a166b-ce23-4e2c-b003-21bfd2fd09c3`) was verified unreferenced and removed by exact identity at 12:18:43Z; final inventory at **12:18:51.278574Z** had **no items and no Secrets**. Only this QA session was revoked, credential files removed and browser cookies cleared. No product fix or fleet mutation was made.
