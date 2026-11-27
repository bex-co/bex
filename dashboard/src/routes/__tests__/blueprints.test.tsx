import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
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
import { Route as SyncRoute } from "../blueprints.$blueprintId.sync.$syncId";
import { Route as HandedOverSyncRoute } from "../blueprints.$blueprintId.sync.index";
import { Route as SettingsRoute } from "../blueprints.$blueprintId.settings";
import type {
  BlueprintSyncDetailView,
  BlueprintSyncView,
  BlueprintView,
} from "@/features/blueprints/types";
import type { PlanBlueprintSyncResult } from "@/features/blueprints/hooks/use-plan-blueprint-sync";
import type {
  BlueprintUpdate,
  BlueprintUpdateResult,
} from "@/features/blueprints/hooks/use-update-blueprint";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  CREATE_REASON,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";
import { datastoreHeaderGeometry } from "@/test/datastore-pages";
import { BlueprintSyncPageSkeleton } from "@/common/components/route-skeletons";

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

vi.mock("@/features/blueprints/hooks/use-blueprint", () => ({
  useBlueprint: () => blueprintDetailState,
}));

// --- Manual sync's plan (w3/m678) ---
const plan = vi.fn(async (): Promise<PlanBlueprintSyncResult> => ({
  status: "planned",
  syncId: "exe-planned",
}));
vi.mock("@/features/blueprints/hooks/use-plan-blueprint-sync", () => ({
  usePlanBlueprintSync: () => ({ plan, busy: false }),
}));

// --- one sync's read (w3/m678) ---
const approve = vi.fn(async () => undefined);
const blueprintSyncState: {
  sync: BlueprintSyncDetailView | null;
  loading: boolean;
  error: Error | undefined;
  approving: boolean;
  approveError: string | null;
} = {
  sync: null,
  loading: false,
  error: undefined,
  approving: false,
  approveError: null,
};
const readSync = vi.fn();
vi.mock("@/features/blueprints/hooks/use-blueprint-sync", async (orig) => ({
  ...(await orig<
    typeof import("@/features/blueprints/hooks/use-blueprint-sync")
  >()),
  useBlueprintSync: (blueprintId: string, syncId: string) => {
    readSync(blueprintId, syncId);
    return { ...blueprintSyncState, refetch: vi.fn(), approve };
  },
}));

// --- validate hook stub (used by ValidatePanel inside detail page) ---
vi.mock("@/features/blueprints/hooks/use-validate-blueprint", () => ({
  useValidateBlueprint: () => ({
    validate: vi.fn(async () => null),
    result: null,
    loading: false,
  }),
}));

// --- update hook stub ---
const update = vi.fn(
  async (
    _id: string,
    _fields: BlueprintUpdate,
  ): Promise<BlueprintUpdateResult> => ({ ok: true }),
);
vi.mock("@/features/blueprints/hooks/use-update-blueprint", () => ({
  useUpdateBlueprint: () => ({ update, busy: false }),
}));

// --- disconnect hook stub: null once disconnected, else the refusal ---
const disconnect = vi.fn(async (_id: string): Promise<string | null> => null);
vi.mock("@/features/blueprints/hooks/use-disconnect-blueprint", () => ({
  useDisconnectBlueprint: () => ({ disconnect, busy: false }),
}));

// --- the Branch row's picker reads the repository's branches (w3/m679) ---
const readBranches = vi.fn();
vi.mock("@/features/services/hooks/use-repo-branches", () => ({
  useRepoBranches: (repo: string | null) => {
    readBranches(repo);
    return { branches: repo ? ["main", "staging"] : [], loading: false };
  },
}));

// --- syncs hook stub ---
const loadMore = vi.fn();
const blueprintSyncsState: {
  syncs: BlueprintSyncView[];
  loading: boolean;
  loadingMore: boolean;
  error: Error | undefined;
  hasMore: boolean;
} = {
  syncs: [],
  loading: false,
  loadingMore: false,
  error: undefined,
  hasMore: false,
};
vi.mock("@/features/blueprints/hooks/use-blueprint-syncs", () => ({
  useBlueprintSyncs: () => ({ ...blueprintSyncsState, loadMore }),
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

/** One sync as Syncs lists it. */
function run(overrides: Partial<BlueprintSyncView> = {}): BlueprintSyncView {
  return {
    id: "exe-1",
    commitId: "deadbeefcafe",
    commitMessage: "Serve the api from nginx",
    state: "success",
    startedAt: "2026-08-20T00:00:00Z",
    completedAt: "2026-08-20T00:01:00Z",
    errorMessage: null,
    note: null,
    ...overrides,
  };
}

/** One sync as its page reads it. */
function detail(
  overrides: Partial<BlueprintSyncDetailView> = {},
): BlueprintSyncDetailView {
  return {
    ...run({
      id: "exe-planned",
      commitId: "0123456789abcdef",
      state: "created",
    }),
    actionsRecorded: true,
    actions: [
      {
        action: {
          operation: "create",
          kind: "service",
          name: "express-hello-world",
          sourcePath: "#/services/0",
          resourceId: null,
          changedFields: [],
          message: null,
        },
        state: "created",
      },
    ],
    errors: [],
    requiredManualEnvVars: [],
    ...overrides,
  };
}

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
  "/sync/$syncId": SyncRoute,
  "/sync/": HandedOverSyncRoute,
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
  blueprintSyncsState.loadingMore = false;
  blueprintSyncsState.error = undefined;
  blueprintSyncsState.hasMore = false;
  loadMore.mockClear();
  blueprintSyncState.sync = null;
  blueprintSyncState.loading = false;
  blueprintSyncState.error = undefined;
  blueprintSyncState.approving = false;
  blueprintSyncState.approveError = null;
  approve.mockClear();
  readSync.mockClear();
  readBranches.mockClear();
  disconnect.mockReset();
  disconnect.mockResolvedValue(null);
  update.mockReset();
  update.mockResolvedValue({ ok: true });
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  plan.mockReset();
  plan.mockResolvedValue({ status: "planned", syncId: "exe-planned" });
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
    [SYNCS, "Syncs", "No syncs yet."],
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
        "No syncs yet.",
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

    await user.click(
      await screen.findByRole("button", { name: "Disconnect Blueprint" }),
    );
    await user.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", {
        name: "Disconnect Blueprint",
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
        screen.queryByRole("button", { name: "Disconnect Blueprint" }),
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
    expect(
      screen.getByRole("button", { name: "Disconnect Blueprint" }),
    ).toBeDisabled();
    await user.hover(button.parentElement!);
    expect((await screen.findAllByText(CREATE_REASON)).length).toBeGreaterThan(
      0,
    );
  });

  it("links a waiting Blueprint's empty Resources to Syncs, where its plan is", async () => {
    blueprintDetailState.blueprint = bp({ status: "created", resources: [] });
    blueprintSyncsState.syncs = [run({ id: "exe-planned", state: "created" })];
    const user = userEvent.setup();
    const router = renderDetailPage();

    await user.click(await screen.findByRole("link", { name: "Go to Syncs" }));
    expect(router.state.location.pathname).toBe(SYNCS);
    expect(
      await screen.findByRole("img", { name: "Awaiting approval" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View details" })).toHaveAttribute(
      "href",
      "/blueprints/exs-abc123/sync/exe-planned",
    );
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
  // The page under its heading: its cards (with their edit-in-place rows'
  // labels, w3/m679), its sync cards (w3/m678), then its loose controls.
  const pageShape = (heading: Element) => {
    const page = heading.closest(".max-w-5xl")!;
    return {
      cards: [...page.querySelectorAll("[data-slot=card]")].map((card) => ({
        title: (
          card.querySelector("[data-slot=card-title]")?.textContent ?? ""
        ).replace(NAME, ""),
        columns: [...card.querySelectorAll("th")].map((th) => th.textContent),
        labels: [...card.querySelectorAll("dt")].map((dt) => dt.textContent),
        rows: [
          ...card.querySelectorAll(
            "[data-slot=card-content] > .space-y-2 > div:first-child > div:first-child",
          ),
        ].map((label) => label.textContent),
      })),
      syncCards: [...page.querySelectorAll(".rounded-lg.border.bg-card")].map(
        (card) => ({
          box: boxClasses(card),
          lines: [...card.querySelectorAll(".h-5, .leading-5")].length,
        }),
      ),
      controls: [...page.querySelectorAll("[data-slot=button], .invisible")]
        .filter((control) => control.textContent)
        .map((control) => ({
          label: control.textContent,
          classes: boxClasses(control),
        })),
    };
  };

  // The page's heading: the layout's, or on Settings the Settings card's
  // title (w3/m679).
  const pageHeading = () =>
    screen.findByText(
      (_, element) =>
        element?.tagName === "H2" &&
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
    // Syncs' frame holds three sync cards.
    blueprintSyncsState.syncs = ["exe-3", "exe-2", "exe-1"].map((id) =>
      run({ id }),
    );
    const readyShape = await ready(href);
    blueprintDetailState.blueprint = null;
    blueprintDetailState.loading = true;

    expect(await pending(href)).toEqual(readyShape);
  });

  // A sync's page heads itself (w3/m678): its frame holds the heading's line,
  // the sync's card without View details and the card of its actions.
  it("frames a sync's page in its ready shape", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.sync = detail();
    renderDetailPage("/blueprints/exs-abc123/sync/exe-planned");
    const heading = await screen.findByRole("heading", {
      level: 2,
      name: "Sync: 0123456",
    });
    const readyPage = heading.parentElement!;
    const readyCard = readyPage.querySelector(".rounded-lg.border.bg-card")!;
    const readyShape = {
      lineBox: heading.className.includes("leading-7"),
      card: boxClasses(readyCard),
      // The text column's lines, and the columns beside the icon.
      cardLines: readyCard.querySelector(":scope > div > div")!.children.length,
      cardColumns: readyCard.querySelectorAll(":scope > div > div").length,
      sections: readyPage.querySelectorAll("[data-slot=card]").length,
    };
    cleanup();

    const { container } = render(<BlueprintSyncPageSkeleton />);
    const frame = container.querySelector(
      '[data-route-skeleton="blueprint-sync"]',
    )!;
    const pendingCard = frame.querySelector(".rounded-lg.border.bg-card")!;
    expect({
      lineBox: !!frame.querySelector(
        '[data-skeleton-region="sync-heading"] .h-7',
      ),
      card: boxClasses(pendingCard),
      cardLines:
        pendingCard.querySelector(":scope > div > div")!.children.length,
      cardColumns: pendingCard.querySelectorAll(":scope > div > div").length,
      sections: frame.querySelectorAll("[data-slot=card]").length,
    }).toEqual(readyShape);
  });
});

describe("Syncs (w3/m678)", () => {
  it("lists Render's sync cards: state, linked commit and message, View details, the time", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.syncs = [
      run({ id: "exe-3", state: "running", commitId: "aaaaaaa1111" }),
      run({ id: "exe-2", state: "error", commitId: "bbbbbbb2222" }),
      run({ id: "exe-1", commitId: "ccccccc3333", commitMessage: "" }),
    ];
    renderDetailPage(SYNCS);

    const cards = await screen.findAllByTestId("blueprint-sync-card");
    expect(cards).toHaveLength(3);
    const [running, failed, succeeded] = cards;
    expect(within(running).getByRole("img", { name: "Running" })).toBeVisible();
    // Only a sync that is applying carries a badge.
    expect(within(running).getByText("Running")).toBeVisible();
    expect(within(failed).getByRole("img", { name: "Error" })).toBeVisible();
    expect(within(failed).queryByText("Error")).not.toBeInTheDocument();
    expect(
      within(succeeded).getByRole("img", { name: "Success" }),
    ).toBeVisible();

    const commit = within(running).getByRole("link", { name: "aaaaaaa" });
    expect(commit).toHaveAttribute(
      "href",
      "https://github.com/example/hello-go/commit/aaaaaaa1111",
    );
    expect(commit).toHaveAttribute("target", "_blank");
    expect(within(running).getByText("Serve the api from nginx")).toBeVisible();
    // No message: the short id alone, no dangling colon.
    expect(within(succeeded).queryByText(":")).not.toBeInTheDocument();
    expect(
      within(failed).getByRole("link", { name: "View details" }),
    ).toHaveAttribute("href", "/blueprints/exs-abc123/sync/exe-2");
    expect(within(failed).getByRole("time")).toHaveAttribute(
      "dateTime",
      "2026-08-20T00:00:00Z",
    );
  });

  it("reads older syncs with Load more while there are more", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.syncs = [run()];
    blueprintSyncsState.hasMore = true;
    const user = userEvent.setup();
    renderDetailPage(SYNCS);

    await user.click(await screen.findByRole("button", { name: "Load more" }));
    expect(loadMore).toHaveBeenCalledOnce();
  });

  it("offers no Load more on the last page", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.syncs = [run()];
    renderDetailPage(SYNCS);

    await screen.findByTestId("blueprint-sync-card");
    expect(
      screen.queryByRole("button", { name: "Load more" }),
    ).not.toBeInTheDocument();
  });

  // w6/043: the syncs read is not awaited by the route's title loader, so a
  // fresh visit has an in-flight window: it never reads "No syncs yet.".
  it("shows the cards' shape, not the empty copy, while syncs load", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.loading = true;
    renderDetailPage(SYNCS);

    await screen.findAllByText("hello-go");
    expect(screen.queryByText("No syncs yet.")).not.toBeInTheDocument();
    expect(
      document.querySelectorAll(
        '[data-skeleton-region="sync-history-rows"] .rounded-lg.border.bg-card',
      ),
    ).toHaveLength(3);
  });

  it("keeps the cards during a refetch", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncsState.loading = true;
    blueprintSyncsState.syncs = [run()];
    renderDetailPage(SYNCS);

    expect(await screen.findByTestId("blueprint-sync-card")).toBeVisible();
    expect(
      document.querySelector('[data-skeleton-region="sync-history-rows"]'),
    ).not.toBeInTheDocument();
  });
});

describe("a sync's page (w3/m678)", () => {
  const PAGE = "/blueprints/exs-abc123/sync/exe-planned";

  it("reviews a waiting plan and approves it with the values it needs", async () => {
    blueprintDetailState.blueprint = bp({ status: "created" });
    blueprintSyncState.sync = detail({
      requiredManualEnvVars: [
        {
          serviceType: "web",
          serviceName: "express-hello-world",
          key: "API_TOKEN",
        },
      ],
    });
    const user = userEvent.setup();
    renderDetailPage(PAGE);

    expect(
      await screen.findByRole("heading", { level: 2, name: "Sync: 0123456" }),
    ).toBeVisible();
    // The layout's own page heading gives way to the sync's.
    expect(
      screen.queryByRole("heading", { level: 2, name: "Syncs" }),
    ).not.toBeInTheDocument();
    expect(readSync).toHaveBeenCalledWith("exs-abc123", "exe-planned");
    const card = screen.getByTestId("blueprint-sync-card");
    expect(within(card).queryByText("View details")).not.toBeInTheDocument();
    const actions = screen.getByRole("list", { name: "Planned changes" });
    expect(actions).toHaveTextContent("Create Service express-hello-world");
    expect(
      within(actions).getByRole("img", { name: "Awaiting approval" }),
    ).toBeVisible();

    const approveButton = screen.getByRole("button", { name: "Approve" });
    expect(approveButton).toBeDisabled();
    await user.type(
      screen.getByLabelText("express-hello-world: API_TOKEN"),
      "s3cret",
    );
    await user.click(approveButton);
    expect(approve).toHaveBeenCalledWith({
      "express-hello-world/API_TOKEN": "s3cret",
    });
  });

  it("says why Approve failed, under the button", async () => {
    blueprintDetailState.blueprint = bp({ status: "created" });
    blueprintSyncState.sync = detail();
    blueprintSyncState.approveError = "Couldn't approve this sync. Try again.";
    renderDetailPage(PAGE);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't approve this sync. Try again.",
    );
    expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled();
  });

  it("holds Approve for a role that cannot change the Blueprint", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    blueprintDetailState.blueprint = bp({ status: "created" });
    blueprintSyncState.sync = detail();
    renderDetailPage(PAGE);

    expect(
      await screen.findByRole("button", { name: "Approve" }),
    ).toBeDisabled();
  });

  it.each([
    ["running", "Running"],
    ["success", "Success"],
  ])("follows a %s sync's actions, with no Approve", async (state, label) => {
    blueprintDetailState.blueprint = bp();
    const base = detail();
    blueprintSyncState.sync = detail({
      state,
      actions: base.actions.map((a) => ({ ...a, state })),
    });
    renderDetailPage(PAGE);

    const actions = await screen.findByRole("list", {
      name: "Planned changes",
    });
    expect(within(actions).getByRole("img", { name: label })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Approve" }),
    ).not.toBeInTheDocument();
  });

  it("reads Resources already up to date for a plan that changes nothing", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.sync = detail({
      state: "success",
      actions: [
        {
          ...detail().actions[0],
          action: { ...detail().actions[0].action, operation: "noop" },
          state: "success",
        },
      ],
    });
    renderDetailPage(PAGE);

    expect(
      await screen.findByText("Resources already up to date"),
    ).toBeVisible();
    expect(
      screen.queryByRole("list", { name: "Planned changes" }),
    ).not.toBeInTheDocument();
  });

  it("shows a failed plan's located errors alone", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.sync = detail({
      state: "error",
      actionsRecorded: false,
      actions: [],
      errors: [
        {
          code: "",
          error: "runtime rocket is not supported",
          path: "services[0].runtime",
          line: 4,
          column: 14,
        },
        { code: "", error: "name is required", path: "", line: 0, column: 0 },
      ],
    });
    renderDetailPage(PAGE);

    const errors = await screen.findByTestId("blueprint-sync-errors");
    expect(errors).toHaveTextContent(
      "There were a few problems syncing with your Blueprint:",
    );
    expect(errors).toHaveTextContent(
      "line 4, column 14 · services[0].runtimeruntime rocket is not supported",
    );
    expect(
      screen.queryByTestId("blueprint-sync-actions"),
    ).not.toBeInTheDocument();
  });

  it("reads Render's critical line for a failed sync with neither actions nor errors", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.sync = detail({ state: "error", actions: [] });
    renderDetailPage(PAGE);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "We weren't able to fully sync these resources.",
    );
  });

  it("says an older sync's actions were not recorded, and shows its note", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.sync = detail({
      state: "success",
      actionsRecorded: false,
      actions: [],
      note: "Detached old-api (srv-old); it remains running and may continue to incur charges.",
    });
    renderDetailPage(PAGE);

    expect(
      await screen.findByText(/recorded before syncs kept their actions/),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Detached old-api (srv-old); it remains running and may continue to incur charges.",
      ),
    ).toBeVisible();
  });

  it("says a sync that is gone is gone, and links Syncs", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage(PAGE);

    expect(
      await screen.findByText(/A planned sync not approved within 24 hours/),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Go to Syncs" })).toHaveAttribute(
      "href",
      SYNCS,
    );
  });

  it("frames the page while the sync loads", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.loading = true;
    renderDetailPage(PAGE);

    await screen.findAllByText("hello-go");
    expect(
      document.querySelector('[data-route-skeleton="blueprint-sync"]'),
    ).toBeInTheDocument();
  });

  it("sends /sync with nothing handed over to Syncs", async () => {
    blueprintDetailState.blueprint = bp();
    const router = renderDetailPage("/blueprints/exs-abc123/sync");

    expect(await screen.findByText("No syncs yet.")).toBeVisible();
    expect(router.state.location.pathname).toBe(SYNCS);
  });
});

describe("Manual sync (w3/m678)", () => {
  it("plans a sync and opens its page", async () => {
    blueprintDetailState.blueprint = bp();
    blueprintSyncState.sync = detail();
    const user = userEvent.setup();
    const router = renderDetailPage(SETTINGS);

    await user.click(await manualSync());
    expect(plan).toHaveBeenCalledWith("exs-abc123");
    expect(
      await screen.findByRole("heading", { level: 2, name: "Sync: 0123456" }),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe(
      "/blueprints/exs-abc123/sync/exe-planned",
    );
  });

  // Render's Manual sync is offered whatever the Blueprint's status: a created
  // Blueprint's new plan is approved on its page too.
  it("plans a created Blueprint's sync too", async () => {
    blueprintDetailState.blueprint = bp({ status: "created" });
    renderDetailPage(SYNCS);

    expect(await manualSync()).toBeEnabled();
  });

  it("opens a page saying Resources already up to date for a plan with no changes", async () => {
    plan.mockResolvedValueOnce({ status: "no_changes" });
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    const router = renderDetailPage(SYNCS);

    await user.click(await manualSync());
    expect(
      await screen.findByText("Resources already up to date"),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { level: 2, name: "Sync" }),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe("/blueprints/exs-abc123/sync");
  });

  it("shows a refused plan in Render's dismissible Sync Error banner", async () => {
    plan.mockResolvedValueOnce({
      status: "error",
      reason: "Blueprint file not found on branch main",
    });
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    const router = renderDetailPage(SYNCS);

    await user.click(await manualSync());
    const banner = await screen.findByTestId("blueprint-sync-error");
    expect(banner).toHaveTextContent("Sync Error");
    expect(banner).toHaveTextContent("Blueprint file not found on branch main");
    expect(router.state.location.pathname).toBe(SYNCS);
    await user.click(within(banner).getByRole("button", { name: "Dismiss" }));
    expect(
      screen.queryByTestId("blueprint-sync-error"),
    ).not.toBeInTheDocument();
  });

  // w2/m269: links carry the API's exs- id; a bookmarked blp- URL still
  // opens, plans by the exs- id and keeps its spelling.
  it.each([
    ["exs-db136288mmqc73d4hpug", "exs-db136288mmqc73d4hpug"],
    ["blp-db136288mmqc73d4hpug", "exs-db136288mmqc73d4hpug"],
  ])("plans /blueprints/%s by %s", async (urlId, apiId) => {
    blueprintDetailState.blueprint = bp({ id: apiId });
    blueprintSyncState.sync = detail();
    const user = userEvent.setup();
    const router = renderDetailPage(urlId);

    await user.click(await manualSync());
    expect(plan).toHaveBeenCalledWith(apiId);
    await screen.findByRole("heading", { level: 2, name: "Sync: 0123456" });
    expect(router.state.location.pathname).toBe(
      `/blueprints/${urlId}/sync/exe-planned`,
    );
  });
});

describe("BlueprintDetailPage", () => {
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
});

/** A refusal from bex-api's GraphQL with its code, as Apollo throws it. */
function refusedWith(code: string, message: string) {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [{ message, extensions: { code } }],
  });
}

// w3/m679: Render's Settings card (Blueprint Name, Auto Sync, Branch,
// Blueprint Path), its refusals in Render's words, and Disconnect Blueprint
// with Render's modal (docs/render-artifacts/blueprint-pages.md).
describe("a Blueprint's Settings (w3/m679)", () => {
  const settingsCard = async () =>
    (
      await screen.findByRole("heading", { level: 2, name: "Settings" })
    ).closest("[data-slot=card]") as HTMLElement;
  const editRow = async (
    user: ReturnType<typeof userEvent.setup>,
    label: string,
  ) => user.click(await screen.findByRole("button", { name: `Edit ${label}` }));
  const save = () => screen.getByRole("button", { name: "Save changes" });

  it("is Render's one Settings card, its rows in Render's order and words", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage(SETTINGS);

    const card = await settingsCard();
    // The card heads the page: no second Settings heading above it.
    expect(
      screen.getAllByRole("heading", { level: 2, name: "Settings" }),
    ).toHaveLength(1);
    const labels = [
      ...card.querySelectorAll(
        "[data-slot=card-content] > .space-y-2 > div:first-child > div:first-child",
      ),
    ].map((label) => label.textContent);
    expect(labels).toEqual([
      "Blueprint Name",
      "Auto Sync",
      "Branch",
      "Blueprint Path",
    ]);
    for (const help of [
      "A unique name for your Blueprint.",
      'Automatically sync changes to your Blueprint file? Select "No" to handle syncs manually.',
      "The Git branch to build and deploy.",
      "The path to the Blueprint file in your repo (e.g., infra/render.yaml). Defaults to render.yaml at the root of your repository.",
    ]) {
      expect(within(card).getByText(help)).toBeInTheDocument();
    }
    expect(
      within(card).getByRole("textbox", { name: "Blueprint Name" }),
    ).toHaveValue("hello-go");
    expect(
      within(card).getByRole("combobox", { name: "Auto Sync" }),
    ).toHaveTextContent("Yes");
    expect(within(card).getByRole("combobox", { name: "Branch" })).toHaveValue(
      "main",
    );
    expect(
      within(card).getByRole("textbox", { name: "Blueprint Path" }),
    ).toHaveValue("bex.yml");
    // bex's read-only facts sit below it, then the manifest, Validate and
    // Disconnect Blueprint last.
    const titles = [...document.querySelectorAll("[data-slot=card-title]")].map(
      (title) => title.textContent,
    );
    expect(titles.slice(0, 3)).toEqual([
      "Settings",
      "Details",
      "render.yaml manifest",
    ]);
    expect(
      screen.getByText("https://github.com/example/hello-go"),
    ).toBeInTheDocument();
    const disconnectButton = screen.getByRole("button", {
      name: "Disconnect Blueprint",
    });
    expect(
      card.compareDocumentPosition(disconnectButton) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("reads Auto Sync No for a paused Blueprint, and saves only Auto Sync", async () => {
    // Render reads the status: paused reads No whatever autoSync says.
    blueprintDetailState.blueprint = bp({ status: "paused", autoSync: true });
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    const autoSync = await screen.findByRole("combobox", { name: "Auto Sync" });
    expect(autoSync).toHaveTextContent("No");
    await editRow(user, "Auto Sync");
    await user.click(autoSync);
    await user.click(await screen.findByRole("option", { name: "Yes" }));
    await user.click(save());
    expect(update).toHaveBeenCalledWith("exs-abc123", { autoSync: true });
  });

  it("saves only the name, and refuses an empty or taken one at the field", async () => {
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    await editRow(user, "Blueprint Name");
    const name = screen.getByRole("textbox", { name: "Blueprint Name" });
    await user.clear(name);
    expect(screen.getByRole("alert")).toHaveTextContent(/^Required$/);
    expect(save()).toBeDisabled();

    update.mockResolvedValueOnce({
      ok: false,
      error: refusedWith(
        "BLUEPRINT_NAME_TAKEN",
        'a Blueprint named "taken" already exists in this workspace',
      ),
    });
    await user.type(name, "taken");
    await user.click(save());
    expect(update).toHaveBeenCalledWith("exs-abc123", { name: "taken" });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /^Name already in use$/,
    );
    // The row stays open on the refused name, which cannot be sent again.
    expect(save()).toBeDisabled();
    await user.type(name, "-2");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    await user.click(save());
    expect(update).toHaveBeenLastCalledWith("exs-abc123", { name: "taken-2" });
    expect(
      await screen.findByRole("button", { name: "Edit Blueprint Name" }),
    ).toBeInTheDocument();
  });

  it("moves the Blueprint to a branch from the repository's branches", async () => {
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    // The branches are read once Branch is edited, as Render's picker does.
    await settingsCard();
    expect(readBranches).not.toHaveBeenCalledWith(bp().repo);
    await editRow(user, "Branch");
    expect(readBranches).toHaveBeenLastCalledWith(bp().repo);
    const branch = screen.getByRole("combobox", { name: "Branch" });
    await user.clear(branch);
    await user.click(await screen.findByRole("option", { name: "staging" }));
    await user.click(save());
    expect(update).toHaveBeenCalledWith("exs-abc123", { branch: "staging" });
    expect(
      await screen.findByRole("button", { name: "Edit Branch" }),
    ).toBeInTheDocument();
  });

  it.each([
    [
      "file_not_found",
      'Blueprint file "bex.yml" not found on branch "staging".',
      'Blueprint file "bex.yml" not found on "staging" branch. Please check that the file exists on this branch.',
    ],
    [
      "branch_not_found",
      'Branch "staging" does not exist in this repository.',
      "Branch not found in this repository. Choose another branch.",
    ],
  ])(
    "refuses a branch with %s in Render's words and saves nothing",
    async (code, message, sentence) => {
      blueprintDetailState.blueprint = bp();
      update.mockResolvedValueOnce({
        ok: false,
        error: refusedWith(code, message),
      });
      const user = userEvent.setup();
      renderDetailPage(SETTINGS);

      await editRow(user, "Branch");
      const branch = screen.getByRole("combobox", { name: "Branch" });
      await user.clear(branch);
      await user.type(branch, "staging");
      await user.click(save());
      expect(await screen.findByRole("alert")).toHaveTextContent(sentence);
      expect(save()).toBeDisabled();
    },
  );

  it.each([
    [
      "the file is not at the path",
      refusedWith(
        "file_not_found",
        'Blueprint file "infra/render.yaml" not found on branch "main".',
      ),
      'Blueprint file "infra/render.yaml" not found on "main" branch. Please check that the file exists on this branch.',
    ],
    [
      "the server refuses for another reason",
      refusedWith(
        "unavailable",
        "Could not reach GitHub to read the Blueprint file. Try again in a few minutes.",
      ),
      "Could not reach GitHub to read the Blueprint file. Try again in a few minutes.",
    ],
    [
      "the request fails with no reason",
      new Error("Failed to fetch"),
      "Failed to update Blueprint. Please try again.",
    ],
  ])("refuses a path when %s at the field", async (_, error, sentence) => {
    blueprintDetailState.blueprint = bp();
    update.mockResolvedValueOnce({ ok: false, error });
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    await editRow(user, "Blueprint Path");
    const path = screen.getByRole("textbox", { name: "Blueprint Path" });
    await user.clear(path);
    await user.type(path, "infra/render.yaml");
    await user.click(save());
    expect(update).toHaveBeenCalledWith("exs-abc123", {
      path: "infra/render.yaml",
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(sentence);
  });

  it("checks a path's shape before sending it", async () => {
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    renderDetailPage(SETTINGS);

    await editRow(user, "Blueprint Path");
    const path = screen.getByRole("textbox", { name: "Blueprint Path" });
    await user.clear(path);
    await user.type(path, "../escape.yaml");
    expect(screen.getByRole("alert")).toHaveTextContent(
      /clean repository-relative/,
    );
    expect(save()).toBeDisabled();
    expect(update).not.toHaveBeenCalled();
  });

  it("shows a role that cannot change the Blueprint its rows read-only with the reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    blueprintDetailState.blueprint = bp();
    renderDetailPage(SETTINGS);

    const card = await settingsCard();
    expect(within(card).queryAllByRole("button", { name: /^Edit / })).toEqual(
      [],
    );
    expect(within(card).getAllByText(CREATE_REASON)).toHaveLength(4);
    expect(
      screen.getByRole("button", { name: "Disconnect Blueprint" }),
    ).toBeDisabled();
  });

  it("asks with Render's modal: Cancel keeps the Blueprint", async () => {
    blueprintDetailState.blueprint = bp();
    const user = userEvent.setup();
    const router = renderDetailPage(SETTINGS);

    await user.click(
      await screen.findByRole("button", { name: "Disconnect Blueprint" }),
    );
    const dialog = await screen.findByRole("alertdialog", {
      name: "Disconnect Blueprint",
    });
    expect(dialog).toHaveTextContent(
      "Resources will no longer sync automatically from your Blueprint file.",
    );
    expect(dialog).toHaveTextContent(
      "This will not delete the file itself. This will not delete the managed resources. You can always connect your file later from the Blueprint page.",
    );
    expect(dialog).toHaveTextContent(
      "Are you sure you want to disconnect your Blueprint spec hello-go?",
    );
    expect(within(dialog).getByText("hello-go").tagName).toBe("STRONG");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(disconnect).not.toHaveBeenCalled();
    expect(router.state.location.pathname).toBe(SETTINGS);
  });

  it("keeps a refused disconnect in the modal", async () => {
    blueprintDetailState.blueprint = bp();
    disconnect.mockResolvedValueOnce(
      "A sync is currently applying — retry disconnect after it settles",
    );
    const user = userEvent.setup();
    const router = renderDetailPage(SETTINGS);

    await user.click(
      await screen.findByRole("button", { name: "Disconnect Blueprint" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Disconnect Blueprint" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "A sync is currently applying — retry disconnect after it settles",
    );
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(SETTINGS);
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
