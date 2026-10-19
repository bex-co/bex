# CLI sweep 509 — `ea` and `workflows` read dialects

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `0952611b8`, 2026-10-19T09:10Z. Read-only; nothing created.

| probe | observed result |
| --- | --- |
| `ea sandboxes list -o json` | Exit `0`, `null` — the upstream rendering of an empty list (same shape sweep 458 recorded for `workflows list`); the endpoint itself is graded `[x]` and envelope-verified in the compat ledger. |
| `ea sandbox-groups list -o json` | Exit `1`, server `404 not found` — matches the ledger's recorded disposition exactly (graded gap 2026-09-15; "sandbox groups remain a separate non-goal", `docs/cli-compatibility-checklist.md:289-291`). Not a new finding. |
| `ea objects list` (no region) | Exit `1` client-side: `--region is required (or set RENDER_REGION environment variable)` — upstream string, client-side requirement already recorded by sweep 458. |
| `ea objects list --region oregon` | Exit `1`, server `404` — the ledger's `[-]` non-goal grade for all four `ea objects` verbs. |
| `workflows list -o json` / `-o text` | Exit `0`; `null` in JSON, an honest header-only table in text. Workflows are a recorded non-goal (DO_NOT_DO), and the empty-workspace reads neither error nor fabricate rows. |
| `workflows tasks list -o json` | Exit `1` client-side: a workflow version id is required in non-interactive mode — upstream arg validation before any request. |

## Safety and unrun checks

- Read-only sweep; no sandbox was created (create/exec/copy/stop are live-verified in the ledger and were not re-run), nothing mutated.
- Unrun: `ea sandboxes list` with a live sandbox present (envelope already live-verified 2026-10-02 per the ledger), `workflows versions`, `ea objects --local` client-side paths.
- Deploy lag means deployed revisions can precede local `main`.
