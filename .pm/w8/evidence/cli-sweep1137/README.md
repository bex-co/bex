# CLI sweep 1137 — instances list mirrors service lifecycle (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A fresh free service lists exactly one instance (`<service-id>-<suffix>`) with a creation timestamp.
- `limit=0` on the instances list → 400 (same 1–100 window as sweep 1117).
- After service DELETE, the instances route → 404 with the parent: no orphaned instance records outlive the service.
