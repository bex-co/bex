# w11 · m6 — Phase-1 agent mission control

**Worker:** worker11 **Goal:** let a configured tenant create, track, steer, and cancel a fire-and-forget coding-agent session from a phone, inspect bounded evidence, receive PR-ready push, and hand deep review to GitHub Mobile. **Status:** blocked on w3/m41 and w11/m5

## Gating

Hard gate: `w3/m41/t008` and `w11/m5/t011`. Consume the agent image/credentials/API/egress/completion work in w3/m37–m41; do not reimplement it. Provider credentials, raw model endpoints, template selection, and broad egress remain desktop configuration.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose a mobile-safe agent capability and readiness projection | 45m | w3/m41/t008, w11/m5/t011 |
| t002 | Add the typed session client, list, and terminal-state handling | 60m | t001 |
| t003 | Build the thin repo/branch/profile/prompt task composer | 60m | t002 |
| t004 | Add evidence, failure, cancel, and draft-PR detail | 60m | t003 |
| t005 | Add PR-ready/failure push and honest degraded states | 45m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 60m | t006 |
| t009 | Closeout | 10m | t008 |

## Definition of done

A preconfigured user selects a repo/branch and approved agent profile, submits a prompt once, observes lifecycle state, cancels safely, inspects bounded command/test evidence, receives failed/PR-ready push, and opens the draft PR in GitHub Mobile. Missing GitHub/provider readiness gives a desktop-configuration callout rather than exposing secrets or infrastructure parameters. Cross-workspace access, duplicate submit, terminal-state steering, and absent gateway configuration fail honestly.

## Source + Goal linkage

- **Source:** ADR048 M2 and ADR047 D4/D8 phase 1; depends on w3/m41's delivery/evidence contract.
- **Goal linkage:** ADR008 pillar 5 and ADR048's differentiating delegation loop.
- **Expected outcome:** “assign from phone → get evidence and a draft PR” works without rebuilding PR review or agent infrastructure in the client track.
- **Why now:** materialized now for dependency clarity, but blocked until the existing phase-1 integration and push close; this avoids parallel contract invention.
- **Render parity:** included because agent sessions are a bex extension exposed consistently across REST/GraphQL/MCP, while repo/lifecycle primitives retain their established behavior.
