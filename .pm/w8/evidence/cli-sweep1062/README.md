# CLI sweep 1062 — leftover census across resource families (live, 2026-10-20)

Read-only census of `bex-canary`. No new bug filed.

- Across services, webhooks, projects, env groups, registry credentials, and Key Value instances, no `qa1xxx`-series fixture survives. The loop's create→exercise→delete→verify-absent discipline holds across every family it has touched in this series.
