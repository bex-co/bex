# CLI sweep 1021 — displayName is the mutable label; slug immutable (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `PATCH {displayName: "QA 1021 Display"}` → 200; the service's `name` reads back as the new label while `slug` stays `qa1021-web` (the immutable routing identity, consistent with the rename findings from earlier sweeps).
- `PATCH {displayName: ""}` → 200 and clears the label (explicit empty is a clear, distinct from omission — the pointer semantics documented in `rest.go`).
