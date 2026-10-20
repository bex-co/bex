# CLI sweep 1128 — leftover census: none of this loop's fixtures survive (live, 2026-10-20)

Read-only census. No new bug filed.

- Services, env-groups, and webhooks lists in `bex-canary` contain no `qa11xx-*` fixtures from sweeps 1100–1127: every fixture this loop created was deleted and verified absent in its own sweep.
- Older `qa-2026*` resources dated September–mid-October predate this sweep series and belong to other work; they were left untouched.
- `GET /projects` without `ownerId` → 400 naming the parameter (required-parameter discipline); with `ownerId` → 200 `[]`.
