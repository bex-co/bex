# w1 · m80 — Operator dedup pass

**Worker:** worker1 **Goal:** land the operator's four high-confidence consolidations from the 2026-08-19 refactor review — headline: the host-bound git credential helper (the shell string confining `GIT_AUTH_TOKEN` to github.com) exists as two verbatim copies with its SECURITY comment on only one, plus the build/publish/predeploy Job state machine and two smaller literal clones. **Status:** done

## Tasks (in order)

| id   | title                                                            | est | depends_on             |
| ---- | ---------------------------------------------------------------- | --- | ---------------------- |
| t001 | Hoist the git credential helper to one guarded constant — **DONE** | 45m | —                      |
| t002 | `execution.EnsureOwnedJob` for build/publish/predeploy — **DONE** | 1h  | —                      |
| t003 | `deleteStaleChildren` call sites + shared `currentRevisionPodsReady` — **DONE** | 30m | —                      |
| t004 | Database/KeyValue narrow carve-outs + `ptr` consolidation — **DONE** | 45m | —                      |
| t005 | Simplify — **DONE** | 30m | t001, t002, t003, t004 |
| t006 | Test coverage — **DONE** | 1h  | t001, t002, t003, t004 |
| t007 | Closeout — **DONE** | 15m | t006                   |

## Definition of done

The credential-helper expression exists once, carries the SECURITY comment, and a test asserts both `build.BuildJob` and `publish.PublishJob` clone containers embed it (divergence fails CI); the three Ensure functions share one create→adopt→classify helper while keeping their own outcome enums and error prefixes; the two inline `deleteStaleChildren` copies and the `podsReady` twins are gone; `growOnlyIntent`/`secretClient`/`setNotReadyCondition` are shared; emitted Kubernetes object bytes are unchanged throughout (`make test` green, no golden-manifest churn).

## Source + Goal linkage

- **Source:** 2026-08-19 architectural refactor review §2.6, §3-operator (ledger artifact: https://claude.ai/code/artifact/fe4af1ce-211f-4109-a541-f0aabd273c73). Evidence: `build/build.go:919` ↔ `publish/publish.go:400` byte-identical helper, SECURITY comment only at `build.go:904-912`; `build.go:444-497` ↔ `publish.go:203-231` ↔ `predeploy.go:192-224` same state machine; `app_controller.go:2155/:2283` inline copies of `:1829`'s helper; `app_controller.go:2027` ↔ `keyvalue_controller.go:882` same 22 lines.
- **Goal linkage:** build-plane security custody (the helper is the only thing keeping the tenant GitHub token off non-GitHub hosts on the publish plane) + operator code health ahead of the app_controller file split (`w1/060`).
- **Expected outcome:** ~−170 lines; the token-confinement invariant becomes CI-enforced; the drift between the three purge-Job builders stays a separate visible decision (`w1/061`).
- **Why now:** t001 is the review's only duplication with a security consequence if left; the rest is low-risk and clears the ground for the file split. Render parity omitted: operator-internal mechanism, no REST/GraphQL/MCP/UI surface, emitted object bytes unchanged.
