# Plan picker omits Free/Paid regions while loading and expands on settle

**Severity:** minor. **Source:** functional $qa-find-bugs w4 pass a77, 2026-10-10 UTC. **Status:** open. This is a visible layout shift; no plan selection, purchase, rollout, or data loss was reproduced.

## Reproduction and target

1. Create an owned Free image Web service using BusyBox 1.36.1, port 3000, and a tiny httpd marker. This pass used `qa-20261010-a77-layout`, `srv-db57rrcb8ulc73efu900`, Live deploy `dep-db57rrcb8ulc73efu90g`, rev-1. Its public URL returned HTTP 200 with `qa-a77-layout-marker\n`.
2. Fresh-load `/services/<owned-id>/plan` at 1440×1000. Hold the original API HTTP response containing InstanceTypes for four seconds, without changing the payload.
3. The loading content is one six-card grid. Once settled, the same card contains a separate Free region (one card) and Paid region (six cards). The outer card height changes 355→551 pixels and its content changes 277→473 pixels: **196 pixels of expansion**.
4. Repeat fresh-load at 390×844. The card changes 771→1015 pixels and content changes 693→937 pixels: **244 pixels of expansion**. Document width stays 390. Parent card top changes three pixels as other bootstrap data settle; that movement is excluded from the helper claim.
5. Free remains checked and Save Changes remains disabled. Never select a paid tier or submit a plan change in this replay.

**Target:** for this Free Web journey, the loading state reserves the separate Free and Paid regions, the ready card dimensions at the same breakpoint, and the footer action slot. Settling changes placeholders to the current catalog without the observed 196/244-pixel expansion. Preserve the current selection and inactive Save action. This is a structure/height requirement, not a rule that every variable-length list must have an exact skeleton count.

## Harness boundaries and durable evidence

Apollo 4.1.3 batches bootstrap reads. The delayed HTTP array contained **Services, Projects, BillingReadiness, Workspaces, InstanceTypes, Deploys**. It was not a catalog-only network delay. This is why the finding uses within-card geometry, rather than attributing the entire page's movement to InstanceTypes. The actual unchanged response was fulfilled successfully with HTTP 200.

The independent single-operation control below establishes the complete seven-tier catalog without treating a projected batch entry as an independent receipt:

```json
{
  "at": "2026-10-10T18:12:45.862Z",
  "request": {
    "operationName": "InstanceTypes",
    "variables": {},
    "extensions": {
      "clientLibrary": {
        "name": "@apollo/client",
        "version": "4.1.3"
      }
    },
    "query": "query InstanceTypes {\n  instanceTypes {\n    id\n    name\n    cpu\n    memory\n    monthlyUsd\n    __typename\n  }\n}"
  },
  "status": 200,
  "response": {
    "data": {
      "instanceTypes": [
        {
          "__typename": "InstanceType",
          "cpu": "100m",
          "id": "free",
          "memory": "512Mi",
          "monthlyUsd": "0.00",
          "name": "Free"
        },
        {
          "__typename": "InstanceType",
          "cpu": "500m",
          "id": "starter",
          "memory": "512Mi",
          "monthlyUsd": "4.90",
          "name": "Starter"
        },
        {
          "__typename": "InstanceType",
          "cpu": "1",
          "id": "standard",
          "memory": "2Gi",
          "monthlyUsd": "17.50",
          "name": "Standard"
        },
        {
          "__typename": "InstanceType",
          "cpu": "2",
          "id": "pro",
          "memory": "4Gi",
          "monthlyUsd": "59.50",
          "name": "Pro"
        },
        {
          "__typename": "InstanceType",
          "cpu": "4",
          "id": "pro_plus",
          "memory": "8Gi",
          "monthlyUsd": "122.50",
          "name": "Pro Plus"
        },
        {
          "__typename": "InstanceType",
          "cpu": "4",
          "id": "pro_max",
          "memory": "16Gi",
          "monthlyUsd": "157.50",
          "name": "Pro Max"
        },
        {
          "__typename": "InstanceType",
          "cpu": "8",
          "id": "pro_ultra",
          "memory": "32Gi",
          "monthlyUsd": "315.00",
          "name": "Pro Ultra"
        }
      ]
    }
  }
}
```

These are the measured pending/ready DOM captures, including the actual checked and disabled controls:

```json
{
  "desktop": {
    "pending": {
      "at": "2026-10-10T18:07:20.606Z",
      "intercepts": [
        {
          "startedAt": "2026-10-10T18:07:19.762Z",
          "operation": "InstanceTypes",
          "fulfilled": false
        }
      ],
      "ui": "- main:\n  - navigation \"Breadcrumbs\":\n    - link \"Projects\":\n      - /url: /\n    - button \"qa-20261010-a77-layout\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261010-a77-layout\" [level=1]\n  - text: Service Running Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db57rrcb8ulc73efu900\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261010-a77-layout.onbex.co\":\n    - /url: https://qa-20261010-a77-layout.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261010-a77-layout\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 3m\n  - text: Pick an Instance Type\n  - button \"Cancel\"\n  - button \"Save Changes\" [disabled]",
      "boxes": [
        {
          "slot": "card",
          "box": {
            "x": 400,
            "y": 251,
            "w": 896,
            "h": 355
          }
        },
        {
          "slot": "card-content",
          "box": {
            "x": 401,
            "y": 308,
            "w": 894,
            "h": 277
          }
        }
      ],
      "skeletonCount": 7
    },
    "ready": {
      "at": "2026-10-10T18:07:24.991Z",
      "ui": "- main:\n  - navigation \"Breadcrumbs\":\n    - link \"Projects\":\n      - /url: /\n    - button \"qa-20261010-a77-layout\"\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261010-a77-layout\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Live\"':\n    - /url: /services/srv-db57rrcb8ulc73efu900/deploys/dep-db57rrcb8ulc73efu90g\n    - text: Latest deploy Live\n  - link \"Free\":\n    - /url: /services/srv-db57rrcb8ulc73efu900/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db57rrcb8ulc73efu900\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261010-a77-layout.onbex.co\":\n    - /url: https://qa-20261010-a77-layout.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261010-a77-layout\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 3m\n  - text: Pick an Instance Type\n  - radiogroup \"Pick an Instance Type\":\n    - text: Free\n    - radio \"Free 512 MB (RAM) 0.1 CPU\" [checked]\n    - text: Paid\n    - radio \"Starter 512 MB (RAM) 0.5 CPU\"\n    - radio \"Standard 2 GB (RAM) 1 CPU\"\n    - radio \"Pro 4 GB (RAM) 2 CPU\"\n    - radio \"Pro Plus 8 GB (RAM) 4 CPU\"\n    - radio \"Pro Max 16 GB (RAM) 4 CPU\"\n    - radio \"Pro Ultra 32 GB (RAM) 8 CPU\"\n  - button \"Cancel\"\n  - button \"Save Changes\" [disabled]",
      "boxes": [
        {
          "slot": "card",
          "box": {
            "x": 400,
            "y": 251,
            "w": 896,
            "h": 551
          }
        },
        {
          "slot": "card-content",
          "box": {
            "x": 401,
            "y": 308,
            "w": 894,
            "h": 473
          }
        },
        {
          "slot": "radiogroup",
          "box": {
            "x": 421,
            "y": 308,
            "w": 854,
            "h": 396
          }
        }
      ],
      "freeChecked": true
    },
    "delayMs": 4000
  },
  "mobile": {
    "pending": {
      "at": "2026-10-10T18:10:26.168Z",
      "intercepts": [
        {
          "operation": "InstanceTypes",
          "fulfilled": false
        }
      ],
      "ui": "- main:\n  - button \"Toggle Sidebar\"\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261010-a77-layout\"\n  - button \"Search\"\n  - button \"New\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261010-a77-layout\" [level=1]\n  - text: Service Running Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db57rrcb8ulc73efu900\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261010-a77-layout.onbex.co\":\n    - /url: https://qa-20261010-a77-layout.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261010-a77-layout\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 6m\n  - text: Pick an Instance Type\n  - button \"Cancel\"\n  - button \"Save Changes\" [disabled]",
      "boxes": [
        {
          "slot": "card",
          "box": {
            "x": 16,
            "y": 333,
            "w": 358,
            "h": 771
          }
        },
        {
          "slot": "card-content",
          "box": {
            "x": 17,
            "y": 390,
            "w": 356,
            "h": 693
          }
        }
      ],
      "geometry": {
        "viewport": 390,
        "documentWidth": 390
      },
      "planSkeletonCount": 6
    },
    "ready": {
      "at": "2026-10-10T18:10:30.546Z",
      "ui": "- main:\n  - button \"Toggle Sidebar\"\n  - navigation \"Breadcrumbs\":\n    - button \"qa-20261010-a77-layout\"\n  - button \"Search\"\n  - button \"New\"\n  - button \"P\"\n  - text: Web Service\n  - heading \"qa-20261010-a77-layout\" [level=1]\n  - text: Service Running\n  - 'link \"Latest deploy: Live\"':\n    - /url: /services/srv-db57rrcb8ulc73efu900/deploys/dep-db57rrcb8ulc73efu90g\n    - text: Latest deploy Live\n  - link \"Free\":\n    - /url: /services/srv-db57rrcb8ulc73efu900/plan\n  - text: Runtime image\n  - button \"Connect\"\n  - button \"Manual Deploy\"\n  - text: \"Service ID: srv-db57rrcb8ulc73efu900\"\n  - button \"Copy service ID\"\n  - link \"https://qa-20261010-a77-layout.onbex.co\":\n    - /url: https://qa-20261010-a77-layout.onbex.co\n  - button \"Copy service URL\"\n  - term: Slug\n  - definition: qa-20261010-a77-layout\n  - term: Instances\n  - definition: \"1\"\n  - term: Revision\n  - definition: rev-1\n  - term: Created\n  - definition:\n    - time: 6m\n  - text: Pick an Instance Type\n  - radiogroup \"Pick an Instance Type\":\n    - text: Free\n    - radio \"Free 512 MB (RAM) 0.1 CPU\" [checked]\n    - text: Paid\n    - radio \"Starter 512 MB (RAM) 0.5 CPU\"\n    - radio \"Standard 2 GB (RAM) 1 CPU\"\n    - radio \"Pro 4 GB (RAM) 2 CPU\"\n    - radio \"Pro Plus 8 GB (RAM) 4 CPU\"\n    - radio \"Pro Max 16 GB (RAM) 4 CPU\"\n    - radio \"Pro Ultra 32 GB (RAM) 8 CPU\"\n  - button \"Cancel\"\n  - button \"Save Changes\" [disabled]",
      "boxes": [
        {
          "slot": "card",
          "box": {
            "x": 16,
            "y": 336,
            "w": 358,
            "h": 1015
          }
        },
        {
          "slot": "card-content",
          "box": {
            "x": 17,
            "y": 393,
            "w": 356,
            "h": 937
          }
        },
        {
          "slot": "radiogroup",
          "box": {
            "x": 37,
            "y": 393,
            "w": 316,
            "h": 860
          }
        }
      ],
      "geometry": {
        "viewport": 390,
        "documentWidth": 390
      },
      "freeChecked": true,
      "saveDisabled": true
    },
    "delayMs": 4000
  },
  "batchOperationNames": [
    "Services",
    "Projects",
    "BillingReadiness",
    "Workspaces",
    "InstanceTypes",
    "Deploys"
  ],
  "interceptStatus": 200,
  "originalResponseUnchanged": true
}
```

Reusable delay and measurement recipe (equivalent replay, not a saved raw script from the original captures):

```js
// In a signed-in Playwright page. Replace serviceId with a new owned Free Web fixture.
const serviceId = "srv-db57rrcb8ulc73efu900"; // this fixture has been deleted
await page.setViewportSize({ width: 1440, height: 1000 }); // repeat at 390 x 844
let fulfilled = false;
const handler = async (route) => {
  const raw = route.request().postData() ?? "";
  if (
    route.request().method() !== "POST" ||
    !raw.includes('"operationName":"InstanceTypes"')
  ) {
    return route.continue();
  }
  const response = await route.fetch();
  await new Promise((resolve) => setTimeout(resolve, 4000));
  await route.fulfill({ response }); // original upstream bytes, status and headers
  fulfilled = true;
};
const measure = async () =>
  page.evaluate(() => {
    const title = [
      ...document.querySelectorAll('[data-slot="card-title"]'),
    ].find((el) => el.textContent === "Pick an Instance Type");
    const card = title?.closest('[data-slot="card"]');
    const content = card?.querySelector('[data-slot="card-content"]');
    const box = (el) => {
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return { x: r.x, y: r.y, w: r.width, h: r.height };
    };
    return {
      card: box(card),
      content: box(content),
      skeletons: content?.querySelectorAll('[data-slot="skeleton"]').length,
      radioGroup: box(content?.querySelector('[role="radiogroup"]')),
      viewport: innerWidth,
      documentWidth: document.documentElement.scrollWidth,
    };
  });
await page.route("https://api.bex.co/graphql", handler);
try {
  await page.goto("https://dashboard.bex.co/services/" + serviceId + "/plan");
  await page.getByText("Pick an Instance Type", { exact: true }).waitFor();
  const pending = await measure();
  // Check the response is still held; otherwise retry the capture.
  if (fulfilled) throw new Error("Pending capture missed the held response");
  await page
    .getByRole("radio", { name: "Free 512 MB (RAM) 0.1 CPU", exact: true })
    .waitFor({ timeout: 15000 });
  const ready = await measure();
  console.log({ pending, ready }); // measurements only
} finally {
  // Let the active response finish before removing this handler.
  await page.unroute("https://api.bex.co/graphql", handler);
}
```

Viewed, existing local screenshots:

- `.playwright-mcp/qa-plan-a77-desktop-catalog-pending.png`
- `.playwright-mcp/qa-plan-a77-desktop-catalog-ready.png`
- `.playwright-mcp/qa-plan-a77-mobile-catalog-pending.png`
- `.playwright-mcp/qa-plan-a77-mobile-catalog-ready.png`

The mobile images cover the viewport, despite the capture's fullPage option; the offscreen card extent is proved by DOM bounds. The earlier `qa-plan-a77-desktop-pending.png` belongs to an inconclusive handler attempt and is excluded. That attempt's “Route is already handled” error was harness noise. A separate first control queried a nonexistent GraphQL field; the corrected control returned the owned service as free/web_service/image. Neither is a product finding.

## Root cause and concrete fix

- `dashboard/src/common/components/detail-skeletons.tsx:209–218` renders six `h-24` skeletons in one responsive grid, without group-heading/spacing slots.
- `dashboard/src/features/services/components/instance-type-picker.tsx:87–149` puts that helper in the same outer Card/Content/footer as the ready picker. The ready branch instead has `space-y-4` group stacking and separate Free/Paid CardGroups.
- `instance-type-picker.tsx:167–185` gives each group its heading, spacing and responsive grid. Actual cards at :186–216 have padding and three text rows; their observed height also exceeds the helper's fixed 96 pixels.
- `dashboard/src/common/components/ui/card.tsx:5–19,77–87` uses normal flex layout and padding with no fixed card height; `ui/skeleton.tsx:3–15` passes the caller's classes to a div. No library-imposed minimum reserves the missing region.
- `dashboard/src/common/apollo/factory.client.ts:29–45` uses BatchHttpLink with batchInterval 10 and batchMax 32. The installed Apollo 4.1.3 `link/batch-http/BaseBatchHttpLink.js:59–83` serializes operation bodies as one array. That actual dependency code explains the capture's extra operations; batching is not the defect.
- The governing `dashboard/AGENTS.md:138` requires structural loading/ready parity, always-present major regions and stable heights at desktop and narrow-mobile widths.

Implement a grouped skeleton using the ready group's spacing, card dimensions and responsive columns. Pass the known service type from InstanceTypePicker and reuse `offeredInstanceTypes`'s existing paid-only policy; do not invent a second type policy. Web/cron allow Free; background workers and private services do not. Preserve currentPlan seeding, catalog errors, action gating, and all API inputs/outputs. Do not hardcode real tier prices or turn placeholders into selectable plans.

The initial server-read and parent/route-pending callers lack a service-type prop today. Trace the existing Server loader/cache context before changing them: use already resolved type when available, otherwise explicitly keep the undecided placeholder branch neutral and inert. Do not guess “Web” or expose a Free option for a paid-only type while unresolved. Determine and verify those branches in t002; they were not isolated in this live capture. Do not claim full route-preloader repair from the catalog-state fix alone.

## Shared callers, aliases and verification work

An exhaustive production TSX search found **three calls to PlanPickerGridSkeleton**:

1. InstanceTypePicker :93 — catalog loading with a serviceType prop.
2. `routes/services.$serviceId.plan.tsx:28–45` — initial service read.
3. `common/components/route-skeletons.tsx:1865–1883` — ServicePlanSkeleton.

ServicePlanSkeleton has **two production references that render it**: the plan route's pendingComponent and ServiceRouteContentSkeleton's plan case (:2311). The latter has **one production caller**, ServiceDetailLayout (:154). This shared-helper fix is global to the three plan-loading callers, with type-appropriate variants; no generic create-plan grid or unrelated route-family skeleton changes.

Canonical route: `/services/$serviceId/plan`. `/web/$`, `/worker/$`, `/pserv/$`, `/cron/$` are redirectRenderAlias shims preserving the suffix. Live replay covered the canonical Free Web route only. Cron, private and worker eligibility/grouping, aliases, the isolated server-query branch and route/parent pending branches are **unverified live** and are t002 verification work. Static redirects through NonStaticRoute to canonical Events and has no instance picker; Postgres and Key Value use separate plan surfaces and are outside this helper.

Adjacent outcomes: keep the existing catalog-error card; an unresolved service/type stays inert; retain existing missing-service, permission and signed-out route handling. No taxonomy or API policy change is proposed. Cached ready data should not be replaced with a fresh loading blank.

## Dedupe and prior completed guarantees

No open milestone covers the grouped Plan loading defect. Scanned all ten current open milestone READMEs, open/done board text, recent 40 dashboard/lego commits and the helper's history. No matching fix exists on main. Anti-goals were reread; loading shape is existing accepted product behavior.

Related `w5/done/m79` (“Route-by-route dashboard preloader skeleton completion”), t018, explicitly owns Scaling/Plan/Disk and the desktop/mobile shape gate. Its evidence table marks Plan Pass/Pass. This is an **original completion gap**, not a proven later regression: `c35b9fe03` already had this six-card helper; `458133694` added the route skeleton using it; the original ready Free/Paid grouping is present in `69f707c61`.

Walk of m79's entire Definition of done:

1. All 82 historical routes in a fail-closed manifest — unverified here; current source has manifest tests, not rerun for this documentation filing.
2. Every visible loading route has a deterministic-delay, chrome-preserving, nonshifting swap — the measured Free Web Plan branch fails the height condition. Chrome remains visible and this pass recorded zero console errors; the remaining routes/duplicate-chrome/hydration guarantee was not surveyed.
3. Exact outer/frame/columns/action/major-region parity, desktop and mobile — Plan fails because the separate Free/Paid structure is absent and height expands at both widths. Do not extend this result to every route.
4. Reuse the prior detail skeletons and fix absent/generic/mismatched states — reuse is visible, but the reused Plan helper does not satisfy the ready grouping. The older w9/m63 is Logs/Scaling work and does not own a Plan repair.
5. Redirect/server-only routes remain instant — not live-replayed; aliases were read, not behaviorally certified.
6. Desktop/mobile sweep, reduced motion and dashboard gates — this Plan pair fails the geometry portion. Reduced motion and the current full dashboard gates were not rerun; historical green claims are not current proof.

Related `w5/done/m7` owns the ready picker and intentionally omits prices/custom-instance contact; those deliberate deltas are not bugs. Its Free/Paid layout is the target, and its selection/disabled-Save behavior remains correct in this replay. Render reference is the repository's actual 2026-07-08 capture in that README; Render's transient loading animation was not captured this pass. No REST/GraphQL/MCP behavior change is needed.

## Cleanup

The only created resource was the owned Free Web service above. UI DeleteService returned HTTP 200 / true; REST lookup then returned 404. All six resource-ID inventory families matched the pass's baseline. Own logout completed; old whoami returned 401, cookies were cleared (0), browser is about:blank. No plan change was submitted. Full local record: `.playwright-mcp/qa-layout-a77-ledger.json`, cleanupVerified true.
