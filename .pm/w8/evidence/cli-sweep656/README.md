# CLI sweep 656 — empty-collection list shapes (live, 2026-10-19)

Read-only sweep; nothing created or mutated.

- `bex workflows list -o json` → `null` (text mode shows just the table header). `bex projects -o json` → `null`.
- Cross-checks: `GET /projects?ownerId=<workspace>` → 200 `[]` (without `ownerId`: 400 `invalid query parameter "ownerId"`), and `bex jobs list` on a real service (sweep 655) returned `[]`. So the CLI renders an empty collection as JSON `null` for workflows/projects while REST and other CLI lists use `[]` — a nil-slice marshaling wart in the CLI output layer.
- `bex workspaces -o json` listed the account's teams correctly; `bex environments <synthetic-project>` → 404 `project not found`.

Recorded as a minor output-shape inconsistency, not filed: consumers must null-check either way, and every sibling surface that returns data is correct. No bug filed.
