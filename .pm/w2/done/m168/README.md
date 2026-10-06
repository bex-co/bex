# w2 · m168 — Email verification is a hard per-request gate

**Worker:** worker2 **Goal:** a human identity whose email is unverified can't use bex through any surface (dashboard, REST/GraphQL/MCP, CLI, SSH, OAuth relying parties) until it verifies. The ADR075 D7 card gate stays the second gate after it. **Status:** done (2026-10-06; shipped `22180488a`, live-verified on prod and dev-2, see [evidence.md](evidence.md))

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Revise ADR075 D8: adopt Option B, fix scope, exemptions and the allowlist — **DONE** | 30m | — |
| t002 | bex-api: refuse unverified humans with `EMAIL_VERIFICATION_REQUIRED` before onboarding — **DONE** | 1h | t001 |
| t003 | SSH gateway, web shell and ticket mints honor the gate — **DONE** | 30m | t002 |
| t004 | OAuth consent refuses unverified subjects for every client — **DONE** | 45m | t001 |
| t005 | Dashboard verification wall for signed-in unverified sessions — **DONE** | 1h | t002 |
| t006 | Production rollout and live verification — **DONE** | 30m | t003, t004, t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage + dev-N end-to-end acceptance — **DONE** | 45m | t007, t008 |
| t010 | Closeout — **DONE** | 10m | t009 |

## Definition of done

- On a `dev-N` stack, sign up with password and leave verification unfinished. The session must then be refused by all of these:
  - REST `GET /v1/services`: 403, `EMAIL_VERIFICATION_REQUIRED`.
  - The same call through GraphQL (`extensions.code`) and MCP (a tool error).
  - The pinned Render CLI after an OAuth device login.
  - An SSH session.
  - An `authorization_code` flow for a third-party Hydra client: no code is issued.
- The refusal happens before `EnsureTenant`, so no personal workspace is minted. After verification, the same session proceeds through `/setup/payment` (D7) and the same calls succeed. No re-login is needed.
- The dashboard sends a signed-in unverified session from any app route to `/auth/verification`, keeping the guarded `next`. The resend flow works. Sign-out and the self-host/delete-account exit stay reachable. The pending state's skeleton matches the verification page.
- Machine callers (`client_credentials` API keys, `bex-bootstrap`) are unaffected. Every exemption is explicit in code, with a test. Recovery still marks the address verified (re-probe recorded).
- In prod, a disposable unverified identity is refused live (REST + OAuth) and then removed. The 41 legacy unverified identities hit the wall on their next request; none is grandfathered (user decision 2026-10-06). The ADR075 D8 text, `lego/backend/AGENTS.md` and `dashboard/AGENTS.md` describe the shipped behavior.

## Source + Goal linkage

- **Source:** [finding.md](finding.md), promoted from inbox note w2/045 (forum.bex.co OIDC setup, 2026-10-05). Its Triage section has the read-only prod evidence. **User decision 2026-10-06:** make email verification a hard gate, un-deferring ADR075 Option B and reversing D8's "no bex-api gate" deferral.
- **Goal linkage:** ADR008 hosted-product trust and the ADR075 open-signup preconditions (PRFAQ001 §open signup). This is the same class of before-open-signup gate as D7.
- **Expected outcome:** unverified identities can't use any product surface or OAuth relying party. The D8 invariant becomes true per request and no longer depends on session lifetime. Today a sliding 7-day session can keep an unverified sign-up alive indefinitely.
- **Why now:** third-party OIDC relying parties (forum.bex.co, `bex-forum`) are going live now, and they inherit any live Kratos session. Of 69 prod identities, 41 are unverified, and 15 of those already hold workspace membership. The card gate (`all`, confirmed live) stops resource creation but not identity use.
- **Render parity included:** the refusal is a new error across REST/GraphQL/MCP plus a dashboard wall. Render's shape is a logged-in `verify-email` wall (ADR075 §research).
- **Coordination:** the `bex6` checkout has uncommitted `OAUTH_IDENTITY_CLIENTS` / `identityclaims` consent work in `dashboard/src/common/server-fn/hydra-consent.ts`. t004 must rebase onto whichever lands first, not fork it.
