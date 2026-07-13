# w5 · m20 — Events tab + deploy list UI (Rollback/Cancel + Manual Deploy)

**Worker:** worker5 **Goal:** Ship the service-detail Events tab — a newest-first deploy feed with Manual Deploy, per-row Cancel (in-progress deploys) and Roll Back actions, all gated behind confirm dialogs, rollback provenance displayed. **Status:** DONE 2026-07-12 — Events tab route, GraphQL query/mutations, local-bex stub support, all locale strings (en + zh); backend gates w3/m7 + w2/m10 + w2/006 all materialized in this session.

## Tasks (in order)

| id   | title                                                                                                              | est | depends_on | status |
| ---- | ----------------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Backend gate: w3/m7 service events feed (events package, REST/GraphQL/MCP, composition root)                      | 60m | —          | DONE   |
| t002 | Backend gate: w2/m10 cancel + rollback verbs (store migration, service, REST/GraphQL/MCP)                         | 60m | —          | DONE   |
| t003 | Backend gate: w2/006 triggerDeploy GraphQL mutation                                                               | 20m | —          | DONE   |
| t004 | GraphQL: `events.graphql` (ServiceEvents query + TriggerDeploy/CancelDeploy/RollbackService mutations)            | 20m | t001–t003  | DONE   |
| t005 | Dashboard: Events tab route (`services.$serviceId.events.tsx`) with confirm dialogs + rollback provenance         | 60m | t004       | DONE   |
| t006 | Nav: add "Events" as second item in service-nav.tsx (after Overview, Render IA)                                   | 10m | t005       | DONE   |
| t007 | Locale: en + zh strings (nav, card title, all button/dialog/toast/status copy)                                    | 20m | t005       | DONE   |
| t008 | definitions.ts: hand-add ServiceEvents/TriggerDeploy/CancelDeploy/RollbackService Document types                  | 30m | t004       | DONE   |
| t009 | local-bex stub: ServiceEvents + TriggerDeploy + CancelDeploy + RollbackService handlers with in-memory state      | 20m | t008       | DONE   |

## Definition of done

The Events tab shows the deploy feed newest-first; Manual Deploy triggers a new deploy (confirm dialog); an in-progress deploy shows a Cancel button; any live deploy shows a Roll Back button; rollback provenance ("Rolled back from …") renders when trigger=rollback; all actions are idempotent in the local-bex stub; typecheck passes.

## Source + Goal linkage

- **Source:** `w5/007.md` (inbox note 2026-07-12); gates `w3/m7`, `w2/m10`, `w2/006` all materialized 2026-07-12.
- **Goal linkage:** pillar 1 (Render Events tab parity) + pillar 3 (agent-triggered deploys).
