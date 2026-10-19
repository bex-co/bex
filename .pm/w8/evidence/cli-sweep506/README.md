# CLI sweep 506 — `logs --instance` filter semantics

**Verdict: no bug filed.** Covers the `--instance` leg left unrun by sweep 501. Same isolated session, `bex-canary`, HEAD build `37dffc290`, 2026-10-19T09:06–09:07Z.

One free image web service created → `live` → one planted HTTPS request (`200`) → `services instances` returned exactly one instance (`srv-…-<suffix>`) → log queries against that ID → deleted by ID (former-URL `404`).

| step | observed result |
| --- | --- |
| `logs --limit 20` (unfiltered) | 3 rows: 1 `request` row with no `instance` label, 2 `app` rows labelled with the listed instance ID. |
| `logs --instance <real instance id>` | Exactly the 2 app rows — the filter matches rows carrying that instance label, which is bex's documented label split (app lines carry instance/container; request lines are edge-attributed and carry method/statusCode instead). |
| `logs --instance <synthetic srv-…-deadbeef>`, `--instance bogus` | Exit `0`, honestly empty — a free-string label filter that matches nothing, same dialect class as sweep 501's `--status-code abc`. |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbatqvnbcrps73dmoafg` deleted by ID; former-URL 404 verified.
- Unrun: multi-instance discrimination (free plan runs one instance; a paid multi-instance fixture is out of hunt rules), `--task-id`/`--task-run-id` (workflows are a deliberate non-goal), instance filter combined with `--tail` (live tail refuses store-only filters by name, covered in the compat ledger).
- Deploy lag means deployed revisions can precede local `main`.
