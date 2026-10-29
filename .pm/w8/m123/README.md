# w8 · m123 — Sanitize client-identity headers at the tenant edge

**Worker:** worker8 **Goal:** No tenant app ever receives a caller-forged `True-Client-IP`, `CF-Connecting-IP`, `CF-IPCountry` or `X-Client-IP`. `True-Client-IP` always carries the real client address, as it does on Render behind Cloudflare. A tenant custom domain behind the tenant's own Cloudflare proxy keeps its genuine `CF-*` headers. **Status:** todo（2026-10-29 解除阻塞，待线上验收；原记录：t001–t006 done 2026-10-20; t007 live check on production after GitOps syncs Traefik

## Tasks (in order)

| id   | title                                                                    | est | depends_on                 |
| ---- | ------------------------------------------------------------------------ | --- | -------------------------- |
| t001 — **DONE** | Traefik local plugin: Cloudflare-aware client-identity header sanitizer  | 60m | —                          |
| t002 — **DONE** | GitOps: load the plugin and attach it to the `web`/`websecure` entrypoints | 40m | w8/m123/t001               |
| t003 — **DONE** | Guards: gitops-validate for two plugins, entrypoint wiring, range drift  | 30m | w8/m123/t002               |
| t004 — **DONE** | Render parity: the client-IP header contract in ADR018/ADR022            | 15m | w8/m123/t003               |
| t005 — **DONE** | Simplify the changed code                                                | 15m | w8/m123/t004               |
| t006 — **DONE** | Test coverage: forged, plain and Cloudflare-peer requests; plugin load   | 30m | w8/m123/t004               |
| t007 | Close out the milestone                                                  | 30m | w8/m123/t005, w8/m123/t006 |

## Definition of done

Live on production after GitOps syncs Traefik, on a fresh free echo web service (`docker.io/ealen/echo-server:0.9.2`) deleted by ID with a 404 proof:

- `curl https://<slug>.onbex.co/cf -H 'CF-Connecting-IP: 203.0.113.66' -H 'True-Client-IP: 203.0.113.77' -H 'X-Client-IP: 203.0.113.88' -H 'CF-IPCountry: KP'`: the container sees `true-client-ip` equal to the real client address (the same value as `x-real-ip`), and no `cf-connecting-ip`, `cf-ipcountry` or `x-client-ip`.
- A plain request carries `true-client-ip` equal to `x-real-ip`. The `X-Forwarded-*`/`X-Real-IP` controls are unchanged.
- A static site and a custom domain behave the same.
- A request whose TCP peer (after PROXY protocol) is inside Cloudflare's published ranges keeps its `CF-Connecting-IP`/`CF-IPCountry`, and its `True-Client-IP` is the `CF-Connecting-IP` value. Proven by the plugin's unit tests, since no tenant orange-cloud fixture exists.
- `scripts/gitops-validate.sh` passes, and the rendered Traefik shows both plugins mounted and the entrypoint middleware.

## Source + Goal linkage

- **Source:** promoted from inbox note `w8/143` on 2026-10-20 (`/qa-find-bugs-cli` sweep 1241, [evidence](../evidence/cli-sweep1241/README.md)). The note's text is kept in [`done/143.md`](../done/143.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render alternative, and [ADR018](../../../docs/ADR018-render-parity.md)'s inbound-IP row: Render's own guidance tells apps to trust `CF-Connecting-IP`/`True-Client-IP` for rate limits, allowlists and audit logs, because Cloudflare overwrites them. On bex a migrated app trusts caller-controlled values.
- **Expected outcome:** IP-based rate limits and audit trails in migrated apps stop being spoofable. Apps keep one header that always names the client.
- **Why now:** severity major (security-relevant parity gap); the fix is edge-only and needs no tenant change.
- **Why the preferred variant, not the note's minimum:** stripping `CF-*` unconditionally would also strip the genuine value for a custom domain behind the tenant's own Cloudflare proxy. Traefik overwrites `X-Forwarded-For` with the Cloudflare edge address, so those apps would lose the real client IP entirely. A Cloudflare-aware sanitizer keeps them working and closes the spoof, including the bypass of hitting the bex LB directly with a forged header. Render parity is included because the header contract is user-visible on every web surface.

## Design

- **t001:** a new local plugin `deploy/gitops/charts/traefik-plugins/clientipheaders/` (module `github.com/bex-co/bex-client-ip-headers`, Yaegi-safe: stdlib only, no `unsafe`/reflection tricks). Per request, with `peer` the post-PROXY `RemoteAddr`:
  - delete `X-Client-IP`;
  - if `peer` is inside the pinned Cloudflare ranges and `CF-Connecting-IP` parses as an IP: keep the `CF-*` headers and set `True-Client-IP` to it;
  - otherwise delete `CF-Connecting-IP`, `CF-Connecting-IPv6`, `CF-IPCountry`, `CF-Ray`, `CF-Visitor` and `CF-Worker`, and set `True-Client-IP` to `peer`.
- **t002:** a ConfigMap generator entry, a Traefik volume and `experimental.localPlugins` entry, a `Middleware` (`traefik` namespace) using the plugin, and `web`/`websecure` entrypoint middlewares in the base and prod values. The entrypoint covers every App, static site, custom domain and platform host.
- **t003:** `gitops-validate` checks each plugin ConfigMap's mounted name and the entrypoint middleware on both renders. A drift guard asserts the plugin's Cloudflare ranges equal `lego/backend/internal/core/proxiedhosts.go`'s `cloudflareRanges`.
- **Out of scope:** dropping `X-Forwarded-Server` (optional in the note; harmless pod name), and making bex's own allowlist trust `CF-Connecting-IP` (m150 Decision 2 stands).

## Blast radius

- Every HTTP request through the tenant entrypoints, platform hosts included (`api.bex.co`, `dashboard.bex.co`, `oauth.bex.co`). None of them trusts these headers (`core/proxiedhosts.go`).
- `experimental.abortOnPluginFailure: true`: a plugin that fails to load stops Traefik. t006 must load the plugin in a real Traefik (pinned v3.7.5) before t007.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m123
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。
