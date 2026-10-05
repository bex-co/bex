# w5 · m110 — Key Value: platform clients log in as a platform ACL user, so locking down the tenant user can't break them

**Worker:** worker5 **Goal:** The tenant `default` user is denied admin commands by category while the operator, exporter, probe and backup keep what they need, which fixes the persistence-switch regression from w4/191. **Status:** blocked — t001–t004, t006–t008 done 2026-10-05; t005 needs a deployed operator for the live replay; t009 closeout follows

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Boot the Valkey engine test with production args — **DONE** | 30m | — |
| t002 | Add a platform ACL user with its own secret — **DONE** | 45m | t001 |
| t003 | Move live-prep, exporter, probe and backup to the platform user — **DONE** | 1h | t002 |
| t004 | Deny admin commands to the tenant user by category — **DONE** | 45m | t003 |
| t005 | Roll out to existing Key Values and verify live | 45m | t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006, t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- On dev-5 a running Key Value switches snapshot → journal-snapshot → snapshot through the API with its data intact (w4/m153's replay passes again).
- The tenant `default` user is refused CONFIG SET/REWRITE, MODULE, DEBUG, ACL, REPLICAOF, SHUTDOWN, CLIENT KILL/PAUSE, MONITOR and SYNC, and keeps data commands, CONFIG GET, INFO and CLIENT LIST.
- Exporter scrapes, the readiness probe and backups authenticate as the platform user; no API returns its password.
- The engine test boots Valkey with the production args.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `e9fb4178c` (w4/191). Code-verified at `4baa77e02`: `prepareKeyValueJournalAtPod` (`lego/operator/internal/controller/keyvalue_persistence_live.go:54-60`) logs in with the password only (user `default`) and runs `CONFIG SET appendonly yes` (`:155`), which `valkeyDefaultUserRule` now denies (`keyvalue_controller.go:244-247`, `-config|set`). `deferKeyValuePersistence` then marks the store Failed/`PersistenceTransitionFailed` and retries every 10 s, so the requesting PATCH never takes effect. w4/m153 verified the switch live on 2026-10-02, before w4/191. CI missed it because the engine test runs Valkey with `--requirepass`, not the production args.
- **Goal linkage:** ADR021 Key Value management; ADR008 reliable managed data.
- **Expected outcome:** Persistence-mode changes work again; tenants can't reconfigure or disrupt platform-managed Valkey state; admin verbs added by future Valkey releases are denied by default.
- **Why now:** A shipped regression in a paid feature: every persistence change since w4/191 deployed wedges the store.
- **Render parity included:** the tenant-visible command surface and the persistence PATCH change.

## Evidence — 2026-10-05

- Reproduced: the engine test booted with `valkeyArgs` failed the live handoff with `NOPERM User default has no permissions to run the 'config|set' command` (valkey 8), the exact wedge every Snapshot → Journal + Snapshot switch has hit since w4/191.
- Fixed: `bex` platform user (immutable, tenant-mount-protected `<kv>-platform` Secret) for the handoff, exporter and backup; tenant `default` = `+@all -@admin +config|get +client|list +slowlog|get +slowlog|len`; ADR021 §8.
- Migration: every running store predates the user, so a pending switch first rolls the serving pod once in its current snapshot mode (the same ordinary restart w4/191 made fleet-wide), then journals from the new process. Stores with no pending switch roll once on their next reconcile.
- Verified locally: engine suite on both pinned images (valkey 7, 8) with production args, gate/envtest/unit suites, `make test`, `make lint`; mutation checks on the gate and the handoff's `Username`.
- Not verified locally: a live switch on a running cluster. Every `dev-N` stack shares the one in-cluster bex operator, which this workstream may not replace (AGENTS.md isolation rule), so t005 runs after the release pipeline deploys this change.

## Blocked — t005 live replay (after deploy)

On an owned Free `qa-` Key Value in `bex-canary` (QA session per `/qa-find-bugs`), after the deploy that includes this milestone:

1. Create with Journal + Snapshot; write a baseline key and counter. Switch to Snapshot only; write a new key; switch back to Journal + Snapshot through the API. Expect Available with the baseline, counter and new key intact, and runtime `appendonly yes` (w4/m153's sequence).
2. As the tenant: `CONFIG SET maxmemory 0`, `SAVE`, `MONITOR`, `CLIENT KILL ID 1` → NOPERM; `CONFIG GET maxmemory`, `INFO`, `CLIENT LIST`, `FLUSHALL` → OK; `ACL WHOAMI` → `default`.
3. The Metrics tab shows memory and connected clients (exporter scrapes as `bex`).
4. Delete the fixture; record the evidence here, then run t009.
