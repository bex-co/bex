# Cycle 11 — SSH remedies: two minor defects

Why: the remedy for shell access should explain the actual remaining blockers and make its destination usable, rather than promising Free SSH or retaining a modal overlay over the next page.

Live `qa-find-bugs`, 2026-10-08 UTC, user-directed w4 / `muse.env` loop. Workspace `bex` / `tea-d98210cbbpdc73dcrkvg`. Source coordinates below are pinned to checkout `c7ef05ae7`; the relevant dashboard helpers are unchanged from the initial `bbed7a446` checkout. This filing contains no product fixes.

## Fixture and observed journey

Owned Free web service `qa-20261008-c11-shell-72f4`, `srv-db3ldnchm7os73dpdc90`, image `docker.io/library/busybox:1.37`, port 3000, empty HTTP health-check path (TCP default). Initial Docker Command:

```sh
sh -c 'mkdir -p /tmp/qa-c11; printf "marker=qa-c11-shell-72f4\nport=%s\n" "$PORT" > /tmp/qa-c11/index.html; exec httpd -f -p "$PORT" -h /tmp/qa-c11'
```

The first deploy reached Live / Running, and browser and external curl returned HTTP 200. Shell and Connect explained the normal Free-plan restriction correctly. For the crash/cancel journey, Settings → Docker Command was changed to `sh -c 'printf "qa-c11-expected-exit42\n"; exit 42'`, then Save changes and its confirmation. The new deploy showed a correct exit-42 stall reason and repeated marker lines in its logs. Five public responses still served the original HTTP-200 marker. Cancel settled to Canceled and the service returned to Running `rev-1` with the bad saved command still unapplied. Events showed Canceled. The operator's normal 15-minute failure deadline was **not** awaited; no terminal crash-failure behavior is claimed.

The two defects below were then observed during suspension and remedy navigation. Resume served the original healthy marker again, not the canceled crash command. `undeployedChanges: true` in the captures is explained by that retained saved command and is not evidence of a failed resume. A pristine-history fixture was not tested live; the essential combined gate inputs and navigation were reproduced after fresh loads and again at mobile width.

### 1. Suspended Free service says Resume enables SSH

- **Severity:** minor — misleading copy/action guidance; the correct backend access gate stays intact.
- **Repro:** with the owned fixture suspended, fresh-load `/services/<id>/shell`; read the unavailable card. Open Connect and read its SSH refusal. Both say “This service is suspended. Resume it to connect over SSH.” with Resume service. Follow the link, dismiss the separate menu bug if necessary, and activate Settings Resume. After Running settles, fresh-load Shell: it now says Free services do not include shell access and offers Change instance type. The same suspended copy reproduced at 1440×1000 and 390×844.
- **Expected / actual:** the combined state must name **both** suspension and the paid-plan restriction. Resume restores serving; it cannot grant native SSH or browser-shell eligibility to a Free instance. Current copy omits the remaining plan blocker and presents Resume as the way to connect.
- **API control:** the exact suspended and resumed GraphQL probes below return `plan: free` and empty `sshAddress` in both states, with only suspension/phase changing. Resume runtime control returns the healthy marker and HTTP 200. No ticket was requested, private key created, or paid action submitted.
- **Root cause:** `dashboard/src/features/services/lib/ssh-remedy.ts:34` returns the suspension-only reason before checking `service.plan === "free"` at line 40. `service-shell-page.tsx:36` / `:81` and `service-detail-header.tsx:435` / `:440` render the selected reason verbatim. The English promise is `features/services/locales/en.ts:74`; zh has the corresponding reason at `:73`. `features/services/lib/status.ts:25` / `:44` correctly converts the wire suspension enum to a boolean. This is not an enum/truthiness error.
- **Correct target/layer:** add a combined reason for the existing SSH-supported type family before the suspension-only branch. Keep a clearly described Resume lifecycle action if desired, and state that shell access still requires a paid instance type. Update en and zh together. Preserve the ordinary non-suspended Free plan remedy, paid suspension remedy, generic explanation and unsupported-type exclusion. Do not “fix” the text by granting Free shell access or inventing `sshAddress`.
- **Backend control:** `lego/backend/internal/apps/service.go:1218` rejects suspended or unknown/Free plans in `sshEligible`; `:1238` additionally requires a live Running runtime for actual sessions. `:1169` advertises an SSH address only for eligible services. Native resolution (`:1559`) and browser-shell mint (`:1628`) both call the readiness gate. `render.go:409` and `graphql.go:328` project the same SSH value. These are code-traced contracts; only GraphQL and the normal REST service read were probed live, not shell transports or MCP.
- **Blast radius:** exactly **two production `sshRemedy` calls**: Shell page and header refusal. The change is shared across those callers but allowlisted to the combined Free/suspended state within web/private/worker types. The caller tests currently cover ordinary Free and **paid** suspended separately (`service-detail-header.test.tsx:302`, `:318`, with `plan: starter`) and one Free Shell case (`services.$serviceId.shell.test.tsx:95`); none covers this intersection.
- **Adjacent classes:** unknown/null plan and unconfigured/unready paid services retain the existing generic explanation; unsupported static/cron retain no plan/resume SSH remedy. Authorization, 401/403/404/timeout behavior and existence disclosure do not change.
- **Unverified:** Chinese rendering, paid runtime/SSH/browser-shell execution, private/worker variants, unsupported types, aliases, pristine deployment history and MCP are not live passes. Verification belongs in t003/t004.
- **Render:** [SSH compatibility table](https://render.com/docs/ssh) excludes Free web services from native SSH and dashboard shell. It does not specify this exact remedy copy; no authenticated Render reproduction was performed.
- **Estimate:** 30m implementation; shared controls and closing work are separate tasks.

### 2. Connect remedy navigates but leaves the modal menu open

- **Severity:** minor — destination controls require a second manual dismissal; Escape is a working workaround. This is not an API mutation or failed page-load defect.
- **Repro A:** fresh-load the Running Free fixture's Shell page. Connect → Change instance type. URL becomes `/services/<id>/plan`. After five seconds at desktop (and 3.5 seconds on mobile), the picker exists in DOM, but Connect remains `data-state: open`, the main accessibility snapshot is empty, an ancestor is `aria-hidden: true`, and `body.style.pointerEvents` is `none`. Pressing Tab at desktop leaves focus on the remedy link. Escape closes the menu and exposes the picker.
- **Repro B:** suspend the fixture; fresh-load Shell. Connect → Resume service. URL becomes `/services/<id>/settings#suspend`; after 3.5 seconds at both widths, a non-disabled Resume button exists in DOM but has **zero** accessible-role matches while Connect remains open. Escape reveals that button and allows Resume. The earlier desktop capture also retained the menu well past a normal animation.
- **Controls:** the directly clicked **Shell-page** Change instance type and Resume service links lead to the same destinations with no menu, no body pointer lock, and accessible ready controls. These are the same remedy destinations; the consumer context makes the difference. A main-only `aria-hidden` attribute check would miss the hiding because Radix marks an ancestor. Captures below explicitly distinguish menu state, DOM presence and accessibility exposure.
- **Expected:** selecting a menu navigation remedy dismisses Connect and releases its modal focus/pointer restrictions. While destination data loads, its existing route skeleton/pending state remains available; once ready, normal destination controls are accessible. Keep the persistent service header and its navigation/loading behavior.
- **Root cause:** `service-detail-header.tsx:353` creates an uncontrolled DropdownMenu. `SshUnavailable` at `:442` puts a **plain TanStack Link** in that modal content, without a DropdownMenuItem or close handler. `ServiceDetailLayout` (`service-detail-layout.tsx:135`, Outlet under it) retains the header on service-tab navigation, so routing does not unmount the open menu.
- **Actual dependency path:** direct dropdown package **@radix-ui/react-dropdown-menu 2.1.16** resolves its **nested @radix-ui/react-menu 2.1.16**. The separately hoisted 2.1.14 menu belongs to another dependency and is not the implementation used by Connect. The resolved packages/paths were checked through Node resolution and yarn.lock, recorded locally in `.playwright-mcp/qa-c11-resolved-radix.json`. Opened actual files: dropdown `dist/index.js:96` defaults modal to true; nested menu `dist/index.js:197` calls `hideOthers`, `:204-205` traps focus and disables outside pointers while open, `:372` prevents Tab, and `:441-450` closes the Root on normal MenuItem selection. Dismissable layer **1.1.11** `dist/index.js:108-120` sets and restores body pointer events. The app wrapper `common/components/ui/dropdown-menu.tsx:7` forwards Root props and its item wrapper reuses the primitive.
- **Navigation consumer:** pinned **@tanstack/react-router 1.170.25** `dist/esm/link.js:274` performs client navigation, and `:346` composes supplied onClick before its navigation handler. It does not know about the Radix menu. The existing New Service menu (`new-service-menu-items.tsx:30`) already composes DropdownMenuItem asChild with Link. This is app composition, not a Radix defect.
- **Fix:** use existing item-selection semantics for the remedy Link (e.g. DropdownMenuItem asChild), or equally scoped explicit close behavior, verifying actual Link event composition and the hash destination. Keep modal protections on ordinary open menus; do not globally set modal false or remount the service layout. A pathname-only route-change effect is insufficient for same-destination selections (not live-probed; verify in t003).
- **Blast radius:** one `SshUnavailable` mount, one `ServiceConnectButton` mount, one shared-header mount in ServiceDetailLayout; the layout serves **two route families**, /services and /static. Static hides Connect. The two observed action targets share the one plain-Link template. The paid no-key branch has two further navigation templates: Add SSH public key through AddSshKeyCta and Open a browser terminal (`service-detail-header.tsx:391`, `:402`). Those are **code-traced, not live reproduced**. AddSshKeyCta itself has **three production mounts**: service header, Shell page, and agent-session header; an app-wide CTA change must not make standalone/agent callers inherit menu semantics. t003 independently traces/tests them and scopes any necessary closure to Connect.
- **Adjacent classes:** unsupported type/generic refusal has no remedy link and needs no invented action. Existing copy buttons, eligible SSH/key gates, transient data/loading errors, 401/403/404 and protected lifecycle confirmation keep their semantics. Closing on selection must not wait for a successful data response or change permission/error classification.
- **Unverified:** paid no-key CTA navigation, same-destination and modifier-click behavior, agent-session caller, Chinese UI, aliases and datastore siblings were not live-tested. They are verification work, not evidence that every dropdown or CTA is broken.
- **Render:** documentation describes a Connect SSH surface but not its exact menu dismissal behavior; no authenticated Render UI test. The observed inaccessible destination is an independent bex usability defect.
- **Estimate:** 40m implementation; shared-caller verification 30m.

## Dedupe and the original remedy guarantee

Searched open, blocked and done items across workstreams for Connect remedy navigation, menus staying open/dismissal/focus traps, combined Free/suspended SSH, `sshRemedy`, and its locale symbols. Scanned open milestone titles across all workstreams and checked recent dashboard/lego history plus targeted `git log -S` for both `SshUnavailable` and `sshUnavailableSuspended`. The symbols still trace to `2fbcbf2ac`; the subsequent pull to `c7ef05ae7` changed unrelated source-project refresh, environment rollout/artifact reuse and secret-version cleanup, not these helpers. No open item or later landed fix covers the two new cases. `.pm/DO_NOT_DO.md` has no exclusion for truthful guidance or menu composition on the already-approved running-instance shell surface. No new Free or ephemeral/cron shell capability is proposed.

[w4/done/143](../done/143.md) is the relevant precedent, not a duplicate. Its original problem was hover-only refusal and missing relevant links. Its entire Done guarantee maps as follows:

1. **Visible reasons in Connect:** still observed for both Free and suspended states; keep this. **Suspended Resume, Free plan remedy, generic and static/cron exclusion:** the two ordinary actions exist; the new gap is the overlapping Free+suspended inputs and link selection. Paid suspension, generic and unsupported states were inspected in source/tests, not live; t003 must verify them.
2. **Same reason/action in Shell rather than unconditional key management:** observed for both states; keep it. **Eligible services keep key management:** inspected in the branch, not exercised live because paid creation is prohibited; t003 covers it locally.
3. **en + zh and green checks:** both locale entries exist, but Chinese UI and new implementation checks have not been run for this filing. t001 updates both, t006 runs appropriate checks. Existing tests assert separately chosen copy/hrefs and do not establish dismissal or the combined inputs.

Thus this is a follow-up for uncovered behavior introduced with the remedies, not an assertion that the earlier tooltip fix regressed. The incorrect in-flight undeployed-changes notice observed during the crash journey is already **w4/201** and is not filed again. Startup StartError diagnostics are **w4/blocked/m177**; this run's genuine exit-42 diagnosis was correct and is not folded into that finding.

## Scope, aliases and sibling family

`sshRemedy` supports web_service · private_service · background_worker. All consume the /services layout; /web, /pserv and /worker legacy routes redirect there. /cron also redirects into /services but receives the generic unsupported explanation; /static renders the shared header with Connect omitted. Postgres and Key Value use their own resource-detail surfaces and do not call this helper. The generic alias map also accepts static service ids under its legacy segment; no new alias API is needed. These are source-traced relationships, not live passes of every resource family. Keep shared logic scoped to the observed remedy decision/Connect context and verify the siblings in t003.

## Durable API evidence

The blocks below preserve exact POST bodies and **complete matching JSON responses** from direct page.request calls to `https://api.bex.co/graphql`, plus the observed runtime response where present. They do not contain cookie headers or shell tickets. Replace the deleted fixture id with a new owned fixture for a replay. UI text accompanying the API records is a selected accessibility tail, explicitly separated from the complete API response.

### Suspended Free, desktop

```json
{
  "url": "https://api.bex.co/graphql",
  "method": "POST",
  "at": "2026-10-08T08:46:20.568Z",
  "request": {
    "query": "{server(id:\"srv-db3ldnchm7os73dpdc90\"){id type plan phase revision suspended sshAddress undeployedChanges}}"
  },
  "httpStatus": 200,
  "response": {
    "data": {
      "server": {
        "id": "srv-db3ldnchm7os73dpdc90",
        "phase": "Hibernated",
        "plan": "free",
        "revision": "rev-1",
        "sshAddress": "",
        "suspended": "suspended",
        "type": "web_service",
        "undeployedChanges": true
      }
    }
  },
  "runtime": {
    "status": 503,
    "body": "This service is suspended.\n"
  },
  "selectedShellAccessibility": "  - heading \"Shell\" [level=2]\n  - paragraph: Open a shell on a running service instance from your terminal.\n  - text: Shell access isn't available This service is suspended. Resume it to connect over SSH.\n  - link \"Resume service\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/settings#suspend"
}
```

### Resumed Free, desktop

```json
{
  "url": "https://api.bex.co/graphql",
  "method": "POST",
  "at": "2026-10-08T08:49:58.672Z",
  "request": {
    "query": "{server(id:\"srv-db3ldnchm7os73dpdc90\"){id type plan phase revision suspended sshAddress undeployedChanges}deploys(serviceId:\"srv-db3ldnchm7os73dpdc90\"){id status trigger}}"
  },
  "httpStatus": 200,
  "response": {
    "data": {
      "deploys": [
        {
          "id": "dep-db3leq4hm7os73dpdca0",
          "status": "canceled",
          "trigger": "config_change"
        },
        {
          "id": "dep-db3ldnchm7os73dpdc9g",
          "status": "live",
          "trigger": "create"
        }
      ],
      "server": {
        "id": "srv-db3ldnchm7os73dpdc90",
        "phase": "Running",
        "plan": "free",
        "revision": "rev-1",
        "sshAddress": "",
        "suspended": "not_suspended",
        "type": "web_service",
        "undeployedChanges": true
      }
    }
  },
  "runtime": {
    "status": 200,
    "body": "marker=qa-c11-shell-72f4\nport=3000\n"
  },
  "selectedShellAccessibility": "  - heading \"Shell\" [level=2]\n  - paragraph: Open a shell on a running service instance from your terminal.\n  - text: Shell access isn't available SSH needs a paid instance type. Free services don't include shell access.\n  - link \"Change instance type\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/plan"
}
```

### Suspended Free, mobile

```json
{
  "url": "https://api.bex.co/graphql",
  "method": "POST",
  "at": "2026-10-08T08:53:11.936Z",
  "request": {
    "query": "{server(id:\"srv-db3ldnchm7os73dpdc90\"){id type plan phase revision suspended sshAddress undeployedChanges}}"
  },
  "httpStatus": 200,
  "response": {
    "data": {
      "server": {
        "id": "srv-db3ldnchm7os73dpdc90",
        "phase": "Hibernated",
        "plan": "free",
        "revision": "rev-1",
        "sshAddress": "",
        "suspended": "suspended",
        "type": "web_service",
        "undeployedChanges": true
      }
    }
  },
  "viewport": {
    "width": 390,
    "height": 844
  },
  "selectedShellAccessibility": "  - heading \"Shell\" [level=2]\n  - paragraph: Open a shell on a running service instance from your terminal.\n  - text: Shell access isn't available This service is suspended. Resume it to connect over SSH.\n  - link \"Resume service\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/settings#suspend"
}
```

## Durable navigation observations

These are **selected DOM/accessibility projections**, not raw HTTP responses or whole-page DOM dumps. The recorded checks make the observation repeatable: click the actual named link, wait for the URL, wait longer than the normal menu animation, then read role/menu state, the main accessibility tree and DOM availability. They include the independently exercised direct-link controls; screenshots are supplemental only.

### Plan remedy, desktop; Tab pressed after waiting

```json
{
  "clickAt": "2026-10-08T08:51:16.445Z",
  "observedAt": "2026-10-08T08:51:21.713Z",
  "url": "https://dashboard.bex.co/services/srv-db3ldnchm7os73dpdc90/plan",
  "body": "- region \"Notifications alt+T\"\n- menu \"Connect\":\n  - text: Internal\n  - code: qa-20261008-c11-shell-72f4:3000\n  - button \"Copy internal address\"\n  - text: SSH\n  - paragraph: SSH isn't available\n  - paragraph: SSH needs a paid instance type. Free services don't include shell access.\n  - link \"Change instance type\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/plan",
  "main": "",
  "dom": {
    "menus": [
      {
        "state": "open",
        "text": "Internalqa-20261008-c11-shell-72f4:3000SSHSSH isn't availableSSH needs a paid instance type. Free services don't include shell access.Change instance type"
      }
    ],
    "bodyPointerEvents": "none",
    "ariaHiddenAncestors": [
      {
        "tag": "DIV",
        "value": "true"
      }
    ],
    "planRadiogroupInDOM": true,
    "focused": {
      "tag": "A",
      "role": null,
      "text": "Change instance type"
    }
  }
}
```

### Plan remedy, mobile

```json
{
  "clickAt": "2026-10-08T08:51:48.852Z",
  "observedAt": "2026-10-08T08:51:52.605Z",
  "url": "https://dashboard.bex.co/services/srv-db3ldnchm7os73dpdc90/plan",
  "viewport": {
    "width": 390,
    "height": 844
  },
  "body": "- region \"Notifications alt+T\"\n- menu \"Connect\":\n  - text: Internal\n  - code: qa-20261008-c11-shell-72f4:3000\n  - button \"Copy internal address\"\n  - text: SSH\n  - paragraph: SSH isn't available\n  - paragraph: SSH needs a paid instance type. Free services don't include shell access.\n  - link \"Change instance type\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/plan",
  "main": "",
  "dom": {
    "menuState": "open",
    "bodyPointerEvents": "none",
    "planRadiogroupInDOM": true
  }
}
```

### Resume remedy, desktop

```json
{
  "clickAt": "2026-10-08T08:55:03.703Z",
  "observedAt": "2026-10-08T08:55:07.467Z",
  "url": "https://dashboard.bex.co/services/srv-db3ldnchm7os73dpdc90/settings#suspend",
  "viewport": {
    "width": 1440,
    "height": 1000
  },
  "body": "- region \"Notifications alt+T\"\n- menu \"Connect\":\n  - text: Internal\n  - code: qa-20261008-c11-shell-72f4:3000\n  - button \"Copy internal address\"\n  - text: SSH\n  - paragraph: SSH isn't available\n  - paragraph: This service is suspended. Resume it to connect over SSH.\n  - link \"Resume service\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/settings#suspend",
  "main": "",
  "resumeAccessible": 0,
  "dom": {
    "menuState": "open",
    "bodyPointerEvents": "none",
    "resumeButtonInDOM": true
  }
}
```

### Resume remedy, mobile and direct-link control

```json
{
  "control": {
    "at": "2026-10-08T08:53:49.754Z",
    "url": "https://dashboard.bex.co/services/srv-db3ldnchm7os73dpdc90/settings#suspend",
    "menuCount": 0,
    "resumeAccessible": 1,
    "bodyPointerEvents": ""
  },
  "clickAt": "2026-10-08T08:53:50.765Z",
  "observedAt": "2026-10-08T08:53:54.515Z",
  "url": "https://dashboard.bex.co/services/srv-db3ldnchm7os73dpdc90/settings#suspend",
  "viewport": {
    "width": 390,
    "height": 844
  },
  "body": "- region \"Notifications alt+T\"\n- menu \"Connect\":\n  - text: Internal\n  - code: qa-20261008-c11-shell-72f4:3000\n  - button \"Copy internal address\"\n  - text: SSH\n  - paragraph: SSH isn't available\n  - paragraph: This service is suspended. Resume it to connect over SSH.\n  - link \"Resume service\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/settings#suspend",
  "main": "",
  "resumeAccessible": 0,
  "dom": {
    "menuState": "open",
    "bodyPointerEvents": "none",
    "resumeButtonInDOM": true
  }
}
```

### Direct Shell-page plan remedy control

```json
{
  "at": "2026-10-08T08:50:53.955Z",
  "url": "https://dashboard.bex.co/services/srv-db3ldnchm7os73dpdc90/plan",
  "ui": "- main:\n  - navigation \"Breadcrumbs\":\n    - link \"Projects\":\n      - /url: /\n    - button \"qa-20261008-c11-shell-72f4\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261008-c11-shell-72f4\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Canceled\"':\n    - /url: /services/srv-db3ldnchm7os73dpdc90/deploys/dep-db3leq4hm7os73dpdca0\n    - text: Latest deploy Canceled\n  - link \"Free\":\n    - /url: /services/srv-db3ldnchm7os73dpdc90/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db3ldnchm7os73dpdc90\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261008-c11-shell-72f4.onbex.co\":\n    - /url: https://qa-20261008-c11-shell-72f4.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261008-c11-shell-72f4\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 9m\n  - status: Saved changes aren't live yet. Use a standard deploy to apply them.\n  - text: Pick an Instance Type\n  - radiogroup \"Pick an Instance Type\":\n    - text: Free\n    - radio \"Free 512 MB (RAM) 0.1 CPU\" [checked]\n    - text: Paid\n    - radio \"Starter 512 MB (RAM) 0.5 CPU\"\n    - radio \"Standard 2 GB (RAM) 1 CPU\"\n    - radio \"Pro 4 GB (RAM) 2 CPU\"\n    - radio \"Pro Plus 8 GB (RAM) 4 CPU\"\n    - radio \"Pro Max 16 GB (RAM) 4 CPU\"\n    - radio \"Pro Ultra 32 GB (RAM) 8 CPU\"\n  - button \"Cancel\"\n  - button \"Save Changes\" [disabled]",
  "menuCount": 0,
  "bodyPointerEvents": ""
}
```

## Local evidence and cleanup

Checked every referenced path on disk. Viewed the suspended desktop/mobile and menu mobile screenshots; the screenshot of the still-open desktop Resume menu is also available. All screenshots are ignored local supplements; the durable evidence above survives the filing commit.

- `.playwright-mcp/qa-shell-c11-suspended-free-desktop.png`
- `.playwright-mcp/qa-shell-c11-suspended-free-mobile.png`
- `.playwright-mcp/qa-shell-c11-remedy-menu-stays-open.png`
- `.playwright-mcp/qa-shell-c11-plan-menu-open.png`
- `.playwright-mcp/qa-shell-c11-plan-menu-mobile.png`
- `.playwright-mcp/qa-shell-c11-resume-menu-mobile.png`
- `.playwright-mcp/qa-c11-suspended-shell-probe.json` · `qa-c11-resumed-shell-probe.json` · `qa-c11-suspended-shell-mobile.json`
- `.playwright-mcp/qa-c11-plan-menu-probe.json` · `qa-c11-plan-menu-mobile.json` · `qa-c11-resume-menu-desktop.json` · `qa-c11-resume-menu-mobile.json` · `qa-c11-plan-direct-control.json`
- `.playwright-mcp/qa-c11-crash-probe.json` · `qa-c11-failing-command-mutation.json` · `qa-c11-cancel-mutation.json` · `qa-c11-suspend-mutation.json` · `qa-c11-resume-mutation.json`
- `.playwright-mcp/qa-c11-remedy-console.txt` (zero console errors before logout) · `qa-c11-remedy-network.txt` (API calls returned 200; canceled navigation requests and the deliberate logout's later 401 are not product defects)
- `.playwright-mcp/qa-c11-resolved-radix.json` · `qa-c11-cleanup-probe.json` · `qa-loop-c11-ledger.json`

The owned service was deleted through its confirmed Delete Service dialog. Its REST read and public URL returned 404, and a fresh Overview had no owned service link. Read-only production cluster checks found zero remaining App, exact primary/slug Services, Ingress or owned app pods. The foreign baseline Postgres and environment group still returned 200. No paid resource, SSH key, project or credential was created. The matching cycle-owned Kratos session was revoked; browser and private-jar whoami returned 401 and the owned jars were removed without bulk logout.

Skipped: private-service/worker creation and paid shell/scaling/maintenance actions because their offered instance types require payment. Custom-domain attachment lacked a controlled domain. No unrelated resource was changed. This milestone ships only the researched findings; deployment and live fix acceptance belong to its tasks.
