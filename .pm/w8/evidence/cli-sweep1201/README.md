# CLI sweep 1201 — `dockerCommand` lives in envSpecificDetails (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH `serviceDetails.envSpecificDetails.dockerCommand` → 200 and reads back verbatim in the same envelope — the nested home for run commands on image services (top-level placement refuses, sweep 1185).
