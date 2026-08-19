# w8 · m27 — Granular OAuth capability scopes and authorization-decision audit

**Worker:** worker8 **Goal:** let a human delegate least-privileged control-plane access to a third-party OAuth client and leave bounded, queryable evidence of the effective grant on every recorded authorization decision **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define the closed capability vocabulary and compatibility policy — **DONE** | 45m | — |
| t002 | Carry normalized OAuth grant provenance through `core.Identity` — **DONE** | 45m | t001 |
| t003 | Enforce the capability matrix at the shared authorization seam — **DONE** | 60m | t002 |
| t004 | Persist and expose bounded OAuth authorization provenance — **DONE** | 60m | t003 |
| t005 | Update consent, discovery, platform clients, and Connected Agents — **DONE** | 45m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 60m | t006 |
| t009 | Closeout — **DONE** | 15m | t007, t008 |

## Definition of done

A non-platform human OAuth token issued to the Bex API audience can exercise an ordinary read only with `bex.read`, a mutation only with `bex.write`, and a sensitive-value read only with `bex.sensitive`, in addition to the caller's existing OpenFGA permission. Missing or near-match capabilities fail closed with one coded insufficient-scope error across REST, GraphQL, and MCP, including `Can`-based response shaping. Kratos sessions, machine API keys, the official Render CLI launcher, and the mobile platform client retain their existing effective authority; legacy `bex.api` is accepted only for explicitly platform-marked clients during the documented rollout. Each authorization event the existing audit policy records identifies the subject, OAuth client, accepted audience, canonical scope set, required relation, and outcome without storing a bearer token or attacker-controlled free-form metadata. Discovery, consent, Connected Agents, docs, and executable tests all describe the same closed vocabulary and migration behavior.

## Source + Goal linkage

- **Source:** Proposal 1 from `/pm-brainstorm for w8`, selected by the user on 2026-08-18. It promotes the deferred operation-level least-privilege follow-up in `docs/ADR069-security-review-round14.md` Finding 1 and `docs/ADR012-auth.md`'s current single-`bex.api` scope contract.
- **Goal linkage:** `.pm/GOAL.md` goals 5 and 7 plus ADR008's agent-facing control-plane pillar: tenant authorization must remain enforceable when a human delegates access to a third-party client, and security decisions must be reviewable after the fact.
- **Expected outcome:** Third-party clients can request only the capabilities they need; a read-only client cannot mutate or reveal sensitive values even when its user is an admin; operators can prove which client, audience, scopes, relation, and result governed a recorded decision.
- **Why now:** Round 14 closed the audience/no-scope privilege hole with the coarse `bex.api` backstop and explicitly deferred per-operation scoping. `core.Base` is now the single REST/GraphQL/MCP authorization seam and the audit interception point, so the matrix can land once instead of drifting across adapters. Render parity is included because OAuth is a Bex extension but its denials, audit projections, dashboard consent/settings UI, and official-CLI behavior cross every user-facing surface.
- **Anti-goal boundary:** The dashboard remains a first-party Kratos-session application; this milestone does not store browser-readable OAuth tokens, build a Hydra login provider, change OpenFGA roles, add per-resource grants, or fork the official Render CLI.
