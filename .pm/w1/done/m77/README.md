# w1 · m77 — Shared PROXY-protocol parser in lego/types

**Worker:** worker1 **Goal:** one implementation of the security-critical PROXY-protocol parser — the code that produces the *trusted client address* used for per-source admission and `ssh_sessions.remote_address` — instead of two byte-identical, never-synced copies split across the backend and operator modules. **Status:** done

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Create `lego/types/proxyproto` and move the parser + tests — **DONE** | 45m | —          |
| t002 | Retarget `backend/internal/proxyproto` to the shared package — **DONE** | 30m | t001       |
| t003 | Retarget `operator/internal/sniproxy` to the shared package — **DONE** | 30m | t001       |
| t004 | Simplify — **DONE** | 30m | t002, t003 |
| t005 | Test coverage — **DONE** | 45m | t002, t003 |
| t006 | Closeout — **DONE** | 15m | t005       |

## Definition of done

Exactly one PROXY v1/v2 parser body exists in the repo, in `lego/types/proxyproto`; the backend and operator packages are thin wrappers keeping their module-specific pieces (`Conn`/`Wrap` in backend; `TLSHandshakeType`/`ReadTLSRecord`/`ExtractSNI` in operator); the merged conformance test suite is the union of both prior suites (backend's 8 cases + operator's 5) and runs against the shared parser; no call site outside the two wrapper packages changed; all module test suites green.

## Source + Goal linkage

- **Source:** 2026-08-19 architectural refactor review §2.1 (ledger artifact: https://claude.ai/code/artifact/fe4af1ce-211f-4109-a541-f0aabd273c73). Evidence: `lego/backend/internal/proxyproto/proxyproto.go:39-214` and `lego/operator/internal/sniproxy/proxy.go:31-197` are identical except one comment; commit lineages have never synced (`9c111369` vs `240b000f`); the copy's stated justification ("no shared third module") is stale — `lego/types/netutil` already serves exactly this role for both modules.
- **Goal linkage:** tenant isolation and abuse-bounding (ADR022/ADR066 per-source admission depends on this parser's output); platform code health.
- **Expected outcome:** a future parser fix lands once instead of risking a spoofable-source bug in whichever copy it missed; ~345 prod+test lines deleted; conformance coverage strictly increases.
- **Why now:** best value-to-risk item in the review (pure code motion into an existing shared leaf module), and it de-risks the sniproxy consolidation queued behind it (`w1/059`). Render parity omitted: no REST/GraphQL/MCP/UI surface changes — internal code motion only.
