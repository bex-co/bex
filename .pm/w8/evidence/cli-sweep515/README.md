# CLI sweep 515 — settings updates on a suspended service

**Verdict: no bug filed.** Companion to sweep 503 (deploy/restart refuse while suspended): configuration edits are accepted, held truthfully as undeployed, and roll out on the next deploy after resume. Same isolated session, `bex-canary`, HEAD build at `c603c8ec2` (tree has since advanced), 2026-10-19T09:17–09:19Z.

One free image web service created → `live` → suspended (REST, `202`; host `503`) → two CLI updates → resumed → manual deploy → deleted by ID (former-URL `404`).

| step | observed result |
| --- | --- |
| `services update --health-check-path /healthz515` while suspended | Exit `0`; readback shows the new path, still `suspended: suspended`, host still `503`, `services instances` still `[]`. |
| `services update --name …upd515b` while suspended | Exit `0`; name changes, **slug stays** `…upd515` (immutable address) — the old-slug host keeps answering the suspended `503`; a guessed new-slug host is `404` because it never existed. |
| Deploy bookkeeping for the held change | The update opened a `service_updated`/`config_change` deploy row that settles `canceled` within ~9 s with `cancelReason: "Canceled: the service was suspended"`; the service reads `undeployedChanges: true`. Nothing rolled, nothing woke. |
| Resume | `202`; the last live release serves `200` again (~15 s) with `undeployedChanges` still `true` — resume restores the last build, it does not silently roll held settings. |
| `deploys create --wait` | Exit `0`, new deploy `live`, `undeployedChanges` clears, host `200` under the new health-check path. |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbau03vbcrps73dmoang` deleted by ID; former-URL 404 verified. Suspend/resume ran over REST (no pinned-CLI verb), as setup for the CLI update journey.
- Adjacent-but-distinct from w8/m119–m121 (releases held *through rollout* by a newer save): here the service was fully suspended before any edit, and every surface agreed at each step, so nothing here contradicts or duplicates those milestones.
- Unrun: plan change while suspended (paid-plan rules put it out of hunt scope), env-var edit while suspended reaching the process after resume+deploy (env has no `services update` flag at this pin).
- Deploy lag means deployed revisions can precede local `main`.
