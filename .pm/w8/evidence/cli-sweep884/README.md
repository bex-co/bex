# CLI sweep 884 — rename is a free-form display label, bounded only at create (live, 2026-10-20)

Disposable free web service created, renamed repeatedly, deleted (204, then GET 404). No new bug filed.

- PATCH `name` accepts spaces (`qa884 name with spaces`), an empty string (display label clears; `name` falls back to the immutable name), 31 characters, and a trailing hyphen — while `immutableName`/`slug` never move. This is the long-documented rename-is-a-label contract (sweeps 583/653/726): the strict coded `RESOURCE_NAME_INVALID` rule from sweep 857 binds the **identity** name at create, not the mutable display label afterwards.
