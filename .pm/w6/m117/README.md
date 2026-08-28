# w6 · m117 — The Recovery card reports "No backups yet" for databases with proven backups, because two failed Kubernetes reads degrade silently to "nothing"

**Worker:** worker6 **Goal:** the disaster-recovery surface distinguishes "there are no backups" from "I could not read the backups", and never renders a restore point it has no evidence for **Status:** in progress (t001+t002 done, landed in `94c0a185`; t003 half-done — the dashboard fallback shipped, the API-side refusal of an unsubstantiated restore has not)

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Resolve the fork: do the CNPG Backup objects still exist, or can bex-api not read them?    | 30m | —          | — **DONE**
| t002 | Stop reporting an unreadable cluster as an empty one — propagate both read failures        | 45m | t001       | — **DONE**
| t003 | Make the Recovery card honest, and stop offering a restore it cannot substantiate          | 45m | t002       | — partially done in `94c0a185` (dashboard "No backup yet" fallback landed); the API-side refusal of an unsubstantiated restore is still open — `Recover` gates only on `Status.BackupsEnabled`, never on `firstRecoverabilityPoint`
| t004 | Render parity                                                                              | 20m | t003       |
| t005 | Simplify                                                                                   | 20m | t004       |
| t006 | Test coverage                                                                              | 30m | t004       |
| t007 | Closeout                                                                                   | 10m | t005, t006 |

## Definition of done

- **The Recovery card never contradicts itself.** On a database whose backups are readable, "Earliest restore point", "Latest restore point" and the Backups list agree that backups exist. Today the card renders `Earliest restore point: No backup yet` and `Backups: No backups yet.` directly beside `Latest restore point: August 27, 2026 at 5:54 AM` (live capture below).
- **A failed read is visibly a failed read.** With the CNPG `Cluster` Get or the `Backup` List failing (RBAC removed, CRD absent, wrong namespace — pick one and simulate it), `recovery-info` reports an explicit unknown/error state, and the card says so. It must not be byte-identical to a database that genuinely has no backups, which is the current behavior and the reason this shipped unnoticed.
- **No restore point without evidence for it.** `latestRecoveryTime` is only present when the recovery window is actually established. It is currently `s.Now()` (`recovery.go:133`), the one field in the response that touches no Kubernetes read and therefore survives when everything else fails.
- **Restore is not offered when it cannot succeed.** `Restore to new instance` is enabled today whenever `enabled` is true; `Recover` (`recovery.go:199`) gates only on `!src.Status.BackupsEnabled`, so with backups enabled but unreadable it passes validation, passes `RequirePlanBilling`, and provisions a **new paid database** that cannot bootstrap. Verify the new behavior refuses before creating anything billable. (Not executed this run — running it would have created a paid resource on a production workspace.)
- `cd lego/backend && go test ./internal/postgres/...` covers the unreadable-cluster case explicitly, asserting it is distinguishable from the empty case. The existing tests cannot see this: `listBackups`' own doc calls the empty return "best-effort … an unavailable CNPG CRD (e.g. envtest) yields an empty list", which is exactly the production failure mode dressed as a test convenience.
- Re-run this milestone's probe against all three databases and confirm the reported state matches what the cluster actually holds, whichever way `t001` resolves.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 43rd run, 2026-08-27, journey 11 (Postgres). Workspace `tea-d98210cbbpdc73dcrkvg`. Read-only throughout — nothing was created, modified or deleted.

  All three pre-existing databases, probed in one pass (`POST /v1/postgres/{id}/recovery-info`, in-page authenticated `fetch`), `2026-08-27T12:53:33Z`:

  ```
  dpg-d9nqg95cavls73fp8m20  beancount-forum-db   basic-256mb  backupsEnabled:true  available
    -> {"enabled":true,"latestRecoveryTime":"2026-08-27T12:53:32Z","backups":[]}
  dpg-d9rrkoc4h4mc73edurp0  tianpan-forum-db     basic-256mb  backupsEnabled:true  available
    -> {"enabled":true,"latestRecoveryTime":"2026-08-27T12:53:32Z","backups":[]}
  dpg-d9rs3ee0ccis738kc7c0  blockeden-forum-db   basic-1gb    backupsEnabled:true  available
    -> {"enabled":true,"latestRecoveryTime":"2026-08-27T12:53:33Z","backups":[]}
  ```

  No `earliestRecoveryTime` key on any of them, `backups: []` on all three, and `latestRecoveryTime` tracking the wall clock exactly (an earlier call one minute before returned `12:52:41Z`). `GET .../export` returned `[]` for all three.

  What the dashboard renders from that, read from the live accessibility tree at `/databases/dpg-d9nqg95cavls73fp8m20#recovery`:

  ```
  Recovery — Point-in-time recovery and backups. Restore always creates a new database — this one is never modified.
    Earliest restore point:  No backup yet
    Latest restore point:    August 27, 2026 at 5:54 AM
    [Restore to new instance]   [Create export]
    Backups: No backups yet.
    Exports: No exports yet.
  ```

  Three of the four facts on the card say nothing is restorable; the fourth names a precise restore point, and the Restore button is enabled.

- **Why this is a defect and not a true report:** `.pm/w7/039.md` established, by execution rather than inspection, that these exact three databases had real backups six days earlier (2026-08-21):

  ```
  dpg-d9nqg95cavls73fp8m20-backup-20260821030000  completed
  dpg-d9rrkoc4h4mc73edurp0-backup-20260821030000  completed
  dpg-d9rs3ee0ccis738kc7c0-backup-20260821030000  completed
  ```

  and that a restore of `dpg-d9rrkoc4h4mc73edurp0` was performed and row-count-verified against the source (18,324 posts / 4,168 topics / 190 users, matching exactly). So the platform's own verified evidence says these databases have daily completed backups and a working restore path, while its own API tells the tenant they have none. That note is the evidence **for** this filing, not a duplicate of it: it fixed the *restore* path (`serverName` projection), and never touched the *reporting* path.

- **Root cause:** `lego/backend/internal/postgres/recovery.go` reports three fields from three sources, and only the source that cannot fail survives.
  - `:128` — `if err := s.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: name}, cluster); err == nil { … }`. A failed Get is dropped on the floor, so `EarliestRecoveryTime` is simply never set.
  - `:145-147` — `if err := s.Client.List(ctx, list, …); err != nil { return []BackupView{} }`. A failed List returns the same value as an empty one.
  - `:133` — `info.LatestRecoveryTime = s.Now().UTC().Format(time.RFC3339)`. Pure clock, no cluster read, always succeeds.

  One mechanism explains all three observed symptoms at once: if the Cluster Get and the Backup List are both failing, the response degrades to exactly `{enabled:true, latestRecoveryTime:<now>, backups:[]}` with no earliest — which is what all three databases return. The `enabled:true` comes from `d.Status.BackupsEnabled`, read off the Database CR, which also needs no CNPG access.
- **Consumer, checked:** `dashboard/src/features/databases/components/recovery-panel.tsx:95-105`. The earliest field already handles absence — `<LocalDateTime value={info.earliestRecoveryTime} fallback={t("databases.recoveryNoBackupYet")} />` — and that fallback is what renders "No backup yet". The latest field has **no** fallback and no condition (`value={<LocalDateTime value={info.latestRecoveryTime} />}`), because the backend never omits it. The panel authors anticipated the missing case on one field and could not on the other; the fix has to come from the backend for the UI to be able to express it.
- **Goal linkage:** [docs/ADR009-postgresql-management.md](../../docs/ADR009-postgresql-management.md) (managed Postgres backups + PITR) and [ADR006](../../docs/ADR006-bex-api.md). Also [ADR031]'s restore-drill lineage via `w7/039`'s follow-up — "a green backup that cannot restore is worse than a red one" has an exact mirror here: a red backup list that is actually green is worse than either, because it is the signal a tenant checks when deciding whether they are protected.
- **Expected outcome:** a tenant opening the Recovery card sees what the cluster actually holds, or an honest "could not read" — never a confident empty list standing in for a failed query.
- **Why now:** this is the disaster-recovery surface, and it is wrong in the direction that costs the most at the worst moment. A tenant reading "No backups yet" on a database that has nightly backups either wastes an incident re-establishing what they already had, or concludes bex's managed Postgres does not back up and moves the data elsewhere. `w6/m107` already flagged this same panel as "the highest-stakes instance" of a different defect, so it is a surface that has earned a second look.
- **Render parity:** included (t004). Render's Recovery tab shows a backup list and a PITR window; the field-level shape (`earliest`/`latest`/list) already mirrors it, and adding an explicit unknown state is a bex extension over Render's model — record it in `docs/ADR018-render-parity.md` rather than letting it drift in silently. REST, GraphQL and MCP all project `RecoveryInfoView`, so a shape change moves all three plus the dashboard.
- **Blast radius:** `listBackups` has exactly **1** caller (`RecoveryInfo`, `recovery.go:134`), and the Cluster Get is inline in the same function, so the change is contained — but `RecoveryInfoView` is a wire type on three surfaces, so the *shape* change is not. Enumerated across the resource-type family: only Postgres has a recovery/backup reporting surface at all — web · static · cron · worker · private have none, and Key Value has operator-side nightly RDB backups (`BEX_KV_BACKUP_*`, ADR021) with **no** API to list them, which is a separate gap and explicitly out of scope here rather than folded in.
- **Adjacent classes:** t002 introduces a new state next to "no backups", so place its neighbours: a database whose plan has backups disabled must keep today's `enabled:false` disabled-panel path (untouched); a caller who lacks permission on the Database itself must still get the existing authz error from `fetchDatabase` before any CNPG read is attempted, so the new unknown state can never become a probe for whether a database exists.
- **Unverified this run — and t001 exists to settle it:** whether the CNPG `Backup` objects still exist. The two candidate explanations are (a) bex-api cannot read them (RBAC on `postgresql.cnpg.io`, a namespace mismatch between the `Database` CR and the CNPG `Cluster`, or the CRD being unavailable to its client) and (b) they were genuinely pruned or the `ScheduledBackup` stopped after 2026-08-21. This hunt has no cluster access and could not distinguish them; the by-name-vs-by-id lookup hypothesis was raised and **ruled out live** (`POST /v1/postgres/beancount-forum-db/recovery-info` → `404 app not found` — databases are id-addressed only, so the caller-supplied identifier the Cluster Get uses is always the id). If (b) is the answer, that is an ops incident on top of this reporting defect, and it does not make the silent degradation correct — the API would still have been unable to tell the difference.
