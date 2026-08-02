# w11 · m2 — Secure native shell: auth, API, workspace, navigation

**Worker:** worker11 **Goal:** turn the clean Expo shell into a secure first-party bex client that authenticates through a reviewed native flow, calls bex-api, partitions state by workspace, and provides accessible navigation without changing the dashboard's Kratos-cookie architecture. **Status:** done

## Gating

Start after `w11/m1/t008`. Task t001 must reconcile ADR048 D5 before implementation chooses a native release posture.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile ADR048's delivery vehicle and native auth threat model — **DONE** | 45m | w11/m1/t008 |
| t002 | Implement system-browser PKCE authentication and secure session storage — **DONE** | 60m | t001 |
| t003 | Wire typed Apollo/codegen access to bex-api — **DONE** | 45m | t002 |
| t004 | Add workspace bootstrap, switching, and cache isolation — **DONE** | 45m | t003 |
| t005 | Build the accessible Expo Router shell and deep-link states — **DONE** | 45m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 10m | t008 |

## Definition of done

The iOS and Android production bundles implement system-browser Authorization Code + PKCE, cold-restore refresh rotation, local-first logout, typed bex-api workspace bootstrap, and fail-closed tenant/cache boundaries. Automated failure-boundary tests prove credentials never enter AsyncStorage, WebView state, logs, or generated config; workspace switches/logout abort or partition pending and cached state; and invalid deep links cannot bypass auth/workspace guards. Expo config/export, backend schema codegen, backend tests, bilingual strings, safe areas, light/dark tokens, reduced motion, rotation, and accessibility semantics are green. The web dashboard remains a Kratos-cookie first-party client. Physical OS-browser/Keychain/Keystore qualification remains mandatory before distribution and is tracked separately in `w11/002` plus `mobile/e2e/m2-real-device.md`, because no app-store release is part of this source milestone.

## Source + Goal linkage

- **Source:** ADR048 D5 gap 5 plus the user-directed Expo adoption; three-agent architecture/auth audit 2026-08-02.
- **Goal linkage:** ADR008 API-first operation and ADR012's fail-closed identity boundary.
- **Expected outcome:** a secure, reusable native entry point exists before any resource or agent feature is exposed.
- **Why now:** copied Beancount JWT/WebView assumptions are unsafe for bex; identity and cache isolation must be settled before feature work.
- **Render parity:** included because the client consumes tenant-facing GraphQL and must not invent fields, error semantics, or auth behavior that drift from REST/MCP/dashboard contracts.
