# CLI sweep 511 — `services` list filters and output modes

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build at sweep start (`0952611b8`; tree has since advanced to `c603c8ec2` via concurrent work, no `lego/cli` journey impact), 2026-10-19T09:11Z. Read-only.

| probe | observed result |
| --- | --- |
| `services -o json` | 8 rows (concurrent hunts' churn accounts for movement from this loop's earlier baselines; none of it is this sweep's). |
| `--include-previews` | Same 8 rows — accepted and honestly a no-op (PR previews are a deliberate non-goal, so there is never a preview row to add). |
| `-e <synthetic evm-id>`, `-e bogus` | Exit `0`, `null` (empty). Correct filter semantics: `-e` filters the list, it does not look the environment up — contrast the `environments` lookup command's named 404 (sweep 458). An unknown environment simply contains no services. |
| `-e ''` | Exit `0`, full unfiltered list (empty value = unset). |
| `-o yaml` | Same rows/fields as JSON, YAML-shaped. |
| `-o bogusfmt` | Exit `1`, `invalid output format: bogusfmt` on stderr before any request. |

## Safety and unrun checks

- Read-only sweep; nothing mutated.
- Unrun: `-e` with a real environment that actually contains services (project/environment membership reads were sweep 489; the canary baseline's rows are ungrouped, so a positive `-e` match needs an owned grouped fixture — deferred to a later sweep rather than mutating shared projects), comma-separated multi-environment `-e a,b` with one bogus half.
- Deploy lag means deployed revisions can precede local `main`.
