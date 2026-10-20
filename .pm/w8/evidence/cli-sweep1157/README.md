# CLI sweep 1157 — blueprint create/read refusal forms (live, 2026-10-20)

Read-only plus refused creates only; the baseline blueprint was read, never mutated. No new bug filed.

- Create without a branch (with or without a repo) → 400 `repo and branch are required`: both fields gate the create before any repo access happens, so a bogus repo cannot be distinguished from this side of the gate.
- GET with a wrong-length or malformed blueprint id → 400 (the `blp-`/`exs-` + 20-char id pattern gate from sweep 967); the baseline blueprint reads 200.
