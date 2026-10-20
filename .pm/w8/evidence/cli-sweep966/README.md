# CLI sweep 966 — project service-links replace semantics (live, 2026-10-20)

Two disposable projects and two disposable free services created across the two probes; all deleted (204s). No new bug filed.

- `PUT /projects/{id}/service-links {serviceIds:[sid]}` → 200; membership is read from the service side (`projectId` appears on the service GET — the project wire itself carries only `environmentIds`, matching `projects/rest.go`'s render shape).
- Replacing with an empty list unlinks (`projectId` disappears from the service).
- Linking an unknown service id → 200 with no change. The store applies replace semantics over existing rows and silently drops unknown ids; Render's pinned answer for unknown members is unverified, so this is recorded, not filed.
