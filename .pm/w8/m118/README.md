# w8 · m118 — Redirect tenant HTTP to HTTPS

**Worker:** worker8 **Goal:** A plain-HTTP request to any tenant web host (platform subdomain or custom domain) is redirected permanently to the same `https://` URL, as on Render, so credentials and cookies never travel in cleartext and apps only ever see `x-forwarded-proto: https`. **Status:** todo（2026-10-29 解除阻塞，待线上验收；原记录：t001, t006, t002, t003, t004 done; t005 needs production operator rollout and the owned Cloudflare replay; the user authorized shipping on 2026-10-28. Implementation and local verification completed 2026-10-28.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Redirect the App Ingress's HTTP router to HTTPS | 90m | — |
| t006 — **DONE** | Keep a missing platform TLS Secret from breaking a valid custom domain | 90m | w8/m118/t001 |
| t002 — **DONE** | Render parity: redirect contract across hosts and surfaces | 20m | w8/m118/t006 |
| t003 — **DONE** | Simplify the changed code | 15m | w8/m118/t002 |
| t004 — **DONE** | Test coverage: redirect, exclusions, attribution, ACME paths | 40m | w8/m118/t002 |
| t005 | Close out the milestone | 15m | w8/m118/t004 |

Six tasks, 270m. All implementation and standing verification tasks are complete; t005 remains a production closeout.

## Definition of done

- `curl -sI http://<slug>.onbex.co/x?y=1` on production returns a permanent 3xx whose `Location` is `https://<slug>.onbex.co/x?y=1`. The same holds for a verified custom domain and for a static site.
- HTTPS behavior is unchanged: the same response bodies, the `websecure-` router carries the bytes (w1/m51), and an IP allow-list still refuses with 403.
- cert-manager HTTP-01 issuance for a new custom domain still succeeds, because the solver's own Ingress is not redirected. With the platform subdomain enabled, a missing platform-host TLS Secret does not prevent that ready custom certificate from serving trusted HTTPS.
- `BEX_HTTPS_REDIRECT=false` keeps a cluster without certificates serving plain HTTP.

## Source + Goal linkage

- **Source:** inbox note `w8/121` (promoted 2026-10-18). `/qa-find-bugs-cli` sweep 114, 2026-10-18: live fixture `qa-20261018-38b5ed-echo` returned `curl http://<slug>.onbex.co/plain` → `HTTP/1.1 200` with `x-forwarded-proto: http` at the app, and no `Location` or HSTS. Traefik's `web` entrypoint deliberately has no global redirect, so HTTP-01 challenges work (`deploy/gitops/base/values/traefik.values.yaml:171`). No per-router `RedirectScheme` replaces it for tenant routes; `host_redirect` only redirects host to host.
- **Goal linkage:** ADR008 Render-alternative core hosting. Render: "Render automatically redirects all HTTP requests to HTTPS".
- **Expected outcome:** no tenant traffic is served over cleartext HTTP. Apps that build absolute URLs from `x-forwarded-proto` produce `https` links.
- **Why now:** this is a security-relevant parity gap found live. Session cookies sent to an `http://` link travel unencrypted.
- **Render parity included because** the redirect is tenant-facing behavior of every web service. No REST, GraphQL or MCP shapes change; t002 verifies the hosts, the dashboard's URLs, and the docs.

## Hosted probe — qa pass 8a3f50 (2026-10-19, not the t005 closeout)

A `/qa-find-bugs w4` pass ran t005's owned-fixture checks on production, pinned to `f3871ba9fd44`, which contains `caeb18e41`. bex-canary; the owned Free fixtures `qa-20261019-8a3f50-tls` (web, `srv-dbao8n0u2drc73bm0pag`, image `python:3.13-alpine`) and `qa-20261019-8a3f50-tlsst` (static, `srv-dbao8n74o0uc73fuig2g`, `examples/static-site`) were deleted afterwards (404), and only that run's session was revoked. The `curl` probes ran from outside the browser at 02:46–02:47Z.

| Probe (both hosts unless noted) | Result |
| --- | --- |
| `GET http://<slug>.onbex.co/x?y=1` | `301 Moved Permanently`, `Location: https://<slug>.onbex.co/x?y=1` |
| `HEAD http://<slug>.onbex.co/` | `308 Permanent Redirect`, `Location: https://<slug>.onbex.co/` |
| `POST http://<slug>.onbex.co/p?q=2` | `308`, `Location: https://<slug>.onbex.co/p?q=2` |
| `curl -L http://…/x?y=1` | 1 redirect, final URL `https://…/x?y=1` |
| `GET https://<slug>.onbex.co/` | `HTTP/2 200`: the web service served python's "Directory listing for /" (760 B), and the static site served "bex static site" (1548 B) |
| Web host after `setServiceIpAllowList` to `203.0.113.0/24` only: `GET https://…/` | `HTTP/2 403` "Forbidden" (first poll) |
| Same, `GET http://…/x?y=1` | `301` to `https://…/x?y=1`, and following it gives `403`: the redirect runs before the allow-list, and nothing beyond the hostname is disclosed |

Not covered: the custom-domain bullet and HTTP-01 issuance (no domain this run controls), and `BEX_HTTPS_REDIRECT=false` (cluster config). No `Strict-Transport-Security` header is sent on HTTPS; the DoD does not ask for one.

## Cloudflare unblock investigation — 2026-10-27 UTC

The user's existing `cf` CLI provides DNS access. [The owned live test](evidence/cf-20261027/README.md) completed HTTP-01 issuance and passed custom-only redirect/HTTPS/403 controls, but exposed a default mixed-host TLS failure: the missing rate-limited `onbex.co` Secret stops Traefik from loading the ready custom certificate. t006 is the next actionable task; no external access or policy decision is outstanding. All test resources and DNS were deleted, with physical cluster cleanup and session revocation verified. This test does not close the milestone.

## TLS fix prepared — 2026-10-28

The missing-Secret loading failure is fixed in the working tree. [Pinned provider verification](evidence/provider-20261028/README.md) reproduces the old failure, then passes trusted HTTPS and redirect checks for both missing-platform and missing-custom neighbors. Issuance/deletion, incomplete Secrets, direct static sites, sleeping services and namespace-safe reads have regression coverage. Full operator tests and all-module lint/deadcode passed. The disposable test cluster was deleted. No production operator deployment or commit occurred; t005 is the only remaining task.

## Ship authorization — 2026-10-28

The user explicitly requested `ship`, clearing the commit/push gate. The prepared fix is being landed through the ship workflow. The remaining owner is the release pipeline for the production operator rollout, followed by QA for t005's owned Cloudflare replay and cleanup. No production acceptance is claimed by the ship.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m118
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。
