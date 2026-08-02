# w7 · m74 — Dashboard auth & session-boundary hardening

**Worker:** worker7 **Goal:** close four auth/session-boundary gaps — device-grant auto-accept, unbounded consent body, stale Apollo cache across accounts, and replayable sandbox-exec tickets. **Status:** done

## Tasks (in order)

| id   | title                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------ | ---- | ---------- |
| t001 | Device grant: require session + explicit confirmation before accept — **DONE** | 1h   | —          |
| t002 | Consent POST: body-size cap + session-before-formData — **DONE**   | 30m  | —          |
| t003 | Clear Apollo store on login/logout boundary — **DONE**             | 20m  | —          |
| t004 | Sandbox-exec nonce: cross-replica DB claim — **DONE**              | 30m  | —          |
| t005 | Simplify — **DONE**                                                | 20m  | t001–t004  |
| t006 | Test coverage — **DONE**                                           | 1h   | t005       |
| t007 | Closeout — **DONE**                                                | 10m  | t006       |

## Definition of done

A `verification_uri_complete` link cannot complete a device grant without a confirmed session; `/auth/consent` rejects oversized bodies before buffering; account-scoped cache does not cross a logout/login boundary; a sandbox-exec ticket cannot be replayed across gateway replicas. Verified per-surface.

## Source + Goal linkage

- **Source:** codex-security scan findings #9 (medium, device grant), #11 (medium, consent body), #24 (low, Apollo cache), #30 (low/med, sandbox-exec replay), validated against HEAD.
- **Goal linkage:** Security pillar — authn/authz integrity and per-account/per-replay boundaries (ADR012-auth; ADR035-ssh § Browser Web Shell / sandbox exec).
- **Expected outcome:** no device-grant CSRF, no consent-POST memory blowup, no cross-account cache disclosure, no cross-replica ticket replay.
- **Why now:** these are user/auth-facing integrity gaps; the browser-shell sibling already solved the cross-replica nonce that #30 skipped, giving a proven pattern to copy.
- **Render parity omitted:** these are bex-specific auth/session internals and an internal exec-ticket mechanism — not Render REST/GraphQL/MCP feature surface — so there is no render.com behavior to mirror.

## Ship record — DONE 2026-08-01

Shipped as `c7ed18e2` + build fix `0efe6e55` (deployed → GitOps pin `efb9ec07`). Four fixes: `handleDeviceVerification` renders a same-origin "Authorize the bex CLI?" confirmation interstitial for an authenticated caller, and only a same-origin session-bound POST (`handleDeviceConfirm`) calls Hydra's device/accept — a signed-in victim opening an attacker link no longer silently completes the grant (#9); `handleConsentDecision` fetches the session + caps the body at 64 KiB before `request.formData()` allocates (#11); login `onSuccess` + logout `finally` call `getClient().clearStore()` (the isomorphic wrapper — a direct `factory.client` import violated the SSR import-protection, caught by the production build gate) to drop account-scoped cache at the session boundary (#24); `consumeSandboxNonce` now claims the nonce atomically via `Store.ClaimShellNonce`, mirroring the Browser Web Shell, so a ticket can't replay across the two gateway replicas (#30). CI: typecheck + lint + 1777 tests + production build all green (the build fix was the m74 deploy's one repair commit).
