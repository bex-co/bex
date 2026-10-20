# CLI sweep 876 — repo-backed rootDir/buildFilter acceptance (positive control for 875) (live, 2026-10-20)

Disposable repo-backed free web service created, patched, deleted (204, then GET 404). No new bug filed. Three refused create shapes along the way were strict-decoding working as designed (each named the misplaced field).

- Correct create shape: `rootDir` and `buildFilter` are **top-level** (inside `serviceDetails` they are unknown fields by name), native commands ride `serviceDetails.envSpecificDetails`, and a native runtime without them refuses by name (`native runtime go requires buildCommand and startCommand`).
- Created with `rootDir: examples/hello-go` + a paths/ignoredPaths filter → both read back verbatim; `PATCH {"rootDir":"examples"}` → 200 read back. The build-from-git fields that sweep 875 refused on an image service accept and persist here.
