# w4 · m330 — A Key Value maxmemory-policy change restarts the store, though Render documents no restart for it

**Worker:** worker4 **Goal:** a maxmemory-policy change applies to the running Key Value without a restart, as on Render: no connection drops, a Persistence Off store keeps every key, and no surface asks for or warns about a restart. **Status:** blocked — t001–t004 and t006–t008 done (shipped); t005 live replay waits for the next production deploy, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Operator: the template holds its policy, and the running process takes the change with `CONFIG SET` — **DONE** | 60m | — |
| [t002](done/t002.md) | Backend: descriptions, a restart-free write, no `key_value_config_restart` for a policy change — **DONE** | 25m | t001 |
| [t003](done/t003.md) | Dashboard: the Maxmemory Policy card saves without a dialog — **DONE** | 20m | t001 |
| [t004](done/t004.md) | Docs: ADR021, ADR018 and the webhook record — **DONE** | 10m | t001 |
| [t005](t005.md) | Live verification on an owned Free Key Value with Persistence Off | 20m | t002, t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 5m | t001 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t004 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t004 |
| [t009](t009.md) | Closeout | 10m | t005–t008 |

Nine tasks, 195m (3h15m).

## Source + Goal linkage

- **Source:** promoted from inbox note w4/569 (the w4/m316 follow-up, `/loopx w4` 2026-11-11) on 2026-11-11: the fix spans the operator, the CRD status, bex-api, the dashboard and a live replay.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); a Persistence Off store must never lose its keys to a change Render applies in place (w4/m233, w4/m316).
- **Supersedes:** w4/m316, which made every surface warn about the restart. With the restart gone its confirmation and wording are removed, and m316 closes on this milestone.

## Finding (from w4/569)

Why: bex replaces the store's only pod to change its eviction policy, so connections drop for up to a minute, and a Persistence Off store loses every key. Render documents a restart for persistence changes but not for the policy (render.com/docs/key-value, read 2026-11-11). w4/m316 made every surface warn about the restart. This item removes the restart itself, which closes the parity gap.

- **Source / severity / estimate:** follow-up named by w4/m316 (README "Target behavior"; t005 parity record), filed by the `/loopx w4` run 2026-11-11. **Minor–major**: the warning now exists everywhere, but the outage and the Off data loss remain. ~2h. Promote to a milestone if it spans the operator, the backend and a live replay.
- **Root cause:** `valkeyArgs` passes `--maxmemory-policy` as a container argument (`lego/operator/internal/controller/keyvalue_controller.go`, the `spec.MaxmemoryPolicy != ""` branch), so any change alters the StatefulSet pod template.
- **Fix (target behavior):**
  - The operator applies a policy change to the running server with `CONFIG SET maxmemory-policy <p>` as the platform user (`kvPlatformUser`). The tenant's `default` user cannot run `@admin` (w4/191).
  - The pod template stops changing on a policy edit. For example, the template keeps the policy it was created with, and the runtime value is reconciled on every pass, including after a restart for another reason.
  - `status` records the applied policy, so reads report what the server runs.
  - Once the restart is gone, remove w4/m316's restart confirmation from the Maxmemory card and the restart wording from the GraphQL and MCP descriptions.
- **Tests:** operator envtest: a policy change leaves the StatefulSet template hash unchanged and issues `CONFIG SET`. A restarted pod comes up with the latest policy.
- **DoD (live):** on an owned Free Key Value with Persistence Off, write a marker, then change the policy in the dashboard. No restart happens (`uptime_in_seconds` keeps growing), `CONFIG GET maxmemory-policy` reads the new value, and the marker survives.
- **Dedupe:** w4/m316 (warns about the restart; now in `blocked/` awaiting its live replay) and w4/191 (done: tenants may not `CONFIG SET`). No open item covers a runtime apply.

## Post-release probe (QA loop round 169, 2026-11-12): t005's replay passes

Pin `e7a56b07e0d7` (`c5518dc24`, 02:23 UTC) carries the fix.

- **Fixture:** Free Key Value `qa-20261112-r169-kv` (`red-dbqkjnmrbf4s73estuhg`), created in the dashboard with Persistence **Off** and Public access on. Deleted afterwards (DELETE 204, then GET 404).
- **Method:** Reveal connection info → "Copy External Key Value URL", used from the clipboard without printing it (`redis-cli --tls --sni red-dbqkjnmrbf4s73estuhg.kv.bex.co -u "$(pbpaste)"`). The clipboard was overwritten afterwards.
- **Before** (05:10:15):
  - `SET qa169marker v1` → `OK`;
  - `INFO server` → `run_id e73f3c43…`, `uptime_in_seconds 59`;
  - `CONFIG GET maxmemory-policy` → `allkeys-lru`.
- **Change:** Maxmemory Policy → Edit → `noeviction` → Save (05:10:36). No dialog opened, and the toast read "Updating maxmemory policy to noeviction…". The section's copy says "The new policy applies to the running store without a restart."
- **After** (05:11:28):
  - `GET qa169marker` → `v1`;
  - the same `run_id e73f3c43…`, `uptime_in_seconds 135` (no restart);
  - `CONFIG GET maxmemory-policy` → `noeviction`;
  - REST `options.maxmemoryPolicy` → `noeviction`, and `status` stayed `available`.
