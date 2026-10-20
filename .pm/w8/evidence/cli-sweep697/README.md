# CLI sweep 697 — previews/maintenance subresources (live, 2026-10-19)

Read-only against the baseline `hello-go`.

- `GET /services/{id}/previews` → 404: PR previews are a recorded unsupported capability (ADR049 fail-closed set), not a missing route.
- `GET /services/{id}/maintenance` → 404: Render's platform maintenance-runs surface is not implemented; maintenance **mode** is the bex surface (sweep 663).

Board note: w8/135 (sweep 676) was fixed on main by a parallel worker in `410b9ae7b` while this loop ran; it joins 132–134 in awaiting a production pin past its fix for live verification.

No bug filed.
