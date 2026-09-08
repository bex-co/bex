# w2/m92 · t007 precheck (no milestone code changes yet)

**Date:** 2026-09-08 · **Scope:** confirm m75 invariants still test-pinned before Phase 2/3.

## Commands

```bash
cd lego/operator
go test ./internal/migrate/... ./internal/identity/... ./internal/registry/... -count=1
```

## Result

All packages `ok`. Relied-upon behaviors still pinned, including:

| Invariant | Test |
| --- | --- |
| Digest verify failure aborts without tombstone | `TestVerifyMismatchAbortsWithoutTombstone` (`internal/migrate`) |
| Skip tombstone when sibling owns legacy | `TestSkipTombstoneWhenSiblingOwnsLegacy` |
| Dual-read off by default | `TestEnsureCredsForDualReadDisabledByDefault` (`internal/registry`) |
| Identity / purge tombstone helpers | `internal/identity` suite |

`cmd/registry-migrate` itself has no test files (thin CLI over `internal/migrate`).

No code changes in this milestone yet — full t007 acceptance waits on Phase 2/3 outcomes.
