# CLI sweep 1050 — timestamp monotonicity (string compare is the trap) (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- First pass flagged `createdAt > updatedAt` — a lexical artifact: `createdAt` is second-truncated (`…13Z`) while `updatedAt` carries nanoseconds (`…13.440Z`), and `.` sorts before `Z`. Parsed as instants, `createdAt <= updatedAt` holds, `createdAt` is stable across a PATCH, and `updatedAt` advances. Same conclusion as sweep 941, re-verified on the new pin.
