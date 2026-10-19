# CLI sweep 589 — `workflows` surface: stub and refusal dialects match the recorded non-goal

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Read-only and refused probes only; no fixture created, nothing to clean up. Render Workflows are a deliberate bex non-goal ([checklist](/docs/cli-compatibility-checklist.md) `workflows` rows, DO_NOT_DO); this sweep re-verifies the recorded contract live.

- `workflows list -o json` → `null`, exit 0 (the `GET /v1/workflows` stub answering empty; sweep 458 recorded the same).
- `workflows versions list <synthetic wrk-…>`, `tasks list <synthetic wfv-…>`, `versions release <synthetic>` → server `404 not found`, exit 1. `tasks runs list` surfaces it as `unexpected response: 404 Not Found` — the pinned client's own wording.
- Client-side guards fire before any request: `tasks list`/`versions list` without an id in non-interactive mode → `workflow (version) id must be specified when output is not interactive`; `workflows start` without `--input` → `--input is required`; `workflows cancel` with a `job-…` id → `invalid task run ID …: must start with "trn-"`; `workflows create` without `--name` → `--name is required`; positional args to `create` → cobra `unknown command`.
- `workflows init qa589-proj` in an empty temp dir → `--language is required when using --confirm or non-interactive mode` (client-side scaffolding dialect; no bex call, temp dir removed).

Every observed behavior matches the ledger's `[-]` rows; no drift since the last grading.

## Unrun

- `workflows init --language …` scaffold contents (purely client-side; no server contract).
- `workflows dev` local server behavior (client-side, no bex call).
