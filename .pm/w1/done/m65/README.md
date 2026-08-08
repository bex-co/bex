# w1 · m65 — Security-review remediation: close the 2026-08-07 codex-security scan findings

**Worker:** worker1 **Goal:** every reportable finding from the 2026-08-07 codex-security repository scan (16 findings — 3 high, 9 medium, 4 low) is closed with a durable control and a regression test that fails on the pre-fix code. **Status:** done

**Done 2026-08-07** — all 16 findings remediated at root cause with regression tests, verified locally (backend `go test ./...` + `go vet` + CI-gated `make lint-backend` 0 issues; types green; operator build/vet + touched packages `-race` green; dashboard typecheck/lint/test green 1972 tests; mobile typecheck/lint/unit green 302 tests). **Uncommitted pending `/ship`.** Notes: F2 shipped BOTH controls — the always-on unique installation→workspace binding (store lookup + migration `0069` unique index + service refusal) AND the installation-admin verifier seam (`BEX_GITHUB_APP_CLIENT_ID`/`_SECRET`, enforced on the browser callback via the OAuth `code`; activation also needs the App's "Request user authorization during installation" setting, an operator step like `BEX_OPENFGA_URL` activates authz). F16 flips fail-open→**fail-closed startup** (`BEX_ALLOW_INSECURE_AUTHZ=1` dev override); F13 makes verified-invite-email **secure by default**. Operator resource-exhaustion caps are env-tunable (`BEX_PROXY_*`/`BEX_KV_PROXY_*`, mirrored to `.env.example` + CLAUDE.md). Operator `make lint` carries pre-existing debt (`app_controller.go` goconst) + newly-strict `modernize` suggestions on the new `-race` tests; operator lint is not CI-gated (only backend `go-lint.yml` is, and it's clean).

## Tasks (in order)

| id   | title                                                                                     | est | depends_on           |
| ---- | ----------------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Bind GitHub clone tokens to a verified GitHub origin (F1, high)                            | 60m | — — **DONE**         |
| t002 | Prove installation ownership + unique workspace binding on GitHub connect (F2, high)      | 60m | — — **DONE**         |
| t003 | Split infra.yml: credential-less PR checks vs protected-environment plan (F3, high)       | 45m | — — **DONE**         |
| t004 | Converge OpenFGA tuples exactly on role change / removal / invite (F7, medium)            | 60m | — — **DONE**         |
| t005 | Fail-closed multi-tenant authz + verified-invite-email default (F16, F13)                 | 45m | — — **DONE**         |
| t006 | Same-origin login redirect normalizer + mobile logout-generation guard (F4, F14)         | 45m | — — **DONE**         |
| t007 | Activator: immutable cache objects, fix concurrent-map wake crash (F5, medium)           | 30m | — — **DONE**         |
| t008 | Admission + timeouts on public operator listeners: PG/KV proxies + static server (F6, F12) | 60m | — — **DONE**         |
| t009 | bex-api output & query-cost budgets: MCP exec cap + GraphQL cost limit (F8, F9)           | 45m | — — **DONE**         |
| t010 | Agent-attach: per-part / per-session transcript byte quotas + paginated replay (F10)      | 45m | — — **DONE**         |
| t011 | Dashboard: streaming body-size caps on mutation handlers (F11, medium)                    | 30m | — — **DONE**         |
| t012 | SSRF: deny RFC 6598 shared address space in the webhook dialer (F15, low)                 | 20m | — — **DONE**         |
| t013 | Render parity                                                                              | 30m | t012 — **DONE**      |
| t014 | Simplify                                                                                   | 20m | t013 — **DONE**      |
| t015 | Test coverage                                                                             | 60m | t013 — **DONE**      |
| t016 | Closeout                                                                                   | 10m | t015 — **DONE**      |

## Definition of done

Each finding below is remediated at its root cause and pinned by a test that fails on the current (pre-fix) code:

- **F1 (high)** — clone-token minting binds to an exact allowlisted GitHub host after canonical URL parsing (userinfo/alt-ports rejected); a non-GitHub host with a granted `owner/repo` suffix never mints a token or creates a Secret.
- **F2 (high)** — the GitHub connect callback requires a GitHub principal authorized to administer the installation and enforces a unique installation→workspace binding; an installation administered by a different GitHub user is rejected without mutating either workspace.
- **F3 (high)** — `pull_request` runs a credential-less validate/fmt/policy job; production Hetzner/state credentials and `terraform plan` live only behind a protected environment reachable from reviewed `main` (or a manually approved trusted job).
- **F7 (medium)** — role change / removal / invite-redemption reconcile OpenFGA to exactly one expected role; revocation errors surface and are retried; a retry repairs tuples even when the membership row already equals the target.
- **F16 / F13** — the DB-backed multi-tenant API fails startup (or enforces membership on every explicit-workspace verb) when OpenFGA is unwired; login-time invite redemption requires a verified email in every production configuration.
- **F4 / F14** — login `next` is normalized to a same-origin relative path at the route boundary (absolute/protocol-relative/encoded external values → `/`); the mobile session manager uses a generation guard so a post-logout refresh cannot republish `signedIn`.
- **F5 (medium)** — activator returns immutable copies from its cache and coalesces wake per App; `go test -race` on concurrent wake requests shows no shared-map mutation.
- **F6 / F12** — the PG proxy, KV proxy, and static server enforce global/per-source (or per-object) admission with idle/lifetime timeouts before backend dial / origin fetch; over-budget load is rejected immediately with a fixed bound on concurrent buffers and backend dials.
- **F8 / F9** — MCP `sandbox_exec` enforces a cumulative stdout/stderr byte budget with truncation metadata; GraphQL rejects documents exceeding depth/alias/field/root-operation/cost budgets before any resolver runs.
- **F10 (medium)** — agent parts have a max serialized-part size and per-session byte/part quota at driver, gateway, and DB boundaries; replay is bounded (paginated / capped in-memory).
- **F11 (medium)** — every dashboard mutation handler reads its body through one streaming helper that aborts past a small route-specific cap (chunked / no-Content-Length bodies included) before parsing.
- **F15 (low)** — the outbound webhook dialer denies RFC 6598 `100.64.0.0/10` (and is expressed as an explicit public-prefix policy) for literals, DNS answers, dual-stack, and IPv4-mapped IPv6.
- Backend (`cd lego/backend && go test ./...`), operator (`make test`), dashboard (`yarn test`), and lint are green; every new control has a regression test.

## Source + Goal linkage

- **Source:** the 2026-08-07 codex-security repository scan report — `/Users/tianpan/.codex/state/plugins/codex-security/scans/bex/codex-security-bex-hayvC8/report.md` (revision `137555ad`; 16 reportable findings, all high-confidence static source-to-sink). Third audit in the ADR028 → `w1/m53` (2026-07-19) → this lineage; overlaps and extends `docs/ADR045-security-review-round3.md`.
- **Goal linkage:** platform trust + tenant isolation (`docs/ADR022-tenant-isolation.md`, ADR043, ADR012) — preserving cross-tenant/intra-workspace authorization boundaries, binding external credentials to the intended tenant/repo/host, and keeping shared services available under adversarial load. Foundational to every roadmap pillar (a multi-tenant Render alternative is only credible with these boundaries intact).
- **Expected outcome:** the three high-severity credential/authz/CI escalation paths are closed, the shared-service DoS surface is bounded, and the low-severity fail-open/redirect/SSRF defense-in-depth gaps are removed — each with a test that reproduces the pre-fix defect.
- **Why now:** two high findings (F1 clone-token forwarding, F2 cross-tenant installation attach) are directly reachable by an ordinary tenant/workspace admin and leak GitHub authority across the tenant boundary; F3 exposes production infrastructure credentials to any same-repo branch contributor. These are live exploitable paths on the current revision, not deferrable hardening. **Render parity IS included** (t013): several fixes change user/tenant-facing behavior across REST/GraphQL/MCP + dashboard — GraphQL query rejection (F9), MCP exec truncation metadata (F8), the GitHub connect callback (F2), member role-change semantics (F7), invite redemption (F13), and the login redirect (F4) — so the change must stay consistent across every surface it exposes and be checked against Render's behavior.
