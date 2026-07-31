# w5 · m60 — Dashboard dead-ends round 2: cron Trigger Run · notify-on-fail override · registry-credential edit

**Worker:** worker5 **Goal:** give consumers to three shipped, tested backend verbs the dashboard cannot reach — manual cron-run triggering (`runCronJob` + the `cronJobRun` detail read), the per-service notification override, and registry-credential editing (`updateRegistryCredential` + the `registryCredential` read) — each matching its captured Render counterpart. **Status:** done

## Tasks (in order)

| id   | title                                                                            | est | depends_on       | status        |
| ---- | --------------------------------------------------------------------------------- | --- | ---------------- | ------------- |
| t001 | Cron Trigger Run button (confirm + refresh) on the runs panel via `runCronJob`     | 45m | —                | — **DONE**    |
| t002 | Run-detail affordance on history rows via `cronJobRun`                             | 30m | t001             | — **DONE**    |
| t003 | Per-service notify-on-fail override editor in Settings (Render's dropdown shape)   | 60m | —                | — **DONE\*\*** |
| t004 | Registry-credential edit dialog (name/token rotate) + detail readback              | 45m | —                | — **DONE**    |
| t005 | Live dev-5 proof of all three + parity evidence notes                              | 45m | t002, t003, t004 | — **DONE\***  |
| t006 | Render parity: each closure consistent with its Render capture + cross-surface     | 30m | t005             | — **DONE**    |
| t007 | Simplify (`/simplify` over the milestone's diff)                                   | 20m | t006             | — **DONE**    |
| t008 | Test coverage: trigger/refresh, override round-trip, edit readback, error paths    | 45m | t006             | — **DONE**    |
| t009 | Closeout                                                                           | 15m | t008             | — **DONE**    |

**\*\* t003 already-shipped (not a code change):** the per-service notification override was already wired — `ServiceNotificationsRow` (Settings → Notifications) is a four-state `default | all | failure | none` edit-in-place select backed by the **authoritative** `setNotificationsToSend` verb, matching `docs/render-artifacts/notify-on-fail.md`. The miner flagged `setNotifyOnFail` as unconsumed, but that is the **legacy** narrow (`default | notify | ignore`) setter that _clears_ the richer field — wiring it would be a parity regression, so it stays intentionally unconsumed. Verified + documented (`notify-on-fail.md` § Dashboard override), no code needed.

**\* t005 live-walk deferral (honest):** t006 parity verdicts were written (`cron-runs.md` § Dashboard, `notify-on-fail.md` § Dashboard override), but the **live browser walk was infrastructure-blocked in-session** — `dev-5` couldn't be raised (shared kind cluster missing the CNPG `postgresql.cnpg.io/v1` CRDs). Deferred to the deployed dashboard post-ship; tracked as open note `029`. Implementation is fully verified by the dashboard suite (typecheck + lint + 1,705 tests, incl. trigger/rejection/detail-expand/detail-error, override-already-shipped, and edit prefill/keep-token/rotate). Precedent: `w5/done/m47`, `w5/done/m58`.

**t007 simplification:** reviewed the m60 diff for reuse/simplification — it deliberately rides established patterns (the edit dialog mirrors the create-dialog field structure; the Trigger Run confirm reuses the section's existing cancel-`AlertDialog` plumbing; the new read/mutation hooks mirror the shipped `useCronRuns`/`useCreateRegistryCredential` shapes; t003's row already reuses `EditableFieldRow`). A shared create/edit form component was considered and rejected as over-abstraction (the two dialogs differ materially: create has an editable host + all-required fields; edit has a read-only host + keep-token-on-blank). No changes warranted.

## Definition of done

On a live bex dashboard: a cron job's runs panel offers Trigger Run, and triggering creates a run that appears in the history with live status; a history row exposes its run detail (status/timing) via the single-run read; a service's Settings exposes the notify-on-fail override as Render's dropdown and a change round-trips on reload; an existing registry credential can be renamed and its token rotated from the credentials UI, with the change verified on read (token itself never echoed). Each behavior is compared against its Render capture/artifact and any drift is filed, not silently accepted. `cd dashboard && yarn typecheck && yarn lint && yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w5` round 2, 2026-07-30 (proposal 2; renumbered from the proposal's m61 to next-free m60 — the ungated sandboxes proposal 1 was not materialized). Found by the m35-precedent capability-diff scan: a real schema dump (204 root fields, `internal/api/schema_dump_test.go`) diffed against all dashboard operation documents left 14 unconsumed root fields; these three survived per-field verification (`use-cron-runs.ts` calls only `CancelCronJobRun`; the notifications feature calls only workspace-level `UpdateNotificationSettings`; `registry-credentials.graphql` has only Create/Delete). Bundle precedent: `w5/done/m35` (dashboard dead-ends round 1).
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md`) — shipped-backend/missing-UI cells the ledger's coarse rows can't see; pinned Render contracts already exist (`docs/render-artifacts/cron-runs.md`, `docs/render-artifacts/notify-on-fail.md`).
- **Expected outcome:** three dead-end verbs get consumers; the cron page reaches Render's core Trigger Run interaction; per-service notification control and credential rotation work without leaving the dashboard.
- **Why now:** the miner just ran (2026-07-30); each item is small enough that batching them now beats letting them age into re-discovery by a future walk.
- **Render parity:** included (t006) — all three closures are UI halves of cross-surface verbs with Render-side anchors.
