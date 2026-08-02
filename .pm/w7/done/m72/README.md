# w7 · m72 — Public-surface availability & resource-exhaustion hardening

**Worker:** worker7 **Goal:** bound the work an unauthenticated or tenant caller can cause against the platform's public front doors so a modest flood cannot exhaust goroutines, memory, the kube-apiserver, or node disk. **Status:** done

## Tasks (in order)

| id   | title                                                                  | est  | depends_on |
| ---- | ---------------------------------------------------------------------- | ---- | ---------- |
| t001 | pg-sni-proxy: set the handshake read deadline (mirror kv-sni-proxy) — **DONE** | 20m  | —          |
| t002 | activator: host→App cache + bounded concurrency (no List/scan) — **DONE** | 1h   | —          |
| t003 | static-server: LimitReader + ContentLength cap before cache admission — **DONE** | 30m  | —          |
| t004 | Database.spec.exports: maxItems + per-Database active-export cap — **DONE** | 45m  | —          |
| t005 | activator: http.Server timeouts + MaxHeaderBytes — **DONE**           | 20m  | —          |
| t006 | build/publish: ephemeral-storage requests/limits + emptyDir sizeLimit — **DONE** | 30m  | —          |
| t007 | Simplify — **DONE**                                                   | 20m  | t001–t006  |
| t008 | Test coverage — **DONE**                                              | 1h   | t007       |
| t009 | Closeout — **DONE**                                                   | 10m  | t008       |

## Definition of done

Every cited public surface bounds attacker-amplifiable work: the pg proxy drops idle handshakes; the activator answers from a cache without a per-request cluster List and runs behind a real `http.Server` with timeouts; the static server never `io.ReadAll`s an unbounded tenant object; exports cannot fan out unbounded concurrent pg_dump Jobs; build/publish containers carry ephemeral-storage limits. Verified by targeted tests per surface.

## Source + Goal linkage

- **Source:** codex-security scan findings #2, #7, #10, #13, #21, #3 (all medium/low availability), validated against HEAD.
- **Goal linkage:** Security/Reliability pillar — protect shared availability and provider cost against unauthenticated and tenant-triggered exhaustion (ADR006 § Rate limits / availability; ADR034 build pipeline).
- **Expected outcome:** no unbounded goroutine / memory / job-fanout / node-disk path remains on these surfaces; cheap floods no longer amplify into kube-apiserver load or platform-component OOM.
- **Why now:** each is a concrete, independently shippable DoS/amplification path surfaced by the scan; bundling them as one availability-hardening milestone keeps the theme coherent.
- **Render parity omitted:** operator-internal mechanism + an internal export concurrency cap; no REST/GraphQL/MCP/UI surface change.

## Ship record — DONE 2026-08-01

Shipped as `b0642583` (deployed → GitOps pin `f633316c`). Six fixes: pg-sni-proxy sets a 10s handshake read deadline + clears it post-SNI (#2); the activator resolves hosts from a background-refreshed `hostCache` (O(1), no per-request List) and runs behind an `http.Server` with ReadHeader/Read/Write/Idle timeouts + MaxHeaderBytes (#7, #21); the static server bounds S3 reads (ContentLength pre-check + `io.LimitReader` in `drain`) and rejects oversize objects (`ErrObjectTooLarge` → 413) (#10); build/publish containers carry ephemeral-storage requests+limits and every tenant-controlled emptyDir has a SizeLimit (#3); Database exports cap concurrent pg_dump Jobs per Database (`maxConcurrentExportsPerDB=3`, deferring surplus) and bound the export work emptyDir (#13). CI: build, publish, staticserver, activator, pg-sni-proxy, and controller (incl. envtest) suites green; activator tests rewritten to drive the cache via `primedHostCache`.
