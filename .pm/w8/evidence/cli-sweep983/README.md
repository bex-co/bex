# CLI sweep 983 — service IP allow-list entries (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- The entry wire shape is Render's `{cidrBlock, description}` under `serviceDetails.ipAllowList` (verified against `core/cidr.go`'s pinned-client comment). A first probe with a `cidr` field got a precise 400 naming the unknown field — harness error, corrected.
- With the correct shape: PATCH → 200 and the entry reads back verbatim; a bogus CIDR (`999.999.0.0/16`) → 400; the top-level (non-`serviceDetails`) placement → 400.
- Allow-list editing is accepted on a free web service (no paid gate hit on this field).
