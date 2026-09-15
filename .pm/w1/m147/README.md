# w1 · m147 — A service's bulk environment save skips the secret-map quota every sibling write path enforces

**Worker:** worker1 **Goal:** every write to a service's env-var or secret-file map (per-key, bulk replace, blueprint seed, and the bulk **patch** the dashboard's Environment editor uses) is bounded by the same aggregate quota. That quota is 500 entries and 512 KiB per map, and security review round 11 set it (ADR066 #6) so a map can always fit the Kubernetes Secret it is projected into. A guard stops the next write path from skipping it. **Status:** in progress. t002 and t004–t007 are done. t001 and t003 are implemented and green, and they close after the live DoD on the deployed build.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Enforce `envMapWithinQuota`/`filesMapWithinQuota` inside every `PatchEnvironment` mutate, before any write | 45m | — |
| t002 | Guard test: enumerate every service secret-map write path and fail when one reaches the store without the quota check — **DONE** | 40m | t001 |
| t003 | Dashboard: validate secret files against the server's aggregate contract (not 1 MiB per file) and show the server's refusal on save | 40m | t001 |
| t004 | Read-only sweep: find service env/file maps already over quota because of this gap, and record the remediation decision — **DONE** | 30m | t001 |
| t005 | Render parity — **DONE** | 30m | t001, t002, t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

Each bullet can be repeated from a signed-in page on production (or `dev-1`) against a throwaway free web service. The first failed at filing time and the second passed:

- **An over-quota bulk save is refused, with the server's reason.** Environment → **Edit** → **Upload files** with a 614,400-byte file (a `File` built in the page and attached to the editor's file input) → **Save and deploy**. Required: `PatchServiceEnvironment` returns an error naming the quota (`total secret file size limit of 524288 bytes exceeded`); the UI shows that sentence, not a success toast; `service.secretFile(name)` does not exist afterwards; and no new `config_change` deploy row opens. At filing time the save returned `rolledOut: true`, showed "Environment saved and deployment started", stored the file at **614,400 bytes**, and deployed it.
- **An in-quota bulk save still works (control).** A small secret file saved the same way returns `rolledOut: true` and opens a `config_change` deploy that reaches `live`. This held at filing time.
- **No write path can bypass the quota again.** The t002 guard test fails when any new function writes a service env or file map to the secret store without passing the result through the quota check.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: free web service `qa-20260914-sf` (`srv-dajtb80gsm7s73f649s0`, `examples/hello-go`, docker), created 11:08:20Z and deleted inside the run. After deletion its URL returns `404`.

1. **Control, in quota.** A tiny secret file (named `qa-pass7`, empty contents; the name came from a mis-aimed probe, but the save itself is a valid control) was saved through the Environment editor at ~11:11. It opened `dep-dajtcn26m8ac739r5p50` (`trigger: config_change`), which reached `live`.
2. **Over quota, accepted.** In Edit mode, a 614,400-byte `big.bin` was attached to the editor's upload input. The draft showed `0 variable operations · 1 file operation` and **Save and deploy** was enabled:

   ```text
   PatchServiceEnvironment → 200
   {"data":{"patchServiceEnvironment":{"envVarKeys":[],"rolledOut":true,"secretFileNames":["big.bin","qa-pass7"]}}}
   toast: "Environment saved and deployment started"
   ```

   The deploy it opened, `dep-dajtefi6m8ac739r5pc0` (`config_change`), went `build_in_progress` 11:15:37 → `live` 11:16:49. Afterwards `service.secretFile(name:"big.bin").content` measured **614,400 UTF-8 bytes** (over 512 KiB), and `secretFileNames` listed `["big.bin","qa-pass7"]`.

3. **The guarded control was confirmed live, 2026-09-14 pass 8.** I sent the same 614,400-byte `big.bin` through the **same shared editor** (`EnvironmentEditor`) on an environment group, `qa-20260914-grp` (`evg-dajtooq6m8ac739r5q60`, created and deleted in the run). The env-group bulk patch refused it:

   ```text
   PatchEnvGroupEnvironment → 200
   {"data":{"patchEnvGroupEnvironment":null},"errors":[{"message":"bad request: total secret file size limit of 524288 bytes exceeded","path":["patchEnvGroupEnvironment"]}]}
   group after: revision "egr1_AAAAAAAAAAE" (unchanged), secretFiles []
   toast: "Couldn't save the environment. Your draft is still here."
   ```

   Same editor, same payload, opposite outcomes: the env-group path (`envgroups/patch.go:186-189`) enforces the quota, and the service path (item 2) does not. The toast also confirms t003's copy problem: the server's sentence is discarded (`service-environment-editor.tsx:497-502`; the class belongs to `w1/m145`). In the same pass, linking that group to a free service returned `linkEnvGroup: true`, showed "Linked services are redeploying to apply the change.", and opened a `config_change` deploy (`dep-dajtp9ogsm7s73f64avg`), so linking works as promised.

No screenshots were taken; the transcripts above are the evidence.

## Root cause

- **The quota and its stated scope.** `lego/backend/internal/secrets/service.go:521-546`: `maxEnvKeys = 500`, `maxSecretFiles = 500`, `maxSecretMapBytes = 512 KiB`, enforced by `envMapWithinQuota` / `filesMapWithinQuota`. ADR066 finding 6 states the contract: quotas "enforced inside the CAS mutate (re-checked on every retry), on batch replacement, and on blueprint seeding, before any write". The reason given is that "a map past Kubernetes' 1 MiB Secret ceiling wrote to OpenBao first and then failed at the projection (source/projection divergence)".
- **The bulk patch never calls either check.** `PatchEnvironment` (`lego/backend/internal/secrets/batch.go:80`) → `patchEnvironmentSparse` (`:191-243`) mutates the env map (`:196-204`) and the files map (`:212-219`) through `updateMapCAS` with only `core.ApplyEnvVarPatch` / `core.ApplySecretFilePatch` (`:497-503`). It then projects via `finalizeEnvironmentPatch` → `projectFiles` (`:246-261`). `patchEnvironmentCAS` (`:111-177`) applies the env patch at `:144` with no quota check either. `grep -n "WithinQuota\|ValidateFilesMapQuota\|ValidateEnvMapQuota" lego/backend/internal/secrets/batch.go` finds **nothing**.
- **When it happened.** The round-11 fix `cacde0622` (2026-08-17) changed `secrets/files.go`, `service.go`, `store.go` and `secrets_test.go`, but **not `batch.go`**. `batch.go` already existed (`87a058334`, 2026-07-17, "staged service configuration workflow"). This is a missed alias, not a regression.

## Blast radius

- **Unguarded, three aliases of one verb:** REST `lego/backend/internal/secrets/rest.go:77`, GraphQL `patchServiceEnvironment` (`secrets/graphql.go:218`, the dashboard Environment editor's save), and MCP (`secrets/mcp.go:113`). Both halves are exposed: env vars (500 keys / 512 KiB) and secret files (500 files / 512 KiB).
- **Guarded today (the controls, which need regression tests too):**
  - per-key `SetEnvVar` / `SetSecretFile` (`service.go:372`, `files.go:141`);
  - bulk replace `service.go:321`, `files.go:185` / `:221` (`prepareSecretFiles`);
  - blueprint seeding `service.go:461`;
  - the **env-group** bulk patch, the exact sibling of this verb (`envgroups/patch.go:186-189`, and `envgroups/service.go:604-607`).
- **Client side:** the Environment editor caps a single file at **1 MiB** (`MAX_SECRET_FILE_BYTES`, `dashboard/src/features/services/lib/environment-draft.ts:3`, checked on upload at `service-environment-editor.tsx:434`) and has no aggregate check (`validateEnvironmentDraft`, `environment-draft.ts:142-158`, validates names and contents only). Its save error branch is a bare `catch` with the generic `services.environmentSaveError` (`service-environment-editor.tsx:497-502`); `w1/m145`'s blast radius already lists that site.

## Adjacent classes

- **Staged vs deploy save modes:** the quota must hold for `saveMode` staged (`save_only`) as well as deploy. A staged write still stores the map.
- **Revision conflicts** (`envRevisionConflict`) and **compensation** (`compensateEnvironment`, `restoreSourceMaps`) must not be confused with a quota refusal. A quota refusal happens before any write, so there is nothing to compensate.
- **Render's contract differs.** Render documents "The combined size of all secret files uploaded to any given service or environment group cannot exceed 1 MB" (`render.com/docs/configure-environment-variables`, fetched 2026-09-14). bex's 512 KiB is a deliberate tighter cap (the base64 projection must fit a 1 MiB Secret). Record it as a divergence; do not raise the server cap to match.

## Unverified (reasoned, not probed this run)

- **Pushing a map past the 1 MiB Secret ceiling** through this path, and so the source/projection divergence ADR066 describes. It was deliberately not attempted on production, because a failed projection could leave residue that blocks deleting the fixture. It is reasoned from ADR066 #6 and the missing check.
- **REST (`rest.go:77`) and MCP (`mcp.go:113`)** were not driven live; the claim rests on the shared verb. The **env var** half (500 keys / 512 KiB) was not probed either, only secret files.
- **Whether any existing production service already holds an over-quota map** (t004).

## Dedupe

- `docs/ADR066-security-review-round11.md` finding 6 is the fix this completes. Its enumerated paths do not include the sparse bulk patch.
- `.pm/w2/done/m80/README.md:48` notes "Env-var/secret-file aggregate quotas (ADR066 #6) unchanged": out of scope there, not a record of this gap.
- No later security round (ADR067–ADR083 were grepped) or board item names `PatchEnvironment` together with the quota.
- `.pm/DO_NOT_DO.md`: no conflict.
- Not fixed on `main` as of `9dfdf904f`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 7, journey 4 (the secret-file half of env vars / secret files). The fixture, probes and deletion are above.
- **Goal linkage:** `GOAL.md` #7 (security review lineage ADR028 → … → ADR066) and pillar 1 (Render-compatible environment management across REST/GraphQL/MCP/UI).
- **Expected outcome:** the round-11 aggregate quota holds on every write path, including the one the dashboard actually uses. A guard stops the next write path from skipping it, and the editor tells the user the real limit before and after saving.
- **Why now:** the unguarded path is the dashboard's default save for service environments, so this is the common case, not a corner. The fix is local (two mutate closures plus one validator reuse), and every sibling already shows the pattern.
- **Render parity:** included. The fix changes refusal behavior on REST/GraphQL/MCP and the dashboard editor's limit and copy, and t005 records the 512 KiB vs Render 1 MB divergence.
