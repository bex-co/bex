# CLI sweep 1032 — configuration edits remain open while suspended (live, 2026-10-20)

One disposable free web service created, suspended, edited, resumed, and deleted (204). No new bug filed.

- While suspended, env-var PUT and rename PATCH both → 200 (only deploys conflict, sweep 1031). Suspension stops the workload, not the control plane — configuration can be prepared for the next resume.
