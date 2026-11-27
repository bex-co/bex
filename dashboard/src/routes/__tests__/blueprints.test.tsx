import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  type RouteComponent,
} from "@tanstack/react-router";
import { BlueprintsPage } from "../blueprints";
import { Route as LayoutRoute } from "../blueprints.$blueprintId";
import { Route as IndexRoute } from "../blueprints.$blueprintId.index";
import { Route as ResourcesRoute } from "../blueprints.$blueprintId.resources";
import { Route as SyncsRoute } from "../blueprints.$blueprintId.syncs";
import { Route as SettingsRoute } from "../blueprints.$blueprintId.settings";
import type { BlueprintView } from "@/features/blueprints/types";
import type { BlueprintSyncActionResult } from "@/features/blueprints/hooks/use-sync-blueprint";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  CREATE_REASON,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";
import { datastoreHeaderGeometry } from "@/test/datastore-pages";
import { BlueprintSyncsSkeleton } from "@/common/components/route-skeletons";

// The page's own chrome is the shell's; a Blueprint's pages render inside it.
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

// --- list hook mock ---
const blueprintsState: {
  blueprints: BlueprintView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  blueprints: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => undefined),
};
vi.mock("@/features/blueprints/hooks/use-blueprints", () => ({
  useBlueprints: () => blueprintsState,
}));

// --- detail hook mock ---
const blueprintDetailState: {
  blueprint: BlueprintView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => void;
} = { blueprint: null, loading: false, error: undefined, refetch: vi.fn() };
const syncPreviewState: {
  preview: import("@/features/blueprints/types").BlueprintPreviewResult | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  preview: null,
  loading: false,
  error: undefined,
  refetch: async () => undefined,
};

vi.mock("@/features/blueprints/hooks/use-blueprint", () => ({
  useBlueprint: () => blueprintDetailState,
}));

// --- sync hook mock ---
const sync = vi.fn(async (): Promise<BlueprintSyncActionResult> => ({
  status: "success",
  result: null,
}));
vi.mock("@/features/blueprints/hooks/use-sync-blueprint", () => ({
  useSyncBlueprint: () => ({ sync, busy: false }),
}));

// --- planned-sync hook mock (w2/m256 t012) ---
type PlannedSyncView =
  import("@/features/blueprints/hooks/use-planned-sync").PlannedSyncView;
const waitingPlan: PlannedSyncView = {
  id: "exe-1",
  commitId: "0123456789abcdef",
  actions: [
    {
      operation: "create",
      kind: "service",
      name: "express-hello-world",
      sourcePath: "#/services/0",
      resourceId: null,
      changedFields: [],
      message: null,
    },
  ],
  requiredManualEnvVars: [],
};
const approvePlanned = vi.fn(async () => true);
const plannedSyncState = {
  planned: null as PlannedSyncView | null,
  loading: false,
  busy: false,
  approve: approvePlanned,
  replan: vi.fn(async () => undefined),
};
vi.mock(
  "@/features/blueprints/hooks/use-planned-sync",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/blueprints/hooks/use-planned-sync")
    >()),
    // A disabled query is skipped, as the real hook's is: it reads no plan.
    usePlannedSync: (_id: string, enabled: boolean) =>
      enabled
        ? plannedSyncState
        : { ...plannedSyncState, planned: null, loading: false },
  }),
);

// --- validate hook stub (used by ValidatePanel inside detail page) ---
vi.mock("@/features/blueprints/hooks/use-validate-blueprint", () => ({
  useValidateBlueprint: () => ({
    validate: vi.fn(async () => null),
    result: null,
    loading: false,
  }),
}));

// --- update hook stub ---
const update = vi.fn(async () => null);
vi.mock("@/features/blueprints/hooks/use-update-blueprint", () => ({
  useUpdateBlueprint: () => ({ update, busy: false }),
}));

// --- disconnect hook stub ---
const disconnect = vi.fn(async () => true);
vi.mock("@/features/blueprints/hooks/use-disconnect-blueprint", () => ({
  useDisconnectBlueprint: () => ({ disconnect, busy: false }),
}));

// --- syncs hook stub ---
vi.mock("@/features/blueprints/hooks/use-blueprint-preview", () => ({
  useBlueprintPreview: () => syncPreviewState,
}));

const blueprintSyncsState: {
  syncs: import("@/features/blueprints/types").BlueprintSyncView[];
  loading: boolean;
  error: Error | undefined;
} = { syncs: [], loading: false, error: undefined };
vi.mock("@/features/blueprints/hooks/use-blueprint-syncs", () => ({
  useBlueprintSyncs: () => blueprintSyncsState,
}));

function bp(overrides: Partial<BlueprintView> = {}): BlueprintView {
  return {
    id: "exs-abc123",
    name: "hello-go",
    resourceSuffix: "",
    repo: "https://github.com/example/hello-go",
    branch: "main",
    path: "bex.yml",
    autoSync: true,
    manifest: "services:\n  - name: api\n    type: web_service",
    status: "active",
    lastSync: null,
    resources: null,
    createdAt: "2026-07-01T00:00:00Z",
    updatedAt: "2026-07-13T00:00:00Z",
    ...overrides,
  };
}

/** A valid pre-sync preview — the only state that arms the sync button (w8/m41). */
function validPreview(
  commitId = "abc1234",
): import("@/features/blueprints/types").BlueprintPreviewResult {
  return {
    found: true,
    commitId,
    branch: "main",
    error: null,
    reason: null,
    retryable: false,
    // A preview on behalf of an existing Blueprint carries none of these.
    existingResources: null,
    resourceSuffix: null,
    suggestedName: null,
    autoSync: true,
    matchedResources: [],
    validation: {
      valid: true,
      errors: [],
      errorDetails: [],
      plan: {
        mode: "current_state",
        services: ["web"],
        databases: ["db"],
        keyValue: [],
        envGroups: [],
        syncFalseVars: null,
        totalActions: 2,
        actions: null,
      },
      estimatedPricing: {
        totalUsd: "17.50",
        lines: [
          {
            name: "web",
            tierLabel: "Standard",
            monthlyUsd: "17.50",
            instanceUsd: "17.50",
            storageUsd: null,
            storageGb: null,
          },
        ],
        variable: [],
      },
    },
  };
}

/** What the page pins from `validPreview()` for a `bp()` blueprint. */
const reviewedFrom = (blueprint: BlueprintView, commitId = "abc1234") => ({
  reviewed: {
    commitId,
    path: blueprint.path,
    repo: blueprint.repo,
  },
  confirmation: undefined as string | undefined,
});

function renderBlueprintsPage() {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints",
    component: BlueprintsPage,
  });
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprint/new",
    component: () => <div>new blueprint page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route, newRoute]),
    history: createMemoryHistory({ initialEntries: ["/blueprints"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

const DETAIL_PAGES = {
  "/": IndexRoute,
  "/resources": ResourcesRoute,
  "/syncs": SyncsRoute,
  "/settings": SettingsRoute,
};

/**
 * A Blueprint's layout route with its Resources, Syncs and Settings pages
 * under it, from their file Routes' own options (w3/m677), at `href`. The
 * layout's loader and auth guard stay out; the data hooks are stubbed.
 */
function renderDetailPage(
  href = "/blueprints/exs-abc123",
  { pending = false }: { pending?: boolean } = {},
) {
  const rootRoute = createRootRoute();
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>services home</div>,
  });
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints",
    component: () => <div>blueprints list</div>,
  });
  const layout = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints/$blueprintId",
    // The pending frame is the layout's own pendingComponent, as the router
    // paints it while the title loader runs.
    component: (pending
      ? LayoutRoute.options.pendingComponent
      : LayoutRoute.options.component) as RouteComponent,
  });
  const pages = Object.entries(DETAIL_PAGES).map(([path, page]) =>
    createRoute({
      getParentRoute: () => layout,
      path,
      component: page.options.component as RouteComponent,
    }),
  );
  const router = createRouter({
    // The file Routes' options are typed for the generated tree, not this one.
    routeTree: rootRoute.addChildren([
      homeRoute,
      listRoute,
      layout.addChildren(pages as never) as never,
    ]),
    history: createMemoryHistory({
      initialEntries: [href.startsWith("/") ? href : `/blueprints/${href}`],
    }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

/** The header's one action, Render's Manual sync (w3/m677). */
const manualSync = () => screen.findByRole("button", { name: "Manual sync" });

beforeEach(() => {
  blueprintsState.blueprints = [];
  blueprintsState.loading = false;
  blueprintsState.error = undefined;
  blueprintDetailState.blueprint = null;
  blueprintDetailState.loading = false;
  blueprintDetailState.error = undefined;
  blueprintDetailState.refetch = vi.fn();
  blueprintSyncsState.syncs = [];
  blueprintSyncsState.loading = false;
  blueprintSyncsState.error = undefined;
  // Reset here, not at the end of each test: a failing assertion would
  // otherwise leak one test's preview into the next one's dialog.
  syncPreviewState.preview = null;
  syncPreviewState.loading = false;
  syncPreviewState.error = undefined;
  plannedSyncState.planned = null;
  plannedSyncState.loading = false;
  approvePlanned.mockClear();
  disconnect.mockClear();
  update.mockClear();
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  sync.mockReset();
  sync.mockResolvedValue({ status: "success", result: null });
});

describe("BlueprintsPage (list)", () => {
  it("renders blueprint rows with name/repo/branch/status", async () => {
    blueprintsState.blueprints = [
      bp({
        name: "hello-go",
        repo: "https://github.com/example/hello-go",
        branch: "main",
      }),
    ];

    renderBlueprintsPage();

    expect(await screen.findByText("hello-go")).toBeInTheDocument();
    expect(
      screen.getByText("https://github.com/example/hello-go"),
    ).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  // w8/m153: several Blueprints can be made from one file, so name, repo and
  // branch can all be equal; the suffix each adds to its resource names is
  // what tells them apart.
  it("tells apart Blueprints made from one file by their resource suffix", async () => {
    blueprintsState.blueprints = [
      bp({ id: "exs-first" }),
      bp({ id: "exs-second", resourceSuffix: "ab12" }),
    ];

    renderBlueprintsPage();

    const [first, second] = (await screen.findAllByRole("row")).slice(1);
    const suffix = within(second).getByText("-ab12");
    expect(suffix).toHaveAttribute("title", "Resource name suffix");
    expect(suffix).toHaveTextContent("Resource name suffix -ab12");
    // The name stays the link, and a Blueprint without a suffix shows none.
    expect(within(second).getByRole("link")).toHaveTextContent(/^hello-go$/);
    expect(within(first).getAllByRole("cell")[0]).toHaveTextContent(
      /^hello-go$/,
    );
    expect(
      within(first).queryByTitle("Resource name suffix"),
    ).not.toBeInTheDocument();
  });

  it("shows empty state with auto-registration copy when there are no blueprints", async () => {
    renderBlueprintsPage();

    // Render's wording (w3/049): "Blueprint instance", not "blueprint".
    expect(
      await screen.findByText(
        "You haven't created any Blueprint instances yet.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/auto-register/i)).toBeInTheDocument();
  });

  it("offers New Blueprint from the empty state itself (ADR075 D1)", async () => {
    const router = renderBlueprintsPage();
    const buttons = await screen.findAllByRole("button", {
      name: "New Blueprint Instance",
    });
    expect(buttons).toHaveLength(2); // header + empty state
    await userEvent.click(buttons[1]);
    expect(router.state.location.pathname).toBe("/blueprint/new");
  });

  it("shows an error state when the query fails", async () => {
    blueprintsState.error = new Error("network down");
    renderBlueprintsPage();

    expect(
      await screen.findByText("Couldn't load blueprints"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("navigates to the New Blueprint page (Render-parity create flow)", async () => {
    const router = renderBlueprintsPage();

    // The header's button; an empty list also offers the empty-state one.
    const [headerButton] = await screen.findAllByRole("button", {
      name: "New Blueprint Instance",
    });
    await userEvent.click(headerButton);

    expect(await screen.findByText("new blueprint page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/blueprint/new");
  });
});

const SYNCS = "/blueprints/exs-abc123/syncs";
const SETTINGS = "/blueprints/exs-abc123/settings";

describe("a Blueprint's pages (w3/m677)", () => {
  it.each([
    ["/blueprints/exs-abc123", "Resources", "Managed Resources"],
    ["/blueprints/exs-abc123/resources", "Resources", "Managed Resources"],
    [SYNCS, "Syncs", "Sync History"],
    [SETTINGS, "Settings", "render.yaml manifest"],
  ])(
    "%s opens with its heading, %s, and holds its own cards",
    async (href, heading, card) => {
      blueprintDetailState.blueprint = bp();
      renderDetailPage(href);

      expect(
        await screen.findByRole("heading", { level: 2, name: heading }),
      ).toBeInTheDocument();
      expect(screen.getByText(card)).toBeInTheDocument();
      // The stacked page is gone: each page holds only its own cards.
      const others = [
        "Managed Resources",
        "Sync History",
        "render.yaml manifest",
      ].filter((other) => other !== card);
      for (const other of others) {
        expect(screen.queryByText(other)).not.toBeInTheDocument();
      }
    },
  );

  it("names the Blueprint in the header with one action, Manual sync", async () => {
    blueprintDetailState.blueprint = bp({ status: "in_sync" });
    renderDetailPage(SETTINGS);

    const heading = await screen.findByRole("heading", { level: 1 });
    expect(heading).toHaveTextContent(/^hello-go$/);
    const header = heading.parentElement!.parentElement!;
    expect(
      within(header)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Manual sync"]);
    expect(within(header).getByText("In Sync")).toBeInTheDocument();
  });

  it("disconnects from Settings, and offers Disconnect nowhere else", async () => {
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    const router = renderDetailPage(SETTINGS);

    await user.click(await screen.findByRole("button", { name: "Disconnect" }));
    await user.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", {
        name: "Disconnect",
      }),
    );
    expect(disconnect).toHaveBeenCalledWith("exs-abc123");
    expect(await screen.findByText("blueprints list")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/blueprints");
  });

  it.each(["/blueprints/exs-abc123", SYNCS])(
    "offers no Disconnect on %s",
    async (href) => {
      blueprintDetailState.blueprint = bp();
      renderDetailPage(href);

      await manualSync();
      expect(
        screen.queryByRole("button", { name: "Disconnect" }),
      ).not.toBeInTheDocument();
    },
  );

  // Render disables Manual sync and Disconnect Blueprint behind its
  // no-permission tooltip without updateBlueprint; bex-api changes a
  // Blueprint under can_create.
  it("holds Manual sync and Disconnect for a role that cannot change the Blueprint", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    const button = await manualSync();
    expect(button).toBeDisabled();
    expect(screen.getByRole("button", { name: "Disconnect" })).toBeDisabled();
    await user.hover(button.parentElement!);
    expect((await screen.findAllByText(CREATE_REASON)).length).toBeGreaterThan(
      0,
    );
  });

  it("links a waiting Blueprint's empty Resources to Syncs, where its plan is", async () => {
    blueprintDetailState.blueprint = bp({ status: "created", resources: [] });
    const user = userEvent.setup();
    const router = renderDetailPage();

    await user.click(await screen.findByRole("link", { name: "Go to Syncs" }));
    expect(router.state.location.pathname).toBe(SYNCS);
    expect(await screen.findByText("Planned sync")).toBeInTheDocument();
  });

  it("offers no Syncs link on a never-synced Blueprint's empty Resources", async () => {
    blueprintDetailState.blueprint = bp({ resources: [] });
    renderDetailPage();

    expect(await screen.findByText(/sync the Blueprint/i)).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Go to Syncs" }),
    ).not.toBeInTheDocument();
  });
});

// w3/m677 t006: each page's pending frame is a structural preview of its
// ready page: the same header boxes, the same cards in the same order with
// the same titles, table columns and field labels, and the same labelled
// controls at the same sizes. jsdom has no layout; the pixel comparison at
// 1440 and 390 is the browser walk's (the milestone README).
describe("each Blueprint page's pending frame (w3/m677)", () => {
  const NAME = "hello-go";
  const boxClasses = (element: Element) =>
    element.className
      .split(" ")
      .filter((name) => name && name !== "invisible")
      .join(" ");
  // The page under the layout's heading: its cards, then its loose controls.
  const pageShape = (heading: Element) => {
    const page = heading.parentElement!;
    return {
      cards: [...page.querySelectorAll("[data-slot=card]")].map((card) => ({
        title: (
          card.querySelector("[data-slot=card-title]")?.textContent ?? ""
        ).replace(NAME, ""),
        columns: [...card.querySelectorAll("th")].map((th) => th.textContent),
        labels: [...card.querySelectorAll("dt")].map((dt) => dt.textContent),
      })),
      controls: [...page.querySelectorAll("[data-slot=button], .invisible")]
        .filter((control) => control.textContent)
        .map((control) => ({
          label: control.textContent,
          classes: boxClasses(control),
        })),
    };
  };

  // The layout's page heading; a card's title is a heading too.
  const pageHeading = () =>
    screen.findByText(
      (_, element) =>
        element?.tagName === "H2" &&
        !element.hasAttribute("data-slot") &&
        ["Resources", "Syncs", "Settings"].includes(element.textContent ?? ""),
    );

  async function ready(href: string) {
    renderDetailPage(href);
    const heading = await pageHeading();
    await screen.findByRole("heading", { level: 1 });
    const header = screen
      .getByRole("heading", { level: 1 })
      .closest(".border-b")!;
    const shape = {
      heading: heading.textContent,
      header: datastoreHeaderGeometry(header),
      page: pageShape(heading),
    };
    cleanup();
    return shape;
  }

  async function pending(href: string) {
    renderDetailPage(href, { pending: true });
    const heading = await pageHeading();
    const header = document.querySelector(
      '[data-skeleton-region="resource-header"]',
    )!;
    expect(header).not.toBeNull();
    return {
      heading: heading.textContent,
      header: datastoreHeaderGeometry(header),
      page: pageShape(heading),
    };
  }

  it.each([
    [
      "/blueprints/exs-abc123",
      [{ id: "srv-1", name: "api", type: "web_service" }],
    ],
    [
      "/blueprints/exs-abc123/resources",
      [{ id: "srv-1", name: "api", type: "web_service" }],
    ],
    [SYNCS, null],
    [SETTINGS, null],
  ])("%s pends in its ready shape", async (href, resources) => {
    blueprintDetailState.blueprint = bp({ resources });
    blueprintSyncsState.syncs = [
      {
        id: "exe-1",
        commitId: "deadbeef",
        state: "success",
        startedAt: "2026-08-20T00:00:00Z",
        completedAt: "2026-08-20T00:01:00Z",
        errorMessage: null,
        note: null,
      },
    ];
    const readyShape = await ready(href);
    blueprintDetailState.blueprint = null;
    blueprintDetailState.loading = true;

    expect(await pending(href)).toEqual(readyShape);
  });

  // A created Blueprint's Syncs always has the waiting plan's card; the
  // frame reserves it when the cache already knows the Blueprint is created
  // (its workspace still being adopted), and never otherwise.
  it("reserves the waiting plan's card for a Blueprint the cache knows is created", () => {
    const { container, unmount } = render(
      <BlueprintSyncsSkeleton blueprint={{ status: "created" }} />,
    );
    const regionNames = [
      ...container.querySelectorAll("[data-skeleton-region]"),
    ].map((region) => region.getAttribute("data-skeleton-region"));
    expect(regionNames).toEqual([
      "planned-sync",
      "sync-history",
      "sync-history-rows",
    ]);
    unmount();
    const { container: other } = render(<BlueprintSyncsSkeleton />);
    expect(
      other.querySelector('[data-skeleton-region="planned-sync"]'),
    ).toBeNull();
  });
});

describe("BlueprintDetailPage", () => {
  // w2/m256 t012: a created Blueprint shows its plan and Approve, and offers no
  // Sync that would apply it past the plan: Manual sync is held, with where
  // to go (w3/m677).
  it("shows a created Blueprint's plan with Approve, and holds Manual sync", async () => {
    blueprintDetailState.blueprint = bp({ status: "created" });
    plannedSyncState.planned = waitingPlan;
    const user = userEvent.setup();
    renderDetailPage(SYNCS);

    await screen.findAllByText("hello-go");
    expect(screen.getByText("express-hello-world")).toBeInTheDocument();
    const button = await manualSync();
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    expect(
      (await screen.findAllByText("Approve the waiting plan on Syncs first."))
        .length,
    ).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    expect(approvePlanned).toHaveBeenCalledWith({});
  });

  // w32/076: a no-changes re-plan takes a created Blueprint to in_sync and
  // keeps its waiting first plan (w32/073), and an approved Blueprint can wait
  // on a re-plan. Render links every waiting plan to its dashboard for review,
  // and the CLI, REST, GraphQL and MCP can approve it, so the page shows it.
  it.each(["in_sync", "paused", "error"])(
    "shows a waiting plan and its Approve on a Blueprint that is %s",
    async (status) => {
      blueprintDetailState.blueprint = bp({ status });
      plannedSyncState.planned = waitingPlan;
      renderDetailPage(SYNCS);

      await screen.findAllByText("hello-go");
      expect(screen.getByText("Planned sync")).toBeInTheDocument();
      expect(
        screen.getByText(/this plan is waiting for approval/i),
      ).toHaveTextContent("commit 0123456");
      expect(screen.getByText("express-hello-world")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: /^approve$/i }));
      expect(approvePlanned).toHaveBeenCalledWith({});
      // Past created, the Blueprint still syncs as before.
      expect(await manualSync()).toBeEnabled();
    },
  );

  it("shows no planned-sync card on a Blueprint past created with no waiting plan (w32/076)", async () => {
    blueprintDetailState.blueprint = bp({ status: "in_sync" });
    renderDetailPage(SYNCS);

    await screen.findAllByText("hello-go");
    expect(screen.queryByText("Planned sync")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^approve$/i }),
    ).not.toBeInTheDocument();
    expect(await manualSync()).toBeEnabled();
  });

  // The skeleton rule: a loading frame never invents a region the ready page
  // lacks. A created Blueprint always has the card; any other has it only
  // when a plan waits, which it usually does not.
  it.each([
    ["created", true],
    ["in_sync", false],
  ] as const)(
    "while the plan loads, a %s Blueprint reserves the planned-sync card: %s (w32/076)",
    async (status, reserved) => {
      blueprintDetailState.blueprint = bp({ status });
      plannedSyncState.loading = true;
      renderDetailPage(SYNCS);

      await screen.findAllByText("hello-go");
      const frame = document.querySelector(
        '[data-skeleton-region="planned-sync"]',
      );
      if (reserved) expect(frame).toBeInTheDocument();
      else expect(frame).not.toBeInTheDocument();
    },
  );

  it("renders the manifest and metadata for a found blueprint on Settings", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage(SETTINGS);

    // The header's name and the Settings card's editable name.
    expect(await screen.findAllByText("hello-go")).toHaveLength(2);
    expect(screen.queryByTitle("Resource name suffix")).not.toBeInTheDocument();
    expect(screen.getByText(/services:.*name: api/s)).toBeInTheDocument();
    expect(
      screen.getByText("https://github.com/example/hello-go"),
    ).toBeInTheDocument();
  });

  // w8/m153: the header names the suffix beside the name, outside the heading.
  it("shows the resource suffix of a Blueprint created as a separate set", async () => {
    blueprintDetailState.blueprint = bp({ resourceSuffix: "ab12" });
    renderDetailPage();

    const heading = await screen.findByRole("heading", { level: 1 });
    expect(heading).toHaveTextContent(/^hello-go$/);
    const suffix = screen.getByTitle("Resource name suffix");
    expect(suffix).toHaveTextContent("-ab12");
    expect(heading.parentElement).toContainElement(suffix);
  });

  // w6/043: the syncs query is independent of the route's blocking title
  // loader, so a fresh visit has an in-flight window where zero rows have
  // arrived yet. The card must not read "No syncs yet." during that window.
  describe("Sync History loading window", () => {
    it("shows a table-shaped skeleton, not the empty copy, while syncs are loading", async () => {
      blueprintDetailState.blueprint = bp();
      blueprintSyncsState.loading = true;
      renderDetailPage(SYNCS);

      await screen.findAllByText("hello-go");
      expect(screen.queryByText("No syncs yet.")).not.toBeInTheDocument();
      const skeletonBody = document.querySelector(
        '[data-skeleton-region="sync-history-rows"]',
      );
      expect(skeletonBody).toBeInTheDocument();
      // Structural parity with the ready state: the real header plus
      // placeholder rows spanning the same five columns.
      expect(screen.getByText("Commit")).toBeInTheDocument();
      const skeletonRows = skeletonBody?.querySelectorAll("tr") ?? [];
      expect(skeletonRows.length).toBeGreaterThan(0);
      expect(skeletonRows[0].children).toHaveLength(5);
    });

    it("shows the empty copy only once settled with zero syncs", async () => {
      blueprintDetailState.blueprint = bp();
      blueprintSyncsState.loading = false;
      renderDetailPage(SYNCS);

      expect(await screen.findByText("No syncs yet.")).toBeInTheDocument();
      expect(
        document.querySelector('[data-skeleton-region="sync-history-rows"]'),
      ).not.toBeInTheDocument();
    });

    it("keeps the real rows during a cache-and-network refetch", async () => {
      blueprintDetailState.blueprint = bp();
      blueprintSyncsState.loading = true;
      blueprintSyncsState.syncs = [
        {
          id: "exe-1",
          commitId: "deadbeef",
          state: "success",
          startedAt: "2026-08-20T00:00:00Z",
          completedAt: "2026-08-20T00:01:00Z",
          errorMessage: null,
          note: null,
        },
      ];
      renderDetailPage(SYNCS);

      expect(await screen.findByText("deadbeef")).toBeInTheDocument();
      expect(screen.queryByText("No syncs yet.")).not.toBeInTheDocument();
      expect(
        document.querySelector('[data-skeleton-region="sync-history-rows"]'),
      ).not.toBeInTheDocument();
    });
  });

  it("shows a durable detachment note on an otherwise successful sync", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.syncs = [
      {
        id: "exe-detach",
        commitId: "abc12345",
        state: "success",
        startedAt: null,
        completedAt: null,
        errorMessage: null,
        note: "Detached old-api (srv-old); it remains running and may continue to incur charges.",
      },
    ];
    renderDetailPage(SYNCS);
    expect(
      await screen.findByText(
        "Detached old-api (srv-old); it remains running and may continue to incur charges.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Details" }),
    ).toBeInTheDocument();
  });

  // w2/074: a Blueprint reconnected twice shows the reconnection note on
  // exactly the two rows the API marks, the first sync of each connection,
  // and on none of the syncs around them.
  it("shows one reconnection note per reconnection boundary", async () => {
    const note =
      "re-established a previously disconnected connection to https://github.com/a/app@main; earlier sync history below belongs to that connection";
    const run = (id: string, commitId: string, boundary = false) => ({
      id,
      commitId,
      state: "success",
      startedAt: "2026-11-08T00:00:00Z",
      completedAt: "2026-11-08T00:01:00Z",
      errorMessage: null,
      note: boundary ? note : null,
    });
    blueprintDetailState.blueprint = bp();
    // Newest first: connection 3 (two syncs), 2 (two syncs), 1 (one sync).
    blueprintSyncsState.syncs = [
      run("exe-dbo4cl1jg4r0393ruu50", "c3000002"),
      run("exe-dbo4cl1jg4r0393ruu30", "c3000001", true),
      run("exe-dbo4cl1jg4r0393ruu20", "c2000002"),
      run("exe-dbo4cl1jg4r0393ruu00", "c2000001", true),
      run("exe-dbo4cl1jg4r0393rutsg", "c1000001"),
    ];
    renderDetailPage(SYNCS);

    expect(await screen.findByText("c3000002")).toBeInTheDocument();
    const rowOf = (commit: string) =>
      screen
        .getAllByRole("row")
        .find((row) => row.textContent?.includes(commit)) as HTMLElement;
    expect(screen.getAllByText(note)).toHaveLength(2);
    for (const commit of ["c3000001", "c2000001"]) {
      expect(within(rowOf(commit)).getByText(note)).toBeInTheDocument();
    }
    for (const commit of ["c3000002", "c2000002", "c1000001"]) {
      expect(within(rowOf(commit)).queryByText(note)).not.toBeInTheDocument();
    }
  });

  it("shows the failure reason for an error-state sync row and nothing extraneous for a success row (w6/m50)", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.syncs = [
      {
        id: "exe-1",
        commitId: "deadbeef",
        state: "error",
        startedAt: "2026-08-20T00:00:00Z",
        completedAt: "2026-08-20T00:01:00Z",
        errorMessage: "quota exceeded: workspace at service limit",
        note: null,
      },
      {
        id: "exe-2",
        commitId: "cafef00d",
        state: "success",
        startedAt: "2026-08-21T00:00:00Z",
        completedAt: "2026-08-21T00:01:00Z",
        errorMessage: null,
        note: null,
      },
    ];
    renderDetailPage(SYNCS);

    expect(
      await screen.findByText("quota exceeded: workspace at service limit"),
    ).toBeInTheDocument();

    const successRow = screen
      .getAllByRole("row")
      .find((row) => row.textContent?.includes("cafef00d"));
    expect(successRow).toBeDefined();
    expect(
      within(successRow as HTMLElement).getAllByText("—").length,
    ).toBeGreaterThan(0);
  });

  it("does not tell a failed empty blueprint to sync (w4/065)", async () => {
    blueprintDetailState.blueprint = bp({
      status: "error",
      lastSync: "2026-08-26T10:26:53Z",
      resources: [],
    });
    renderDetailPage();

    expect(
      await screen.findByText(/the last sync failed/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/sync the Blueprint to apply/i),
    ).not.toBeInTheDocument();
    // The failed sync is on Syncs now (w3/m677).
    expect(screen.getByRole("link", { name: "Go to Syncs" })).toHaveAttribute(
      "href",
      SYNCS,
    );
  });

  it("tells a Blueprint awaiting approval to approve, not sync (w2/m256 t013)", async () => {
    blueprintDetailState.blueprint = bp({ status: "created", resources: [] });
    renderDetailPage();

    expect(
      await screen.findByText(/approve the planned sync/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/sync the Blueprint to apply/i),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("Awaiting approval").length).toBeGreaterThan(0);
    expect(screen.queryByText("Queued")).not.toBeInTheDocument();
  });

  it("redirects a dead blueprint id home (w9/m55)", async () => {
    blueprintDetailState.blueprint = null;
    const router = renderDetailPage("blp-missing");

    expect(await screen.findByText("services home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("stays put on the inline error state when the query fails (w9/m55)", async () => {
    blueprintDetailState.blueprint = null;
    blueprintDetailState.error = new Error("bex-api unreachable");
    const router = renderDetailPage("blp-missing");

    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/blueprints/blp-missing");
  });

  it("syncs the commit the dialog reviewed (w8/m41)", async () => {
    const blueprint = bp();
    blueprintDetailState.blueprint = blueprint;
    syncPreviewState.preview = validPreview();
    renderDetailPage();

    await userEvent.click(await manualSync());
    await userEvent.click(screen.getByRole("button", { name: /^sync$/i }));

    expect(sync).toHaveBeenCalledWith("exs-abc123", reviewedFrom(blueprint));
    expect(blueprintDetailState.refetch).not.toHaveBeenCalled();
  });

  // w2/m269: links carry the API's exs- id; a bookmarked blp- URL still
  // opens and addresses the same Blueprint by its exs- id.
  it.each([
    ["exs-db136288mmqc73d4hpug", "exs-db136288mmqc73d4hpug"],
    ["blp-db136288mmqc73d4hpug", "exs-db136288mmqc73d4hpug"],
  ])("opens /blueprints/%s and acts on %s", async (urlId, apiId) => {
    const blueprint = bp({ id: apiId });
    blueprintDetailState.blueprint = blueprint;
    syncPreviewState.preview = validPreview();
    const router = renderDetailPage(urlId);

    await userEvent.click(await manualSync());
    await userEvent.click(screen.getByRole("button", { name: /^sync$/i }));

    expect(sync).toHaveBeenCalledWith(apiId, reviewedFrom(blueprint));
    expect(router.state.location.pathname).toBe(`/blueprints/${urlId}`);
  });

  it("keeps the reviewed pin when a later preview arrives (w8/m41)", async () => {
    const blueprint = bp();
    blueprintDetailState.blueprint = blueprint;
    syncPreviewState.preview = validPreview();
    renderDetailPage();

    await userEvent.click(await manualSync());
    expect(await screen.findByText(/reviewed commit/i)).toBeInTheDocument();

    // A refetch lands a newer commit while the dialog is open: the user
    // confirmed what they read, so the pin must not follow it.
    syncPreviewState.preview = validPreview("f00dcafe");
    await userEvent.click(screen.getByRole("button", { name: /^sync$/i }));

    expect(sync).toHaveBeenCalledWith("exs-abc123", reviewedFrom(blueprint));
  });

  // Each opening reviews the file as it is now (w3/m677): a pin left by a
  // dismissed protected-environment retry never carries into the next one.
  it("reviews afresh each time the dialog opens", async () => {
    sync.mockResolvedValueOnce({
      status: "confirmation_required",
      confirmation: "sudo deploy service api",
    });
    const blueprint = bp();
    blueprintDetailState.blueprint = blueprint;
    syncPreviewState.preview = validPreview();
    const user = userEvent.setup();
    renderDetailPage();

    await user.click(await manualSync());
    await user.click(screen.getByRole("button", { name: /^sync$/i }));
    const protectedDialog = await screen.findByRole("dialog");
    await user.click(
      within(protectedDialog).getByRole("button", { name: /cancel/i }),
    );

    // The file moved on while the user was away.
    syncPreviewState.preview = validPreview("f00dcafe");
    await user.click(await manualSync());
    await user.click(screen.getByRole("button", { name: /^sync$/i }));
    expect(sync).toHaveBeenLastCalledWith(
      "exs-abc123",
      reviewedFrom(blueprint, "f00dcafe"),
    );
  });

  it("shows the computed sync plan in the dialog before applying (w8/m21)", async () => {
    const blueprint = bp();
    blueprintDetailState.blueprint = blueprint;
    syncPreviewState.preview = validPreview();
    renderDetailPage();

    await userEvent.click(await manualSync());
    expect(
      await screen.findByText(/parsed successfully — 2 resources/i),
    ).toBeInTheDocument();
    expect(screen.getByText("(Standard) $17.50 / month")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /^sync$/i }));
    expect(sync).toHaveBeenCalledWith("exs-abc123", reviewedFrom(blueprint));
  });

  it("blocks sync when the preview is unavailable (w8/m41)", async () => {
    blueprintDetailState.blueprint = bp();
    syncPreviewState.error = new Error("network");
    renderDetailPage();

    await userEvent.click(await manualSync());
    expect(
      await screen.findByText(/couldn't compute the sync plan/i),
    ).toBeInTheDocument();
    // No reviewed commit, no sync: proceed-anyway is gone (w8/m41).
    expect(screen.getByRole("button", { name: /^sync$/i })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /refresh preview/i }),
    ).toBeInTheDocument();
    expect(sync).not.toHaveBeenCalled();
  });

  it("does not call sync when the confirm dialog is cancelled", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage();

    await userEvent.click(await manualSync());
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));

    expect(sync).not.toHaveBeenCalled();
  });

  it("edits name and path via PATCH on Settings; branch stays read-only (w8/m21)", async () => {
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    // Path edit: pencil → input → invalid rejected client-side → valid saves.
    await user.click(
      await screen.findByRole("button", { name: /edit manifest path/i }),
    );
    const pathInput = screen.getByRole("textbox", { name: /manifest path/i });
    await user.clear(pathInput);
    await user.type(pathInput, "../escape.yaml");
    expect(
      screen.getByRole("button", { name: /save manifest path/i }),
    ).toBeDisabled();
    expect(screen.getByText(/clean repository-relative/i)).toBeInTheDocument();
    await user.clear(pathInput);
    await user.type(pathInput, "infra/bex/stack.yaml");
    await user.click(
      screen.getByRole("button", { name: /save manifest path/i }),
    );
    expect(update).toHaveBeenCalledWith("exs-abc123", {
      path: "infra/bex/stack.yaml",
    });

    // Name edit PATCHes only the name.
    await user.click(
      screen.getByRole("button", { name: /edit blueprint name/i }),
    );
    const nameInput = screen.getByRole("textbox", { name: /blueprint name/i });
    await user.clear(nameInput);
    await user.type(nameInput, "renamed");
    await user.click(
      screen.getByRole("button", { name: /save blueprint name/i }),
    );
    expect(update).toHaveBeenCalledWith("exs-abc123", { name: "renamed" });

    // Auto-sync PATCHes only the switch.
    await user.click(screen.getByRole("switch", { name: /auto-sync/i }));
    expect(update).toHaveBeenCalledWith("exs-abc123", { autoSync: false });

    // Branch has no edit affordance.
    expect(
      screen.queryByRole("button", { name: /edit branch/i }),
    ).not.toBeInTheDocument();
  });

  it("retries a protected deploy override only with the exact server phrase", async () => {
    sync
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo deploy service api",
      })
      .mockResolvedValueOnce({ status: "success", result: null });
    const blueprint = bp();
    blueprintDetailState.blueprint = blueprint;
    syncPreviewState.preview = validPreview();
    const user = userEvent.setup();
    renderDetailPage();

    await user.click(await manualSync());
    await user.click(screen.getByRole("button", { name: /^sync$/i }));

    const protectedDialog = await screen.findByRole("dialog");
    // The backend's phrase is body copy; the input is labeled "Sudo Command".
    expect(
      within(protectedDialog).getByText("sudo deploy service api"),
    ).toBeInTheDocument();
    const input = within(protectedDialog).getByLabelText("Sudo Command");
    const retry = within(protectedDialog).getByRole("button", {
      name: /^sync$/i,
    });
    await user.type(input, "sudo deploy service ap");
    expect(retry).toBeDisabled();
    await user.type(input, "i");
    await user.click(retry);

    expect(sync).toHaveBeenNthCalledWith(2, "exs-abc123", {
      ...reviewedFrom(blueprint),
      confirmation: "sudo deploy service api",
    });
    expect(blueprintDetailState.refetch).not.toHaveBeenCalled();
  });
});

// w4/m220 (a822): Blueprint list labels stacked vertically in a 180–220px
// mobile header; they now stay on one line (36px, browser-measured).
it("keeps every Blueprint column label on one line (w4/m220)", async () => {
  blueprintsState.blueprints = [bp({ name: "hello-go" })];
  renderBlueprintsPage();
  await screen.findByText("hello-go");
  for (const header of screen.getAllByRole("columnheader")) {
    expect(header).toHaveClass("whitespace-nowrap");
  }
});
