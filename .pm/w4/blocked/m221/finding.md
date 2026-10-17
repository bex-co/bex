# Failed placement reads silently clear valid creation selections

Why: a lookup failure must not erase where the customer intends to create a resource.

**Severity:** major. Functional QA a856, 2026-10-17 UTC, researched through main `24e10c7f8`. Observed impact is unassigned creation; the owned service remained healthy. This ships the filing only.

## What was observed

Create an owned empty Project and Environment, then follow Project Overview's New Service link. It supplies both IDs, and the ready form selects the matching names. Healthy controls preserve them through all five App type previews at1440/390 and a fresh static-form load.

From a fresh web form, replace only the initial Environments operation's result with the recorded GraphQL error (`HTTP200`, `data:null`, errors). The real fetched response contains the valid Environment. Other results in its batch are returned unchanged. The form changes Environment to **No environment**, retains Project, and offers no inline placement error or Retry. The URL still contains both original IDs. This reproduces at390 and1440 and in the separate pre-create repetition.

Repeat with only the Projects operation's result replaced. Its real response contains the valid Project. Both selected fields change to No project/No environment, again without an inline placement explanation, at both widths. These are **five browser-local simulated read failures**. No production backend outage is claimed.

[Durable evidence](evidence.json) contains the exact scripts, original/injected target results, complete native mutation packets and independent reads. The inspected `.playwright-mcp/qa-a856-scope-error-desktop.png` supplements the committed receipts. It shows Project still selected, No environment and the existing accurate placement hint. A later successful list offers the intended Environment again but leaves No environment selected.

On the failed Environment-read form, enter the recorded own image-web configuration and choose Free. Deploy Service is enabled. One actual production create request **omits environmentId**; its complete response returns `projectId:null` and `environmentId:null`. An independent GraphQL read confirms those nulls and an empty Project service list. The service reaches Live and public200 with the exact14-byte marker.

The existing Manage resources flow then assigns only this service to the owned Environment. Its one mutation names only the owned service; an independent read has both intended IDs and the Project membership. The original Live deploy remains the only deploy. That is a passing assignment/recovery control, not a repair to the failed form.

## Located cause and target behavior

`dashboard/src/features/environments/components/project-environment-selector.tsx:32–34` reads only lists/loading from two hooks that also expose errors and refetch functions. At :43–50, `!projectsLoading` plus an absent list member clears Project and Environment. At :52–61, the corresponding Environment predicate clears Environment. Neither branch establishes that the list read succeeded.

`features/projects/hooks/use-projects.ts:64–69,85–98` maps absent project data to an empty list. `features/environments/hooks/use-environments.ts:128–145` does the same for environments and reports settled loading separately from error. Those hook contracts are useful elsewhere; this consumer must distinguish failed/unknown from an authoritative empty list.

Actual pinned Apollo **4.3.3** was opened from the official package capture. `core/QueryManager.js:641–676` emits result data, `loading:false` and a CombinedGraphQLErrors value under `errorPolicy:all`; :681 onward similarly represents transport failure with absent data. The lock and live request extensions both identify4.3.3. Installed4.1.3 was excluded. This is the application's error-blind invalidation rule, not a claimed Apollo defect.

The downstream consumer is confirmed: `features/services/lib/create-service-input.ts:196` sends `form.environmentId || undefined`, matching the actual missing field. Key Value at `routes/keyvalue.new.tsx:127` and Postgres at `features/databases/components/create-database-dialog.tsx:170` likewise serialize their nullable local selection as an optional ID. Their state and existing GraphQL String argument can retain the real ID; no schema or backend change is needed.

**Fix the selector locally:** retain selected IDs and other drafts through pending/error reads; show honest lookup status and a localized read-only Retry. Only a completed error-free result may clear a missing selection. Retry must restore labels from genuine data without another selection or a create. Preserve deliberate No project/No environment choices, Project-change clearing, and normal stale-ID handling. Do not restore URL defaults over a later explicit user choice. Keep the existing two-field responsive frame and unchanged global hooks/Select.

After recovery, the create request must carry the intended Environment ID. Backend validation and auto-joining its Project remain authoritative. No environment intentionally means outside Project; do not change that behavior or add a Project-only assignment to hide this defect.

## Shared scope and existing guarantees

There is **one selector component, two Select sites and three production mounts**: `routes/services.new.tsx:522`, `routes/keyvalue.new.tsx:266`, and `features/databases/components/create-database-dialog.tsx:368`. The service mount spans web/static/private/worker/cron; Postgres and Key Value use their own local state. The fault/actual-create journey was web only; their other error paths require the dedicated t002 audit.

Audit canonical `/services/new`, `/keyvalue/new` and the Postgres dialog (`/?new=database`, reached by `/new/database`), plus service-type Render create aliases, `/r/new`, `/new/redis` and `/d/new`. Verify alias suffixes; do not assume PG/KV accept the service form's URL preselection, since their normal scope starts in local state. Blueprint and Environment Group pickers do not mount this selector. Preserve authentication/permission handling and backend refusal contracts; no such failures were induced.

No matching owner or main fix was found across open/blocked/done text,88 actual open/blocked milestones plus2 evidence-index candidates, DO_NOT_DO, recent40 product commits and selector history. The last selector changes are the original environment/populated-project work.

- [w4/173](../../done/173.md) fixes the hint for deliberate No environment, not automatic clearing after a failed read. Its hint is accurate in this capture and stays unchanged. [w6/042](../../../w6/done/042.md) removes a nonexistent inline-create promise; it does not recover list failures.
- [w5/m41](../../../w5/done/m41/README.md)'s contextual-create guarantee passes on successful reads and fails in this uncovered error case. Its DoD is dispositioned: URL selection/contextual creation previews pass; actual single-service assignment recovery passes; populated search/counts, Runtime/Region/Updated facts, multirow Move and authenticated Render parity were not re-walked here. Project/environment CRUD has separate passing a854 evidence; no broad reversion or whole-milestone closure is asserted.
- m218's plan lookups, m219's pre-handler edits and231/290's post-write list publication have different producers. This happens after startup when a placement read fails.

[Render's Projects documentation](https://render.com/docs/projects) describes selecting the destination during resource creation. Its authenticated failed-read behavior was not tested. ADR008/ADR032/ADR018 link this bounded correction to usable hosting configuration.

**Unverified:** warm-cache, partial field-null and transport/timeout failures, pending explicit choices, stale IDs, non-web error adapters, locales and authenticated Render comparison. Initial blank labels were observed but are not a separate layout/latency finding. Require meaningful real-Apollo/client-state tests and native creation-request assertions rather than error-flag-only mocks.

## Cleanup

Native service deletion and Project deletion succeeded; the latter removed its now-empty Environment. Service/deploy/project/environment lookups all404, seven baseline ID sets match, exact App and serving/build/publish children are absent across namespaces, and public URL404. Logout confirmed, whoami401, cookies0, one blank page and owned session handles removed. An early Project confirmation capture timed out before any DeleteProject request; readback preceded the one successful actual delete. That observation gap is not another finding.
