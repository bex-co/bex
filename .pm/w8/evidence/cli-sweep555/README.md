# CLI sweep 555 — `buildFilter` cap, duplicates, whitespace, glob operators

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `d98d07f62`, production pin still `f4e38371055f`, 2026-10-19T12:06Z. Validate-only; no production resource created or mutated.

Closes sweep 554's unrun items, same hello-go base varied by `buildFilter.paths`:

- **Pattern cap exact:** 100 distinct patterns validates `true`; 101 fails with the named semantic error `buildFilter.paths has too many patterns (max 100)` — matching the `cleanGlobs` cap in `lego/backend/internal/apps/service.go`.
- **Duplicates accepted:** `[src/**, src/**, docs/**]` validates `true`. The validate plan output (`services`, `totalActions`) does not echo the filter, so whether duplicates persist or dedupe at write time is not observable read-only; no contract requires deduplication.
- **Whitespace entries:** a quoted `'   '` plus padded `'  docs/**  '` validates `true` — trim-then-drop/validate per the documented normalizer. The unquoted spelling parses as YAML `null` and is refused at the schema layer (`got null, want string`), the correct layer distinction.
- **Glob operator dialect complete:** `'*'` and `?`/`[a-z]` character-class patterns validate `true`. Unquoted `*` is a YAML alias error (`Blueprint is not valid YAML`), correctly surfaced as a parse failure rather than a glob refusal.

## Unrun

- Whether duplicate patterns dedupe on the stored spec (needs a create + readback flight; low risk either way).
- W8/129–131 live verification still waits for the production image pin to pass `5ae176831`.
