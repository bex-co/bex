# Routine permission refresh silently dismisses action confirmations

**Severity:** minor — a user must reopen the confirmation and race the refresh; immediate actions work and no unauthorized mutation was observed. Typed commit/protected-dialog draft loss is code-reasoned, not live-measured.

## Live reproduction and limits

QA sweep 41, 2026-10-03 UTC / 2026-10-02 local, signed-in Admin in workspace `tea-d98210cbbpdc73dcrkvg`. Owned Free image service `qa-20261002-rollback-r41`, `srv-db08gk8ehcmc739j10mg`, App UID `aee3cd52-aeac-4032-b572-27f11d30c1fb`.

1. Open `/services/srv-db08gk8ehcmc739j10mg/settings`, Manual Deploy → Restart service. Leave the dialog untouched without changing focus, workspace or service. The first attempt vanished before the next click; no Restart mutation was sent. A fresh page load reproduced disappearance within the 47.5-second observation window.
2. Instrument only DOM dialog presence and focus/visibility events, plus complete ViewerCapabilities responses. Restart opened at **04:53:28.351Z**, disappeared at **04:53:32.263Z** (3.912s), and a successful unchanged **ADMIN / all allowed** permission response arrived at **04:53:32.505Z**. No focus/visibility events occurred.
3. Open Settings → Suspend without confirming. It opened at **04:54:17.902Z**, disappeared at **04:54:32.293Z** (14.391s). Permission polls remained successful with unchanged grants. No Suspend mutation was sent.
4. Positive control: Update Source, type unsaved `traefik/whoami:qa-r41-unsaved-draft`, wait from **04:55:04.269Z to 04:55:47.486Z**. Dialog remained open and draft stayed intact across the same polling lifecycle; cancel it, never save or deploy that draft.
5. **Counterexample retained:** another Restart opened at **04:55:48.624Z** and was still open when cleanup began; it was canceled explicitly. Do not claim every dialog always closes after 30 seconds. This is an intermittent expiry/response ordering race. It reproduces across two bound consumers, not a universal fixed timer or whole-page remount.

Expected: retain same-context presentation while access is checked, with confirmation disabled and clear status until a fresh affirmative result. Actual: dialog disappears without a refusal or an action; refreshing the same access does not restore it.

## Independently successful hosting journey

Before the dialog probes, creation of traefik/whoami:v1.10.1 (port 8080, own WHOAMI_PORT_NUMBER=8080) reached Live. Saving v1.11.0 retained the old ready pod and pending flag. Standard deploy ran v1.11.0. Rollback to the first deploy ran v1.10.1 while saved image stayed v1.11.0; Restart retained v1.10.1; a standard deploy returned to v1.11.0 and cleared the pending flag. Every settled pod/image check agreed with HTTP 200 and the response hostname. Fresh Settings and REST/GraphQL/MCP reads agreed on saved state. Deploy IDs: initial `dep-db08gk8ehcmc739j10n0`, new image `dep-db08hk2tm2ss7389qleg`, rollback `dep-db08i6gehcmc739j10q0`, restart `dep-db08j5atm2ss7389qlg0`, final standard `dep-db08jfitm2ss7389qlhg`. Two 0s duration reports duplicate w4/m156, not a new filing. This rules out a broken restart/rollback backend as the cause of the disappearing dialogs.

## Root cause and actual framework path

- `dashboard/src/features/capabilities/lib/capability-policy.ts:36-44` sets a 30,000ms receipt-freshness bound; `common/lib/polling.ts:11` uses the same 30,000ms cadence. This creates a real interval where the old receipt can expire before the next successful response arrives.
- `features/capabilities/context/capabilities-provider.tsx:138-156,258-264,285-327` marks the old eligibility stale/checking and updates the clock at expiry. A later successful poll updates receipt/eligibility. No role downgrade is necessary; `:197-199` only bumps the access generation for a detected downgrade. The captured unchanged Admin responses fit this transient gap.
- `features/capabilities/hooks/use-bound-action-confirm.ts:70-72` defines eligible using loaded/not-stale/not-unavailable. `:78-107` uses that eligibility inside **isBindingCurrent**, both for dispatch and for the dialog's returned pending binding. Any transient false result derives pending to null and the effect invokes clearConfirm, discarding intent and aborting a check permanently.
- `features/services/components/manual-deploy-button.tsx:78-85,236` renders the restart dialog only when stored binding equals that pending binding; `suspend-service-card.tsx:92-98,193` likewise derives open from it. These two observed symptoms have separate consumer citations and the same shared root.
- `common/components/confirm-dialog.tsx:176-188` passes controlled open through `ui/alert-dialog.tsx:9-13`. Actual installed/pinned **@radix-ui/react-alert-dialog 1.1.15** `src/alert-dialog.tsx:26-30` forwards the controlled props to DialogPrimitive.Root; **@radix-ui/react-dialog** `dist/index.js:75-101` uses openProp in useControllableState and passes it to its context. There is no dialog lifetime timeout in this path: the application turns open false.
- Control consumer: `features/services/components/service-source-card.tsx` SourceEditDialog owns its dialog/draft state independently (local state around :116 and controlled Dialog around :195); changing capability availability gates the action rather than destroying the open binding. Live unsaved draft survived the poll. This is why that control is meaningful, not evidence that the provider never transiently loses eligibility.

## Target behavior and safety boundaries

Separate **same-context dialog intent** from **current permission to dispatch**. Keep the selected target/draft visible through an ordinary checking/stale/unavailable gap. Do not keep a usable authorization grant: disable confirm with translated checking/retry copy, permit Cancel, require a new user click after recovery, and run the existing fresh resource check before dispatch. No deferred click or mutation replay.

Keep actual identity/workspace/resource/deploy/access-generation invalidation destructive to old intent. Keep in-flight abort and post-await exact-context checks; a temporary stale period may invalidate an in-flight authorization attempt without deleting the user's same-context dialog. Confirmed denial must never enable the action. Do not just delete `current.eligible` from the current shared predicate, relax freshness, reuse an old allowed decision, or stretch the timer to hide the race. Use distinct presentation/context and dispatch predicates, and update every caller that uses isBindingCurrent after an await so security and UI intent are not accidentally conflated again.

Pre-settle behavior is part of the fix: the existing dialog remains visible with disabled confirmation and a reason while the query is pending; current fresh affirmative access restores availability in that same dialog. An unavailable result remains visible but disabled with retry guidance. A context/generation change closes obsolete intent; a late result cannot reopen it. Preserve selected-row rollback adjustment, extra can_create checks for specific commits and server-issued protected confirmation.

## Shared blast radius and aliases

Production grep finds **five useBoundActionConfirm invocations in four components** (excluding the definition/tests): ManualDeployButton twice (restart and specific-commit), SuspendServiceCard once, ServiceRowActions once, DeployActions once. Named callers: `manual-deploy-button.tsx:62-63`, `suspend-service-card.tsx:66`, `service-row-actions.tsx:126`, `deploy-actions.tsx:121`. Treat the fix as shared across these five consumers; regression-test the already-working immediate confirmation path as hard as the waiting path.

Resource census: web/private/background worker/cron/static use these App service/deploy components with their existing per-type action gates. Postgres and Key Value use separate datastore confirmation components and do not call this hook. Only web Restart/Suspend were live-probed for the closure; the other eligible actions/families are code-traced and need tests, not claimed live failures. Header restart, row lifecycle actions and deploy-row rollback/cancel are UI aliases; API mutation names and REST/MCP routes are unchanged because dispatch authorization, not server execution, owns this fix.

Adjacent classes remain distinct: checking/stale/unavailable cannot authorize; confirmed denial cannot authorize; unauthenticated/session loss clears its context; different workspace/resource/deploy/generation invalidates old intent; transient same-context refresh preserves only presentation. No new backend existence signal, permission grant or mutation semantics.

## History, dedupe and full prior-DoD disposition

`git show 3bf7de0d1` confirms that w6/m146 added eligible to the shared binding predicate and effect-based clear; the pre-change hook in `0a07b59c5` bound presentation to context/generation alone. `3bf7de0d1` is an ancestor of the deployed platform pin `1263d12ae`; latest product commits and targeted hook/provider history contain no later fix. This is a **presentation regression introduced by w6/m146's security fix**, not a request to undo that fix. User explicitly schedules the new finding in w4; w6/m146's still-blocked two-session authorization acceptance is not duplicated or closed here.

Every w6/blocked/m146 DoD item is dispositioned:

1. **Partial GraphQL errors/stale receipts/obsolete responses cannot grant actions:** preserve; code remains guarded. These adversarial states were not induced live here and require regression tests.
2. **Confirmation losing workspace/resource/deploy/access generation sends zero mutations:** preserve the exact-context and post-await checks. No context switch was performed live here; not claimed re-verified.
3. **Header/protected dialogs use shared guard, clear obsolete state and preserve current allowed actions/commit restrictions:** immediate restart/rollback worked, but temporary freshness now clears a still-current dialog. This filing repairs that presentation gap; protected and specific-commit variants remain unprobed live and must be covered.
4. **Two-session dev-6 walkthrough at desktop/mobile plus actual membership removal:** remains blocked/unverified under its existing owner. This production single-session fixture does not satisfy it or authorize modifications to dev-6.
5. **Required gates/parity/simplify/cleanup/original live records:** this is a researched filing, so product tests were not rerun and no old live checkbox changes. This sweep's own fixture/session cleanup passed; w6's separate retained fixtures and acceptance remain its owner's work.

Other matches: w1/done/m153 fixed Apollo-loading skeleton remounts and form loss. The source dialog control now survives, while this binding is explicitly cleared; do not re-file that older root. w4/done/m141 fixed selected-row rollback rechecks and silent refusals after a click. Its three DoD guarantees remain required: (1) action or named refusal at dispatch (the actual image rollback passed; no static re-probe), (2) selected row/button/recheck agreement (no adverse row case tested here), (3) named rollback controls (observed button named the target deploy). This bug is a pre-click transient presentation loss and does not establish a regression of m141's static backend fix.

Whole-tree open/blocked/done confirmation/refresh/stale/poll searches and open milestone titles were reviewed. DO_NOT_DO has no conflicting anti-goal. Existing m146 security scope and m153/m141 are preserved as above. ADR004 governs lifecycle actions, ADR024 authorization, and ADR006 adapter consistency. [Render's deploy documentation](https://render.com/docs/deploys#restarting-a-service), checked 2026-10-03, documents the same restart/manual deploy operation, but does not specify permission-refresh dialog behavior. No authenticated Render dialog-lifetime comparison was performed; do not claim one.

## Evidence, cleanup and unverified work

Screenshots `.playwright-mcp/qa-r41-dialog-open.png` and `qa-r41-dialog-closed.png` were visually inspected. Full captures in `qa-r41-api-captures.json`, network/console logs and the DOM/poll probe below survive as local artifacts; raw evidence needed for handoff is embedded here. The open screenshot was retaken after explicitly waiting for the alertdialog, so it actually shows the dialog. The third surviving attempt is retained in the final event list. Only expected post-delete 404 appears in the console.

No extra restart/suspend was submitted by the waiting probes. Owned service deleted: API/public URL 404, own App/Deployment/ReplicaSet/job/pod/service/Ingress/Secret inventory zero; QA session revoked. Ledger `.playwright-mcp/qa-muse-r41-ledger.json`, inventory `qa-r41-inventory-final.json`. HTTP hostname/image controls saved as `qa-r41-{initial,saved-b,deployed-b,rollback-a,restart-a,standard-b}.json`.

Unverified: actual role downgrade, transport failure, identity/workspace switch, protected phrase and specific-commit draft preservation, mobile layout, and other resource families. These remain explicit implementation/test tasks, not manufactured live findings. Timing is intermittent; tests must hold a permission response until after expiry to reproduce it deterministically.

## Exact observer and complete capability probes

Observer: record timestamps whenever `!!document.querySelector('[role="alertdialog"]')` changes using a body subtree MutationObserver; separately record window focus and document visibilitychange. Response listener records the actual ViewerCapabilities request and raw response. No focus/visibility events occurred during the instrumented disappearing dialogs. Requests use the signed-in browser's normal GraphQL session; no mock permissions were injected.

```json
{
  "initialFreshReloadCheck": {
    "at": "2026-10-03T04:53:02.467Z",
    "opened": "2026-10-03T04:52:14.959Z",
    "open": 0
  },
  "boundDialogEvidence": {
    "at": "2026-10-03T04:55:04.051Z",
    "open": 0,
    "events": [
      {
        "at": "2026-10-03T04:53:28.107Z",
        "type": "dialog",
        "open": false
      },
      {
        "at": "2026-10-03T04:53:28.351Z",
        "type": "dialog",
        "open": true
      },
      {
        "at": "2026-10-03T04:53:32.263Z",
        "type": "dialog",
        "open": false
      },
      {
        "at": "2026-10-03T04:54:17.902Z",
        "type": "dialog",
        "open": true
      },
      {
        "at": "2026-10-03T04:54:32.293Z",
        "type": "dialog",
        "open": false
      }
    ],
    "responses": [
      {
        "at": "2026-10-03T04:53:32.505Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:54:02.081Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:54:32.119Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:55:02.377Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:55:32.833Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:56:02.173Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:56:32.091Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:57:02.119Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      },
      {
        "at": "2026-10-03T04:57:32.105Z",
        "request": {
          "operationName": "ViewerCapabilities",
          "variables": {
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "fresh": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
        },
        "status": 200,
        "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
      }
    ]
  },
  "finalDialogEvents": [
    {
      "at": "2026-10-03T04:53:28.107Z",
      "type": "dialog",
      "open": false
    },
    {
      "at": "2026-10-03T04:53:28.351Z",
      "type": "dialog",
      "open": true
    },
    {
      "at": "2026-10-03T04:53:32.263Z",
      "type": "dialog",
      "open": false
    },
    {
      "at": "2026-10-03T04:54:17.902Z",
      "type": "dialog",
      "open": true
    },
    {
      "at": "2026-10-03T04:54:32.293Z",
      "type": "dialog",
      "open": false
    },
    {
      "at": "2026-10-03T04:55:48.624Z",
      "type": "dialog",
      "open": true
    }
  ],
  "permissionResponses": [
    {
      "at": "2026-10-03T04:53:32.505Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:54:02.081Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:54:32.119Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:55:02.377Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:55:32.833Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:56:02.173Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:56:32.091Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:57:02.119Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    },
    {
      "at": "2026-10-03T04:57:32.105Z",
      "request": {
        "operationName": "ViewerCapabilities",
        "variables": {
          "ownerId": "tea-d98210cbbpdc73dcrkvg",
          "fresh": false
        },
        "extensions": {
          "clientLibrary": {
            "name": "@apollo/client",
            "version": "4.1.3"
          }
        },
        "query": "query ViewerCapabilities($ownerId: String, $fresh: Boolean) {\n  viewerCapabilities(ownerId: $ownerId, fresh: $fresh) {\n    role\n    canView\n    canViewLogs\n    canOperate\n    canCreate\n    canViewSensitive\n    canManageKeys\n    canManage\n    canManageBilling\n    fresh\n    grants {\n      action\n      outcome\n      reason\n      __typename\n    }\n    __typename\n  }\n}"
      },
      "status": 200,
      "responseRaw": "{\"data\":{\"viewerCapabilities\":{\"__typename\":\"ViewerCapabilities\",\"canCreate\":true,\"canManage\":true,\"canManageBilling\":true,\"canManageKeys\":true,\"canOperate\":true,\"canView\":true,\"canViewLogs\":true,\"canViewSensitive\":true,\"fresh\":false,\"grants\":[{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_logs\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_operate\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_create\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_view_sensitive\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_keys\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage\",\"outcome\":\"allowed\",\"reason\":null},{\"__typename\":\"CapabilityGrant\",\"action\":\"can_manage_billing\",\"outcome\":\"allowed\",\"reason\":null}],\"role\":\"ADMIN\"}}}\n"
    }
  ],
  "unboundControl": {
    "at": "2026-10-03T04:55:47.486Z",
    "opened": "2026-10-03T04:55:04.269Z",
    "open": 1,
    "draft": "traefik/whoami:qa-r41-unsaved-draft"
  },
  "mutations": [
    {
      "at": "2026-10-03T04:45:05.607Z",
      "operations": ["CreateService"]
    },
    {
      "at": "2026-10-03T04:46:44.120Z",
      "operations": ["SetImage"]
    },
    {
      "at": "2026-10-03T04:47:12.313Z",
      "operations": ["TriggerDeploy"]
    },
    {
      "at": "2026-10-03T04:48:26.404Z",
      "operations": ["RollbackService"]
    },
    {
      "at": "2026-10-03T04:50:29.254Z",
      "operations": ["RestartServer"]
    },
    {
      "at": "2026-10-03T04:51:10.383Z",
      "operations": ["TriggerDeploy"]
    },
    {
      "at": "2026-10-03T04:57:14.263Z",
      "operations": ["DeleteService"]
    }
  ],
  "cleanup": {
    "url": "https://api.bex.co/v1/services/srv-db08gk8ehcmc739j10mg",
    "status": 404,
    "responseRaw": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
  }
}
```
