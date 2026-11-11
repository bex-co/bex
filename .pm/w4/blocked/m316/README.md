# w4 · m316 — Changing a Persistence Off Key Value's maxmemory policy says first that the restart deletes every key, and asks for the phrase, as the plan and persistence changes do

**Worker:** worker4 **Goal:** a Key Value with Persistence Off never loses its keys to a maxmemory policy change without being warned. On an Off store, the dashboard asks for `sudo clear key value <name>` and says every key will be deleted. On a persisted store it names the restart. The API descriptions and the Blueprint path say the same thing. **Status:** blocked — t001–t003 and t005–t007 done (shipped); t004 live replay waits for the next production deploy, then t008 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Dashboard: Off-aware confirmation on the maxmemory save, and card copy that names the data loss — **DONE** | 40m | — |
| [t002](done/t002.md) | API descriptions and the Blueprint path for a maxmemory change on a running Off store — **DONE** | 30m | — |
| [t003](done/t003.md) | Writer audit: every write that restarts a running Off store warns or asks — **DONE** | 25m | t001, t002 |
| [t004](t004.md) | Live verification on an owned Free Key Value | 25m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Eight tasks, 195m (3h15m). **Major:** the change deletes customer data without warning.

This is a residual of two done milestones:

- [w4/m263](../../done/m263/README.md). Its expected outcome was that "the page's three restarting controls (Instance type, Maxmemory, Persistence) warn in the same way". It treated the Maxmemory card's "connections drop" copy as the precedent to follow, and called the plan change "the last silent restart".
- [w4/m233](../../done/m233/README.md). Its finding states that "A store with Persistence Off must never lose its data", meaning not without the typed confirmation.

## Finding

Live `/qa-find-bugs w4` loop, round 23, 2026-11-10 14:15–14:45 UTC (`muse.env`, workspace `bex-canary`, pin `c12cd57facb1`). Owned Free Key Value `qa-20261110-kv23` (`red-dbpifls5ngvc73e8trjg`), deleted afterwards (GET 404).

1. **Control case.** Persistence Mode → Off went through the existing typed confirmation: `alertdialog "Delete all data and change persistence?"`, phrase `sudo clear key value qa-20261110-kv23`.
2. **Write a marker.**
   - Added an allowlist rule for my /32 with `PATCH ipAllowList`. This store switches its TLS listener at runtime, so nothing restarted.
   - Copied the External Key Value URL with the dashboard's Copy button and used it from the clipboard, without ever printing it.
   - `redis-cli --tls --sni red-dbpifls5ngvc73e8trjg.kv.bex.co -u <copied URL> SET qa:marker qa-20261110-kv23` → `OK`.
   - `GET qa:marker` → `qa-20261110-kv23`, and `CONFIG GET maxmemory-policy` → `noeviction`.
3. **Change the policy.** Maxmemory Policy → Edit → allkeys-lru → Save changes. **No dialog appeared** (alertdialog/dialog count 0). The REST status read `config_restart` four times, then `available`.
4. **Read back.**
   - `GET qa:marker` returned nil three times, and `DBSIZE` → `0`.
   - `INFO server` showed `uptime_in_seconds:83`, and `CONFIG GET maxmemory-policy` → `allkeys-lru`.
   - The key was deleted by a change whose only warning is the card copy "Saving restarts the store, so connections drop for up to a minute."

The copied URL connected, so journey 12's "copied URL connects" holds. The clipboard was cleared afterwards.

## Root cause

- **Operator.**
  - The policy is a container argument. `valkeyArgs` appends `--maxmemory <plan budget> --maxmemory-policy <policy>` (`lego/operator/internal/controller/keyvalue_controller.go:227-253`, `:250-251`), so any change alters the StatefulSet pod template and replaces the pod.
  - Persistence Off is `--appendonly no --save ""` (`:239-241`), so the replacement pod starts empty.
- **Backend.**
  - The only Off-store restart guard is `requireRestartConfirmation` (`lego/backend/internal/keyvalue/service.go:1136-1151`). It covers only the first time external access is turned on.
  - `KeyValuePatch.apply` writes `MaxmemoryPolicy` with no persistence check (`:1100-1110`).
  - Every writer that reaches it is unguarded:
    - REST `PATCH /v1/key-value/{id}`, and the `/v1/redis/{id}` alias;
    - GraphQL `setKeyValueMaxmemoryPolicy` (`keyvalue/graphql.go:236-241`) and the update mutation's `maxmemoryPolicy` argument (`:201-216`);
    - MCP `update_key_value` (`keyvalue/mcp.go:180-187`) and `set_key_value_maxmemory_policy` (`:73`);
    - Blueprint `maxmemoryPolicy` (`apps/blueprint_plan.go:613-614`), including the unattended auto-sync worker.
- **Dashboard.**
  - `KeyValueMaxmemoryPolicySection` saves through `EditableFieldRow` with no confirmation (`dashboard/src/features/keyvalue/components/key-value-maxmemory-policy-section.tsx:69-77`).
  - Its copy, `keyvalue.maxmemoryDescription` (`features/keyvalue/locales/en.ts:589`), doesn't mention the Off data loss.
  - The neighbouring cards already handle it:
    - The plan card warns about it (`keyvalue.planDescription`, `en.ts:541-544`; `planPickerConfirmDataLoss`, `:562-567`; added by w4/m263).
    - The persistence card asks for the typed phrase (`key-value-persistence-mode-section.tsx:131`).

## Target behavior

This follows m263's parity-sufficient pattern.

- **Dashboard, Persistence Off store.** Saving a maxmemory policy change opens a destructive confirmation. It says the restart deletes every key, and its button enables only after `sudo clear key value <name>` is typed.
- **Dashboard, persisted store.** The confirmation states the restart and the outage, with no phrase.
- **Card copy.** It says data survives according to the persistence mode, and that Off loses every key. In en and zh.
- **API.** REST keeps Render's contract: the change is still accepted, with no new refusal that would break the pinned CLI. The MCP and GraphQL descriptions state the restart and the Off data loss, mirroring m263's plan-change decision.
- **Blueprint.** Follows the decision m263 and w4/358 made for the same class of change.

A runtime `CONFIG SET maxmemory-policy`, which removes the restart altogether, would be a better end state. It is a separate follow-up, not part of this milestone.

## Definition of done

Live on production after the release pins this build, on an owned Free Key Value in `bex-canary`. Nothing paid is ever submitted.

- **Off store asks for the phrase.**
  - Set Persistence Off and write a marker through the copied External URL.
  - Maxmemory Policy → Edit → choose a different policy → Save changes opens a destructive confirmation that names the data loss. Its button stays disabled until `sudo clear key value <name>` is typed.
  - Cancel sends no GraphQL `setKeyValueMaxmemoryPolicy` request, and `GET qa:marker` still returns the marker.
- **Confirming does what the dialog said.** After confirming with the phrase, the store restarts (`config_restart` → `available`), REST reads the new policy back, and `DBSIZE` is 0.
- **Persisted store gets the restart warning only.** With Journal + Snapshot, the same change shows a restart-only confirmation with no phrase, and the marker survives the restart.
- **Copy and API.**
  - The card copy and both dialogs exist in en and zh.
  - The MCP and GraphQL descriptions state the Off data loss.
  - REST `PATCH {maxmemoryPolicy}` still answers 200, as on Render.
- **Blueprint.** Blueprint behavior for a running Off store matches the decision recorded in ADR021.
- **Checks.** Dashboard `yarn lint` and `yarn test`, and the backend real-DB suite, pass.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Unverified (carried into tasks)

- Whether Render restarts on a maxmemory change. render.com/docs/key-value says persistence changes restart, but says nothing about maxmemory (t005).
- The plan and version paths on an Off store were not exercised live this round. Plan is paid, and m263 covers the plan copy (t003 audits them).
- The MCP and GraphQL maxmemory writes were read from source (t002, t004).

## Dedupe

- These done items cover the sibling restarts:
  - w4/m263: plan change.
  - w4/m233: allowlist restarts.
  - w4/358: Blueprint `ipAllowList`.
- w4/m153 (done) exercised a maxmemory restart only on Snapshot only, where the data was kept.
- `grep -rliE "maxmemory.{0,80}(Persistence Off|data loss|deletes every key)" .pm` finds no open item.
- Not deploy lag: `git log c12cd57facb1..origin/main` on `dashboard/src/features/keyvalue`, `lego/backend/internal/keyvalue` and `keyvalue_controller.go` changes no maxmemory or restart guard. The only commits are w8/237, w8/m149 and w2/m307.
- No `.pm/DO_NOT_DO.md` entry applies.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 23, 2026-11-10 (the probe above; fixture deleted).
- **Goal linkage:** [ADR021](../../../../docs/ADR021-keyvalue-management.md) (Key Value persistence and restarts), [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), and truthful product state.
- **Expected outcome:** no Persistence Off Key Value loses its keys from a settings change it wasn't warned about. The Instance type, Maxmemory and Persistence cards finally warn in the same way, as m263 intended.
- **Why now:** it deletes customer data silently, on the setting a user tunes when a cache starts evicting. That is exactly when the store holds data they care about.
- **Render parity included:** the dashboard, the GraphQL and MCP descriptions, and the Blueprint path all change (t005).

## Post-release probe (QA loop rounds 113 and 125, 2026-11-11)

The release gate cleared at `6ee0197c7` (14:43 UTC): production is pinned to `f609163ada8e`, which contains `d69b838ac`. Two `/qa-find-bugs w4` rounds replayed the DoD on owned Free Key Value stores. This is a record for the live-replay task, not a replacement for it, and that task stays todo.

- **Round 113** (`qa-20261111-r113-kv`, `red-dbq82p03u30c73cf8et0`): after Persistence → Off (phrase `sudo clear key value <name>`), Maxmemory Policy → noeviction → Save opened "Change the maxmemory policy to noeviction?", which reads "…Persistence is Off, so the restart deletes every key in qa-20261111-r113-kv. This cannot be undone." "Delete data and change policy" stays disabled until the phrase is typed. The zh dialog matches ("…重启会删除 qa-20261111-r113-kv 中的所有键，且无法撤销。"). Deleted afterwards.
- **Round 125** (`qa-20261111-r125-kv`, `red-dbqan1npvbns739s7jbg`, created with this machine's IP allowlisted; deleted afterwards, DELETE 204 then GET 404). Commands used the External URL copied from the dashboard's Copy button. **Every probe passed:**
  - **Persisted store gets the restart warning only.**
    - With Journal + Snapshot, `SET qa:marker persisted-1`, then Maxmemory Policy → volatile-lru.
    - The dialog reads "The store restarts to apply the new policy and is unavailable for up to a minute. Connections drop." It has no phrase field.
    - "Restart and change policy" took the store `config_restart` (17:54:24) → `available` (17:54:44). `GET qa:marker` → `persisted-1`, and `CONFIG GET maxmemory-policy` → `volatile-lru`.
  - **The Off store asks for the phrase.**
    - After Persistence → Off (phrase; `DBSIZE` 0), `SET qa:marker off-1`, then Maxmemory Policy → allkeys-lfu.
    - The phrase dialog opened as in round 113. Cancel sent no GraphQL mutation (request listener: none). `GET qa:marker` still returned `off-1`, and the policy stayed `volatile-lru`.
  - **Confirming does what the dialog said.** Typing the phrase and pressing "Delete data and change policy" took the store `config_restart` (17:56:35) → `available` (17:56:58). REST reads `maxmemoryPolicy: allkeys_lfu`, `DBSIZE` → 0, `GET qa:marker` → empty, and `CONFIG GET maxmemory-policy` → `allkeys-lfu`.
  - **REST still answers 200, as on Render.** `PATCH /v1/key-value/<id> {"maxmemoryPolicy":"noeviction"}` on the Off store → 200, `options.maxmemoryPolicy: noeviction`.
- **Not run:** the MCP and GraphQL descriptions, the Blueprint path, and the CI checks.
