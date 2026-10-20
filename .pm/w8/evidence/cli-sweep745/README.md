# CLI sweep 745 — workspace-member mutation guards (live, 2026-10-19)

Synthetic targets only; the real member list was read back unchanged (1 member).

- `PATCH /owners/{canary}/members/usr-0000000000000000000000 {"role":"ADMIN"}` → 404 (target resolution precedes any role handling); the same with `"role":"bogus"` → 404, not a role-validation error.
- `DELETE` on the synthetic member → 404.

No bug filed.
