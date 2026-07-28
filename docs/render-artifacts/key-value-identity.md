# Render Key Value — identity & rename contract (pinned)

Captured for w9/m6 (the KeyValue mirror of w9/m3's Postgres `dpg-` work). Sources: the pinned Render public OpenAPI (`api-docs.render.com/openapi/render-public-api-1.json`, `key-value` family) and the `render-oss/cli` generated client (`c398207`).

## Id shape

- A Key Value store's id is `red-<token>` — Render's Key Value (formerly Redis) resource prefix, the sibling of `srv-`/`dpg-`/`tea-`. Example ids in the OpenAPI `keyValue.id` schema use the `red-` prefix.
- The id is **opaque and immutable**: it is the primary key in every item route (`GET/PATCH/DELETE /v1/key-value/{id}`, `/v1/key-value/{id}/connection-info`, `.../suspend`, `.../resume`) and does not change when the store is renamed.
- bex mints it through `internal/id` (`id.KeyValue`, `red-<20-char xid>`), so bex ids are drop-in for a Render-shaped client. DNS-/k8s-safe (hyphen separator), per [ADR020-identifiers.md](../ADR020-identifiers.md).

## Mutable name

- `name` is the user-facing display name, mutable and **not** the id. In bex it is the required `KeyValue.spec.name` (a DNS-1123 label, ≤30 chars); the immutable `red-…` id lives in `metadata.name`. The temporary missing-name fallback was retired after the w1/m56 fleet gate reported zero legacy objects.
- `name` is unique **per workspace** (owner), not globally — two workspaces may reuse a name; a duplicate inside one workspace is rejected.

## PATCH (partial update) semantics

- `PATCH /v1/key-value/{id}` takes an all-pointer body: an omitted field means "leave unchanged", not "clear". The rename field is `name`; bex also accepts `plan` on the same route. (Render's `keyValuePATCHInput`; every field optional.)
- A name-only PATCH must not require `plan`. bex additionally supports `dryRun=true` (query or body) to validate + preview without writing — a bex extension consistent with its Postgres/service PATCH routes.

## CLI routing expectation

The unmodified official CLI resolves a typed argument (`render keyvalues get <name>` or `keyvalues update <name> --name <new>`) to a `red-` id by calling `GET /v1/key-value?name=<name>` and requiring exactly one match, then routes every item call (get / patch / delete / suspend / resume) by that opaque id. So:

1. The list `?name=` filter must match the **display name**.
2. The item routes must resolve by the opaque `red-` **id**, never by display name.
3. A rename changes only `spec.name`; the id (and every connection string, which embeds the id in the hostname) is unchanged, so a subsequent get-by-id keeps working and a get-by-new-name resolves back to the same id.

bex's implementation of this contract and its rollout/rollback order are in [ADR021 §6](../ADR021-keyvalue-management.md); the parity + CLI evidence rows are in [ADR018](../ADR018-render-parity.md) and [cli-compatibility-checklist.md](../cli-compatibility-checklist.md).
