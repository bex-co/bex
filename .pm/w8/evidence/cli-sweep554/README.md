# CLI sweep 554 — blueprint `buildFilter` glob dialects (validate-only)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `d98d07f62`, production pin still `f4e38371055f`, 2026-10-19T12:03–12:05Z. No production resource was created or mutated.

Blueprint base was `examples/hello-go/render.yaml` minus env/health fields, with only `buildFilter` varied through `bex blueprints validate -o json`.

| Variant | Result |
| --- | --- |
| `paths: [src/**]`, `ignoredPaths: [docs/**]` | `valid: true`, exit `0` |
| `ignoredPaths` only | `valid: true` (schema any-of permits either list) |
| empty pattern (`''`) in `paths` or `ignoredPaths`; `paths: []` | `valid: true` — traced to the deliberate normalizer contract: `normalizeBuildFilter`/`cleanGlobs` in `lego/backend/internal/apps/service.go` trims and drops empty entries, and an all-empty filter canonicalizes to unset. Not a missed `store.ValidGlob` check. |
| absolute (`/src/**`), traversal (`../secret/**`), embedded traversal (`src/../secret/**`), backslash, malformed bracket (`[`) | `valid: false`, exit `1`, located semantic error naming `buildFilter.paths` and echoing the invalid pattern. |
| `buildFilter: {}` | `valid: false`, schema missing either `paths` or `ignoredPaths`. |
| extra field (`surprise: true`) | `valid: false`, additional properties not allowed. |
| wrong type (`paths: src/**`) | `valid: false`, `got string, want array`. |

## Unrun

- More than 100 patterns in one list (the documented cap in `cleanGlobs`) and duplicate-pattern canonicalization.
- Direct create/update with each malformed variant; blueprint errors already exercise the shared semantic message, and service create/update happy paths plus image-service refusal were covered in prior loop sweeps 458/471.
- W8/129–131 live verification remains queued until production's image pin passes `5ae176831`.
