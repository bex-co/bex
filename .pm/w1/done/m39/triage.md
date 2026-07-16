# w1/m39 Dependabot triage — 2026-07-15

The milestone was opened against 18 alerts. The live Dependabot API returned 31
open alerts when implementation began: 14 critical, 5 high, 11 medium, and 1
low. The increase is 13 duplicate `golang.org/x/crypto` advisories that GitHub
subsequently attached to `lego/operator/go.mod`.

## Classification

| alerts | severity | manifest | package | reachability | fix type | disposition |
| --- | --- | --- | --- | --- | --- | --- |
| #266–#278 (13) | 7 critical, 2 high, 4 medium | `lego/backend/go.mod` | `golang.org/x/crypto` 0.51.0 | Runtime: `cmd/ssh-gateway` imports `x/crypto/ssh`. The gateway does not intentionally expose agent-forwarding/invoking extensions, but the SSH protocol surface is tenant-facing, so the module is treated as reachable. | Minor security release | Safe set: upgrade to 0.52.0. |
| #279–#291 (13) | 7 critical, 2 high, 4 medium | `lego/operator/go.mod` | `golang.org/x/crypto` 0.51.0 | Unreachable advisory code: the operator imports only `x/crypto/bcrypt` from `internal/registry`; the 13 advisories affect SSH invoking/agent paths. | Minor security release | Safe set: upgrade to 0.52.0 anyway so the manifest contains no vulnerable module version. |
| #265 | high | `lego/backend/go.mod` | `github.com/moby/spdystream` 0.5.0 | Runtime transitive: `internal/sshgateway` → client-go `remotecommand` → SPDY. bex is the client side of this path, but the running-instance SSH bridge exercises the dependency. | Patch | Safe set: upgrade to 0.5.1. |
| #263 | medium | `dashboard/yarn.lock` | `js-yaml` 4.1.1 | Build/dev transitive through GraphQL codegen, ESLint, and TanStack's build plugin; tenant YAML is not parsed through these copies at runtime. | Patch | Safe set: update the 4.x copies to 4.3.0. |
| #254 | medium | `dashboard/yarn.lock` | `launch-editor` 2.13.1 | Dev-only through `@tanstack/devtools-vite`; the vulnerable Windows UNC editor-launch path is not shipped. | Patch | Safe set: upgrade to 2.14.1. |
| #252 | low | `dashboard/yarn.lock` | `@babel/core` 7.29.0 | Build/dev-only through GraphQL codegen, TanStack tooling, Vite, and ESLint. | Patch | Safe set: upgrade to 7.29.7. |
| #246 | medium | `dashboard/yarn.lock` | `@tanstack/start-server-core` 1.166.8 | Runtime-reachable: the dashboard SSR server accepts inbound server-function requests. | Coordinated minor | Safe set: upgrade React Start to the first compatible line carrying patched server-core 1.167.30 and align its router/build-tool packages. |

Every live alert is in the safe set. There is no major/breaking set and no
Dependabot dismissal set.

## Applied versions

| package | before | after |
| --- | --- | --- |
| `golang.org/x/crypto` (backend + operator) | 0.51.0 | 0.52.0 |
| `github.com/moby/spdystream` | 0.5.0 | 0.5.1 |
| `js-yaml` | 4.1.1 | 4.3.0 |
| `launch-editor` | 2.13.1 | 2.14.1 |
| `@babel/core` | 7.29.0 | 7.29.7 |
| `@tanstack/start-server-core` | 1.166.8 | 1.167.30 |
| `@tanstack/react-start` | 1.166.11 | 1.167.64 |

`yarn npm audit --all --recursive` also found five fixable findings that had not
yet appeared in Dependabot. They were patched proactively within their existing
parent constraints: `@ungap/structured-clone` 1.3.0→1.3.3, `ajv`
6.12.6→6.15.0, `brace-expansion` 1.1.12→1.1.16, `h3`
2.0.1-rc.16→2.0.1-rc.25, and `yaml` 2.8.2→2.9.0.

## Compatibility and test coverage

The first patched TanStack line made one stale Vite option observable:
`cssCodeSplit: false` requires a generated `style.css` manifest asset, while the
dashboard deliberately imports both CSS files with `?inline` and therefore emits
no CSS asset. Removing the obsolete option restores the production SSR build
without changing CSS delivery. The production `yarn build` gate is the direct
regression check for this behavior; no additional unit test can exercise Vite's
two-environment manifest handoff more directly.

Verification after the final bump set:

- `make test` from `lego/operator/`: pass, including envtest.
- `go test ./...` from `lego/backend/`: pass.
- `yarn typecheck`, `yarn test`, and `yarn build` from `dashboard/`: pass (196
  files, 1,165 tests; client + SSR + Nitro production build) after
  fast-forwarding onto the current `main`.
- `go mod verify` in backend and operator: pass.
- `yarn install --immutable`: pass.
- `yarn npm audit --all --recursive`: no security advisories remain. It reports
  only the three deprecation notices below.

## Non-security residuals

| package | path | reason | revisit condition |
| --- | --- | --- | --- |
| `node-domexception` 1.0.0 | GraphQL codegen → `node-fetch` | Deprecated in favor of the platform `DOMException`; no security advisory and no direct dependency. | Remove when GraphQL codegen's fetch chain drops it. |
| `tsconfck` 3.1.6 | `vite-tsconfig-paths` | Upstream package is marked unmaintained; no security advisory and no direct replacement within the current plugin. | Re-check when `vite-tsconfig-paths` replaces/removes it or a maintained compatible plugin is adopted. |
| `whatwg-encoding` 3.1.1 | TanStack build plugin → Cheerio | Deprecated recommendation only; build-time transitive and no security advisory. | Remove when TanStack/Cheerio's parser chain migrates to its recommended replacement. |

## Default-branch verification

Dependency commit `a2f1ae9ea8caacff2ff56124f48bf74177d52af6` is
published on GitHub's default branch. At `2026-07-16T01:31:02Z`, the live
Dependabot API returned zero open alerts across every severity (31 → 0). No
security alert required dismissal or deferral; the non-security deprecations
above remain documented with explicit revisit conditions.
