# CLI sweep 1179 — CLI identity and flag discipline (live, 2026-10-20)

Read-only CLI sweep. No new bug filed.

- `bex workspaces -o json` lists the same three member workspaces as the REST owners list (sweep 1147), and `bex whoami -o json` reports the QA user.
- A bare `bex workspace` prints its subcommand help and exits 0; an unknown flag (`bex services --nope-flag`) exits 1 with `Error: unknown flag` plus usage. The pinned CLI's own error idiom is intact.
