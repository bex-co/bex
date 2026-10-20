# CLI sweep 901 — deploys do not outlive their service (live, 2026-10-20)

Disposable free web service created, deployed, deleted (204). No new bug filed.

- After the service delete, `GET /services/{id}/deploys/{dep}` → coded 404 and the top-level `GET /deploys/{dep}` → 404: deploy history is cascade-absent with its service (the m81 consistent-absence contract, same as events in sweep 887), so a leaked deploy id cannot resurrect metadata about a deleted workload.
