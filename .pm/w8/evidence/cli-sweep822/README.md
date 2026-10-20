# CLI sweep 822 — API-key management read surface (bex extension) (live, 2026-10-20)

Read-only probes; no key created or revoked. No new bug filed.

- `GET /api-keys` → 200 listing the account's keys with `id`/`name`/`createdAt`/`lastUsedAt` only — no secret material in the list shape (key values are mint-time-only on this bex-ahead-of-Render surface, ADR018 § bex ahead of Render).
- `GET /api-keys/{id}` → 405: the surface is list/create/revoke-shaped; there is no single-key read to drift.
- The pinned CLI has no `api-keys` command (`unknown command`) — this capability is API/dashboard-only, so no CLI parity question arises.
