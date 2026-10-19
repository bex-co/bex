# CLI sweep 602 — `blueprints validate` dialects: schema truth, reference checks, workspace-aware quota

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Validate-only probes in a temp dir (removed); exit codes re-verified separately (valid → 0, invalid → 1) after a pipeline artifact in the first pass masked them.

- **Unsupported kinds are named, not crashed:** `type: workflow` returns `valid: false` with `Render Workflows (type: workflow) are not available on bex` at `services[0].type` (plus generic schema required-field errors for the same entry). `type: banana` → `value must be one of 'web', 'worker', 'pserv', 'cron', 'keyvalue', 'redis'`. A `postgres` entry under `services:` gets the same enum error — databases belong under `databases:`.
- **Strict schema:** unknown top-level key and unknown service field both fail with `additional properties '…' not allowed` at the exact path/line/column. Duplicate service names fail with `duplicate name "…" (first declared at #/services/0)`. An invalid service name fails with the shared resource-name wording at `services[0].name`.
- **References resolve within the file:** an env group declared in the same manifest plus `fromGroup` validates (`totalActions: 2`); a `fromService` pointing at an undeclared service fails with `fromService references unknown service "…" (declare it under services: or create it in this workspace first)`.
- **Datastore shapes:** a `keyvalue` service requires `ipAllowList` in the pinned schema (missing → `missing property 'ipAllowList'`; `ipAllowList: []` validates). A `disk:` block on a web service validates.
- **Validation is workspace-aware:** `databases: [{name, plan: free}]` failed with `workspace is limited to 1 Postgres database; delete an existing Postgres database to create another` — because the baseline free Postgres occupies the slot. Validate is not a pure syntax check; it prices the manifest against the live workspace.

## Unrun

- Validate of a manifest that would exceed other quotas (service count, Key Value slot — the KV slot was free during this sweep, so that arm is untested).
- `blueprints validate` against a file in a repo subdirectory with relative `rootDir` paths.
