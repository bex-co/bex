# CLI sweep 521 — blueprint `maxmemoryPolicy` spellings and `postgresMajorVersion`

**Verdict: no bug filed.** Validate-only companion to sweeps 519–520. Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:25Z. Nothing created.

| probe | observed result |
| --- | --- |
| `maxmemoryPolicy: allkeys-lru` / `noeviction` / `volatile-ttl` | `valid:true`. |
| `maxmemoryPolicy: bogus-policy` and the REST spelling `allkeys_lru` | `valid:false` at `/services/0/maxmemoryPolicy`, naming the hyphenated blueprint enum (`allkeys-lru`, `volatile-lru`, …). Hyphens are canonical in Render's pinned blueprint schema; underscores are the REST dialect — Render's own split, faithfully enforced per layer (compare sweep 520's `pro_plus` in exactly the opposite direction). |
| `postgresMajorVersion: 17` (number) | `valid:false` — `got number, want string`, precisely located. Render's blueprint schema types it as a string. |
| `postgresMajorVersion: "13"…"18"` | No version error (each run then stops at this workspace's current-state 1-Postgres limit, proving the version passed both layers). |
| `"12"`, `"11"` | `valid:false` by name: `PostgreSQL version "12" is not supported; choose one of 13\|14\|15\|16\|17\|18` — matches the capability registry's recorded below-13 boundary. |

## Safety and unrun checks

- Validation only; the workspace Postgres limit meant even a valid manifest could not have created anything in this workspace, and validate never applies.
- Unrun: an actually-applied old-major upgrade path (needs a disposable database; version *creation* correctness is covered by the postgres sweeps' long history), `diskSizeGB` bounds in blueprints.
- Deploy lag means deployed revisions can precede local `main`.
