# CLI sweep 996 — service instances list (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `GET /services/{id}/instances` on a still-deploying service → one instance entry (`{service-id}-{suffix}`) with its creation time; unknown service → 404.
