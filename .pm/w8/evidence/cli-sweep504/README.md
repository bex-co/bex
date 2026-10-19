# CLI sweep 504 — cross-type and synthetic-ID selector dialects (read-only)

**Verdict: no bug filed.** Same isolated session as sweeps 501–503, `bex-canary`, HEAD build `37dffc290`, 2026-10-19T09:03Z. No fixture created; baseline resources were only ever named as read selectors and never mutated.

Every wrong-type or synthetic selector refused with exit `1` and a named not-found/400 — no `500`, no cross-type data, no existence leak beyond the caller's own workspace:

| probe | observed result |
| --- | --- |
| `deploys list <dpg-id>`, `<synthetic srv-id>`, `not-an-id` | `404 Not Found` for all three shapes. |
| `services instances <dpg-id>`, `<synthetic red-id>` | `404 Not Found`. |
| `keyvalues get <srv-id>`, `postgres get <srv-id>` | Resolver's named not-found in the active workspace. Its `` `render workspace set` `` hint is the documented upstream runtime-copy residual (inventory in `docs/bex-cli.md:162`; already recorded by sweep 468) — the branding overlay rewrites help/usage text, not upstream `RunE` error strings, and the pinned resolver builds this string (`pkg/keyvalue/resolve.go:187`, `pkg/postgres/resolve.go:186`). Not a new bug without a changed branding requirement. |
| `keyvalues get <synthetic red-id>` | Same named not-found (the resolver falls back to name lookup and reports "named …" — upstream dialect, identical against Render). |
| `logs -r <dpg-id> --type request` | Precise `400`: managed Postgres logs support only text/time/direction/level/limit/instance filters. |
| `logs -r <synthetic srv-id>` | Client-side `no resource found with ID or name …`, exit 1, before any log query. |

## Safety and unrun checks

- Read-only sweep: nothing created, updated, or deleted; pre-existing resources untouched.
- Unrun: name-collision selectors (same name on two kinds — covered by sweep 479's per-kind namespaces), selectors for resources in another workspace the QA user can access (never probed across tenants per the hunt rules; cross-workspace context guard covered by sweep 479).
- Deploy lag means deployed revisions can precede local `main`.
