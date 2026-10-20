# CLI sweep 1214 — API key list exposes metadata only (live, 2026-10-20)

Read-only sweep; the account's pre-existing keys were listed, never touched, and no key material exists in this evidence. No new bug filed.

- `GET /api-keys` → 200 with metadata only per key (`id`, `name`, `createdAt`, `createdBy`, optional `lastUsedAt`) — no key value or prefix material in the list shape.
- The camelCase and short guesses (`/apiKeys`, `/keys`) → 404: the hyphenated path is the only route.
