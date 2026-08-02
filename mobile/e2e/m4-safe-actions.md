# m4 authenticated device qualification

Run this release gate against a disposable, non-production workspace on one physical iOS device and one physical Android device. Source-level completion does not claim this checklist has been executed with production tenant credentials.

## Setup

- Create one service with a successful deploy plus one active deploy that can be canceled.
- Create one Postgres instance and one Key Value instance in actionable states.
- Include one protected environment so the server-required confirmation phrase is exercised.
- Keep the dashboard audit/event views open as independent server evidence.

## Actions

- Trigger a deploy, cancel the active deploy, and roll back to a known successful deploy. Confirm each dialog binds the intended service/deploy id and that history converges without duplicate operations.
- Restart, suspend, and resume the service. Confirm unsupported/transitional states remove the corresponding control.
- Restart, suspend, and resume Postgres; suspend and resume Key Value. Confirm Key Value never offers restart.
- On the protected resource, verify the first refusal displays the exact server phrase and requires a second explicit confirmation.
- Remove the caller's `can_operate` relation and verify authorization denial is explained without optimistic success.

## Network and lifecycle

- Interrupt the network after confirming an action but before the response arrives. Verify the result is reported as possibly committed, no blind retry is offered, and pull-to-refresh reconciles server truth before another confirmation.
- Double-tap every confirmation and verify the server records at most one intended operation.
- Background and foreground the app during convergence. Verify current server state refreshes and no stale result crosses a workspace/logout boundary.

## Negative scope

- Verify there is no route or control for deletion, PITR, failover, workspace/billing administration, plans, scaling, autoscaling, registry credentials, API/SSH keys, connection allowlists, or build/settings configuration.
- Verify the dashboard event/audit evidence names the same authorized action and target shown on the phone.
