# w7 · m74 — Dashboard auth & session-boundary hardening

**Worker:** worker7 **Goal:** close four auth/session-boundary gaps — device-grant auto-accept, unbounded consent body, stale Apollo cache across accounts, and replayable sandbox-exec tickets. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------ | ---- | ---------- |
| t001 | Device grant: require session + explicit confirmation before accept | 1h   | —          |
| t002 | Consent POST: body-size cap + session-before-formData              | 30m  | —          |
| t003 | Clear Apollo store on login/logout boundary                        | 20m  | —          |
| t004 | Sandbox-exec nonce: cross-replica DB claim                         | 30m  | —          |
| t005 | Simplify                                                          | 20m  | t001–t004  |
| t006 | Test coverage                                                    | 1h   | t005       |
| t007 | Closeout                                                         | 10m  | t006       |

## Definition of done

A `verification_uri_complete` link cannot complete a device grant without an authenticated, confirmed session bound to the accept subject. `/auth/consent` rejects oversized bodies before buffering. The browser Apollo cache does not cross a logout→login boundary. A sandbox-exec ticket cannot be replayed across the two gateway replicas. Verified per-surface.

## Source + Goal linkage

- **Source:** codex-security scan findings #9 (medium, device grant), #11 (medium, consent body), #24 (low, Apollo cache), #30 (low/med, sandbox-exec replay), validated against HEAD.
- **Goal linkage:** Security pillar — authn/authz integrity and per-account/per-replay boundaries (ADR012-auth; ADR035-ssh § Browser Web Shell / sandbox exec).
- **Expected outcome:** no device-grant CSRF, no consent-POST memory blowup, no cross-account cache disclosure, no cross-replica ticket replay.
- **Why now:** these are user/auth-facing integrity gaps; the browser-shell sibling already solved the cross-replica nonce that #30 skipped, giving a proven pattern to copy.
- **Render parity omitted:** these are bex-specific auth/session internals and an internal exec-ticket mechanism — not Render REST/GraphQL/MCP feature surface — so there is no render.com behavior to mirror. (Render's own device flow is the client side, driven by the official CLI.)
