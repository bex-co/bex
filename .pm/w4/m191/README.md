# w4 · m191 — Track TCP and HTTP root health changes as different releases

**Worker:** worker4 **Goal:** a change between TCP health checks and HTTP `/` completes a tracked release, with a new revision and an honest Live deployment. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Distinguish TCP from HTTP root in release identity | 40m | — |
| t002 | Adopt legacy fingerprints without unsolicited rollouts | 60m | t001 |
| t003 | Audit shared identity and retained-configuration consumers | 40m | t002 |
| t004 | Verify health and deploy contracts across surfaces | 30m | t003 |
| t005 | Simplify the changed code | 20m | t004 |
| t006 | Test mode transitions, adoption and release history | 45m | t004 |
| t007 | Replay the live journeys, clean up and close out | 30m | t005, t006 |

## Definition of done

- Create an owned Free BusyBox web service using [finding.md](finding.md)'s command. Wait for its default TCP deployment to become Live. Settings → Health Check Path → `/` → Save changes produces exactly one configuration deployment which becomes Live and advances the served revision; fresh-load and repeat. The public root keeps returning the fixture's HTTP200 marker.
- On a fresh fixture, set `/healthz` and wait for Live, then `/` and wait for Live, then clear the path and wait. Each actual change produces a separate Live release with the saved path retained after reload. Clearing `/` must advance the revision as clearing `/healthz` already does.
- The deployment detail/list and GraphQL `service`/`deploys`, REST get-deploy and MCP `get_deploy` report the release which actually served. A successful mode switch must never end as “Superseded by a newer release” when no newer deployment exists. Preserve the normal health budgets and real cancel/supersede handling.
- The non-root controls in [evidence.md](evidence.md) still complete: TCP → `/healthz`, `/healthz` → `/`, and `/healthz` → TCP. The public root and health markers stay reachable; an actual Cancel + Proceed still returns a canceled deployment.
- Delete the owned fixtures, verify their API/public404s and six workspace inventory families, revoke only the sweep's session and remove its logout handle. Close only after these live states and the task-level compatibility tests hold.

## Source + Goal linkage

- **Source:** user-requested continuous `qa-find-bugs` functional dashboard hunt, cycle33, 2026-10-09–10, workspace bex. [finding.md](finding.md) traces the defect; [evidence.md](evidence.md) preserves exact requests and complete responses. Local captures: `.playwright-mcp/qa-health-mode-a33-ledger.json`, `qa-health-mode-a33-canceled.png`, `qa-health-mode-a33-reverse-created.png`.
- **Goal linkage:** ADR008 reliable hosting and truthful machine surfaces; ADR004 separates TCP-default and explicit HTTP health checks; ADR006 exposes one deploy contract on REST/GraphQL/MCP; ADR018 records Render parity.
- **Expected outcome:** health-mode edits update probe configuration under a distinct release generation and finish Live without inventing a superseding release or overwriting the previous release's identity.
- **Why now:** reproduced in both directions on healthy Free services. The first deployment falsely closed canceled after18m12s; fresh repetitions started replacement instances while retaining the old revision. Ordinary non-root health changes completed in13–19s.
- **Render parity included:** this changes observable deployment/history behavior and must preserve the supported HTTP/TCP contract on every exposed surface. Render documents the two modes; its exact deploy-row timing for a path edit was not tested.
- **Scope:** researched filing only. No product fix was implemented by this hunt. This is the root-path identity gap, separate from w4/m190's HTTP Host-header failure. Legacy fingerprint adoption makes the work larger than a sub-hour edit.

## Unverified

Production App/Deployment objects and release-record contents were not read. Their generation/probe/record behavior is traced from code, while public responses, instance starts, revisions and deployment states were observed. Legacy upgrade, direct CR edits, cancel-template restoration, rollback, source builds, concurrent pending builds and sibling service types require task-level regression checks; they are not additional observed bugs.
