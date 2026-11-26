import { formatInstantDetails } from "@/common/lib/format";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { DeploysListPage } from "../deploys-list-page";
import { DeploysListSkeleton } from "@/common/components/route-skeletons";
import { hydrateAcrossBoundary, serverRendered } from "@/test/hydration";
import { mockAllowedResourceActions } from "@/test/mocks/resource-actions";
import type { DeployRow, UseDeploysResult } from "../../hooks/use-deploys";
import { usePipelineMinutesExceeded } from "@/features/usage/hooks/use-pipeline-minutes-exceeded";

const state: UseDeploysResult = {
  deploys: [],
  loading: false,
  loadingMore: false,
  error: undefined,
  hasMore: false,
  loadMore: vi.fn(),
  retry: vi.fn(),
};
const statusCalls: string[][] = [];

let deployActions: unknown = { status: "checking", refresh: vi.fn() };
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDeployActions: () => deployActions,
}));
vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext();
});
// Manual Build's trigger (w3/m555): the deploy verb, answering the new
// deploy's id.
const triggerDeploy = vi.fn(async (_serviceId: string) => "dep-new");
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({
    deploying: false,
    trigger: triggerDeploy,
    restart: vi.fn(),
  }),
}));

vi.mock("../../hooks/use-deploys", () => ({
  useDeploys: (_serviceId: string, statuses: string[]) => {
    statusCalls.push(statuses);
    return state;
  },
}));
// The service read that turns each row's SHA into a commit link; the repo is
// per-test state so both the linked and the plain-text shapes are covered.
const serverState: { repo: string | null; type: string } = {
  repo: null,
  type: "web_service",
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: { repo: serverState.repo, type: serverState.type },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("../deploy-actions", () => ({
  DeployActions: ({
    deployId,
    status,
  }: {
    deployId: string;
    status: string;
  }) =>
    status === "live" || status === "deactivated" ? (
      <button type="button">Rollback {deployId}</button>
    ) : null,
}));

function row(overrides: Partial<DeployRow> = {}): DeployRow {
  return {
    id: "dep-live",
    status: "deactivated",
    trigger: "api",
    image: "registry.example.com/web:1",
    imageSha: "",
    rollbackOf: "",
    buildAvailable: null,
    commitId: "abc1234def5678",
    commitMessage: "Ship searchable deploy history",
    commitCreatedAt: "2026-07-15T23:59:00Z",
    createdAt: "2026-07-16T00:00:00Z",
    updatedAt: "2026-07-16T00:01:30Z",
    startedAt: "2026-07-16T00:00:00Z",
    finishedAt: "2026-07-16T00:01:30Z",
    preDeployStatus: "succeeded",
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    failureReasonTemplate: null,
    stallReasonTemplate: null,
    cancelReasonTemplate: null,
    ...overrides,
  };
}

function pageRouter() {
  const root = createRootRoute();
  const list = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys",
    component: () => <DeploysListPage serviceId="web" />,
  });
  const detail = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => <p>deploy detail</p>,
  });
  const router = createRouter({
    routeTree: root.addChildren([list, detail]),
    history: createMemoryHistory({
      initialEntries: ["/services/web/deploys"],
    }),
    context: { client: {} as never, session: null },
  });
  return router;
}

function renderPage() {
  return render(<RouterProvider router={pageRouter()} />);
}

function headers(root: ParentNode) {
  return [...root.querySelectorAll("thead th")].map((th) => [
    th.textContent,
    th.className,
  ]);
}

// Row times are elapsed ("Deployed 3 hours ago"), so every test pins the
// clock: the row() defaults finish at 00:01:30, three hours before NOW.
const NOW = Date.parse("2026-07-16T03:00:00Z");

beforeEach(() => {
  vi.spyOn(Date, "now").mockReturnValue(NOW);
  serverState.repo = "https://github.com/acme/web.git";
  serverState.type = "web_service";
  deployActions = { status: "checking", refresh: vi.fn() };
  triggerDeploy.mockClear();
  state.deploys = [];
  state.loading = false;
  state.loadingMore = false;
  state.error = undefined;
  state.hasMore = false;
  state.loadMore = vi.fn();
  state.retry = vi.fn();
  statusCalls.length = 0;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("DeploysListPage", () => {
  it("advances a fixed deploy timestamp without new query data", async () => {
    vi.restoreAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    state.deploys = [
      row({ status: "live", finishedAt: "2026-07-16T02:55:00Z" }),
    ];
    const view = renderPage();
    try {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      const time = screen.getByText("Deployed 5 minutes ago");
      const rows = state.deploys;
      const originalRow = time.closest("tr");
      expect(time).toHaveAttribute("dateTime", "2026-07-16T02:55:00Z");
      act(() => time.focus());
      const tooltip = screen.getByRole("tooltip");
      const exactText = tooltip.textContent;
      const details = formatInstantDetails("2026-07-16T02:55:00Z")!;
      expect(tooltip).toHaveTextContent(`${details.zone}:${details.local}`);
      expect(tooltip).toHaveTextContent(`Timestamp:${details.unix}`);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(120_000);
      });
      expect(screen.getByText("Deployed 7 minutes ago")).toBe(time);
      expect(time).toHaveAttribute("dateTime", "2026-07-16T02:55:00Z");
      expect(time.closest("tr")).toBe(originalRow);
      expect(screen.getByRole("tooltip").textContent).toBe(exactText);
      expect(time).toHaveFocus();
      expect(state.deploys).toBe(rows);
    } finally {
      view.unmount();
      vi.useRealTimers();
    }
  });

  it("renders the Deploy/Trigger/Duration/action column headers", async () => {
    state.deploys = [row()];

    renderPage();

    expect(
      await screen.findByRole("columnheader", { name: "Deploy" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Trigger" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Duration" }),
    ).toBeInTheDocument();
    // The action column header is present for assistive tech but visually hidden.
    expect(
      screen.getByRole("columnheader", { name: "Actions" }),
    ).toBeInTheDocument();
  });

  it("renders rich metadata, honest count, and rollback only for successful history", async () => {
    state.deploys = [
      row(),
      row({
        id: "dep-failed",
        status: "build_failed",
        commitId: "fedcba987654321",
        commitMessage: "Broken startup",
        startedAt: "2026-07-16T00:02:00Z",
        finishedAt: "2026-07-16T00:02:09Z",
        preDeployStatus: "",
      }),
      row({
        id: "dep-current",
        status: "live",
        commitId: "0123456789abcde",
        commitMessage: "Current live deploy",
      }),
    ];
    state.hasMore = true;

    renderPage();

    expect(await screen.findByText("3 deploys loaded")).toBeInTheDocument();
    // Terminal durations render as the bare value under the Duration column (the
    // header supplies the label). Each finished deploy shows it in the desktop
    // cell and the mobile fold, so both settled 1m 30s deploys appear >= 2 times.
    expect(screen.getAllByText("1m 30s").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("9s").length).toBeGreaterThanOrEqual(1);
    expect(
      screen.getByText(/Ship searchable deploy history/),
    ).toBeInTheDocument();
    // Rollback is offered on a historical deactivated row, never the current
    // live row or a failed row (the list's hasListAction gate, not the button).
    expect(
      screen.getByRole("button", { name: "Rollback dep-live" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Rollback dep-failed" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Rollback dep-current" }),
    ).not.toBeInTheDocument();
  });

  it("humanizes every stored trigger, and names the restored deploy for a rollback", async () => {
    state.deploys = [
      row({ id: "dep-a", trigger: "create" }),
      row({ id: "dep-b", trigger: "api" }),
      row({ id: "dep-c", trigger: "deploy_hook" }),
      row({ id: "dep-d", trigger: "blueprint" }),
      row({ id: "dep-e", trigger: "rollback", rollbackOf: "dep-a" }),
      row({ id: "dep-f", trigger: "new_commit" }),
      row({ id: "dep-g", trigger: "config_change" }),
      row({ id: "dep-h", trigger: "manual" }),
    ];

    renderPage();

    expect((await screen.findAllByText("First Deploy")).length).toBeGreaterThan(
      0,
    );
    expect(screen.getAllByText("Manual Deploy").length).toBeGreaterThan(0);
    expect(screen.getAllByText("API Deploy").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Deploy Hook").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Blueprint Sync").length).toBeGreaterThan(0);
    const rollbackLabels = screen.getAllByText("Rollback to dep-a");
    expect(rollbackLabels.length).toBeGreaterThan(0);
    for (const el of rollbackLabels) {
      expect(el).not.toHaveClass("capitalize");
      expect(el.closest(".capitalize")).toBeNull();
    }
    expect(screen.getAllByText("New Commit").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Config Change").length).toBeGreaterThan(0);
    expect(screen.queryByText("new_commit")).not.toBeInTheDocument();
  });

  it("shows a running-elapsed marker for an active deploy and an em-dash before it starts", async () => {
    state.deploys = [
      row({
        id: "dep-running",
        status: "build_in_progress",
        finishedAt: null,
        preDeployStatus: "",
      }),
      row({
        id: "dep-created",
        status: "created",
        startedAt: null,
        finishedAt: null,
        preDeployStatus: "",
      }),
    ];

    renderPage();

    expect((await screen.findAllByText("In progress")).length).toBeGreaterThan(
      0,
    );
    // The created deploy never started, so its Duration reads as an em-dash.
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  // w4/m156: an image rollout first observed already live has no recorded
  // start. GraphQL keeps its empty-string encoding for that, and the row must
  // read as an unknown duration — never a fabricated 0s, never "In progress".
  it("shows an unknown duration for a live deploy whose start was never observed", async () => {
    state.deploys = [
      row({
        id: "dep-restart",
        status: "live",
        startedAt: "",
        preDeployStatus: "",
      }),
    ];

    renderPage();

    expect(await screen.findByText("Deployed 3 hours ago")).toBeInTheDocument();
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    expect(screen.queryByText("0s")).not.toBeInTheDocument();
    expect(screen.queryByText("In progress")).not.toBeInTheDocument();
  });

  it("labels rows by terminal state with elapsed time — Canceled/Failed rows never read 'Deployed' (w6/051)", async () => {
    state.deploys = [
      // createdAt 00:00, finishedAt 00:01:30 from the row() defaults.
      row({ id: "dep-shipped", status: "live" }),
      row({
        id: "dep-canceled",
        status: "canceled",
        finishedAt: "2026-07-16T02:30:00Z",
        preDeployStatus: "",
      }),
      row({
        id: "dep-broken",
        status: "build_failed",
        finishedAt: "2026-07-15T00:00:00Z",
        preDeployStatus: "",
      }),
      row({
        id: "dep-waiting",
        status: "queued",
        createdAt: "2026-07-16T02:59:50Z",
        startedAt: null,
        finishedAt: null,
        preDeployStatus: "",
      }),
    ];

    renderPage();

    // The live deploy is stamped with its finish time (3h before NOW), not
    // createdAt — and as elapsed text, Render's "Deployed 3 hours ago".
    const deployed = await screen.findByText("Deployed 3 hours ago");
    expect(deployed.tagName).toBe("TIME");
    expect(deployed).toHaveAttribute("dateTime", "2026-07-16T00:01:30Z");
    expect(screen.getByText("Canceled 30 minutes ago")).toBeInTheDocument();
    expect(screen.getByText("Failed 1 day ago")).toBeInTheDocument();
    // The queued deploy hasn't finished — it shows when it was created, and
    // under a minute reads "just now" rather than "10 seconds ago".
    expect(screen.getByText("Created just now")).toBeInTheDocument();
    // Exactly one row earned the "Deployed" verb.
    expect(screen.getAllByText(/^Deployed /)).toHaveLength(1);
  });

  // Render's time card is the one behind its "Deployed … ago" too (w4/716):
  // under vitest the viewer's zone is UTC, so there is no second UTC row.
  it("reveals Render's time card on hover, each value with its copy button", async () => {
    state.deploys = [row({ id: "dep-shipped", status: "live" })];
    const user = userEvent.setup();

    renderPage();
    await user.hover(await screen.findByText("Deployed 3 hours ago"));

    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent("UTC:Jul 16 12:01:30 AM");
    expect(tooltip).toHaveTextContent("Timestamp:1784160090000");
    expect(tooltip).not.toHaveTextContent("From previous");
    await user.click(
      within(tooltip).getByRole("button", { name: "Copy Timestamp" }),
    );
    expect(await navigator.clipboard.readText()).toBe("1784160090000");
  });

  it("links each commit SHA to its diff on the repo, outside the deploy link", async () => {
    state.deploys = [row()];

    renderPage();

    const commit = await screen.findByRole("link", { name: "abc1234" });
    expect(commit).toHaveAttribute(
      "href",
      "https://github.com/acme/web/commit/abc1234def5678",
    );
    expect(commit).toHaveAttribute("target", "_blank");
    expect(commit).toHaveAttribute("title", "abc1234def5678");
    // The deploy-detail link and the commit link are separate targets: an
    // anchor can't nest in an anchor, so the SHA sits beside the detail link.
    const detail = screen.getByRole("link", { name: /dep-live/ });
    expect(detail).toHaveAttribute("href", "/services/web/deploys/dep-live");
    expect(detail).not.toContainElement(commit);
    expect(commit).not.toContainElement(detail);
  });

  it("renders the SHA as plain text when the service has no browsable repo", async () => {
    serverState.repo = null;
    state.deploys = [row()];

    renderPage();

    expect(await screen.findByText("abc1234")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "abc1234" }),
    ).not.toBeInTheDocument();
  });

  it("shows a failed deploy's reason on its row and nothing extra on live rows (w1/m138)", async () => {
    state.deploys = [
      row({
        id: "dep-broken",
        status: "build_failed",
        finishedAt: "2026-07-16T00:02:09Z",
        preDeployStatus: "",
        failureReason: "image pull failed: not found",
      }),
      row({ id: "dep-shipped", status: "live" }),
    ];

    renderPage();

    // The failed row shows the same reason text the detail header shows.
    expect(
      await screen.findByText("image pull failed: not found"),
    ).toBeInTheDocument();
    // Exactly one reason line: the live row renders nothing extra, so its
    // height is unchanged.
    expect(screen.getAllByText("image pull failed: not found")).toHaveLength(1);
  });

  it("falls back to createdAt for a live deploy without a stored finish time", async () => {
    state.deploys = [
      row({ id: "dep-legacy", status: "live", finishedAt: null }),
    ];

    renderPage();

    // createdAt 00:00 is three hours before NOW.
    expect(await screen.findByText("Deployed 3 hours ago")).toHaveAttribute(
      "dateTime",
      "2026-07-16T00:00:00Z",
    );
  });

  it("keeps the row action button outside the deploy-detail link (action-click isolation)", async () => {
    state.deploys = [row()];

    renderPage();

    const link = await screen.findByRole("link", { name: /dep-live/ });
    expect(link).toHaveAttribute("href", "/services/web/deploys/dep-live");
    const rollback = screen.getByRole("button", { name: "Rollback dep-live" });
    // Navigation and the sibling action are separate targets: clicking Rollback
    // must not trigger the row's link.
    expect(link).not.toContainElement(rollback);
  });

  it("searches loaded ids, full commit SHAs, and messages case-insensitively", async () => {
    state.deploys = [
      row(),
      row({ id: "dep-failed", commitMessage: "Broken startup" }),
    ];
    const user = userEvent.setup();

    renderPage();
    const search = await screen.findByRole("textbox", {
      name: "Search loaded deploys and commits",
    });
    await user.type(search, "BROKEN");

    // While a search is typed the count says how many loaded deploys match.
    expect(screen.getByText("1 of 2 loaded deploys match")).toBeInTheDocument();
    expect(screen.getByText("dep-failed")).toBeInTheDocument();
    expect(screen.queryByText("dep-live")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "def5678");
    expect(screen.getByText("dep-live")).toBeInTheDocument();
  });

  it("offers a way out of a search with no match", async () => {
    state.deploys = [row()];
    const user = userEvent.setup();

    renderPage();
    const search = await screen.findByRole<HTMLInputElement>("textbox", {
      name: "Search loaded deploys and commits",
    });
    await user.type(search, "no-such-deploy");

    expect(screen.getByText("0 of 1 loaded deploys match")).toBeInTheDocument();
    expect(
      screen.getByText("No loaded deploys match your search."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear search" }));

    expect(search.value).toBe("");
    expect(screen.getByText("dep-live")).toBeInTheDocument();
  });

  it("names a failed load, offers a retry, and claims no count", async () => {
    state.error = new Error("network down");
    const user = userEvent.setup();

    renderPage();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't load deploys");
    expect(screen.queryByText("0 deploys")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(state.retry).toHaveBeenCalledTimes(1);
  });

  it("combines the client search with the server status filter", async () => {
    state.deploys = [
      row({ status: "build_failed", commitMessage: "Broken startup" }),
    ];
    const user = userEvent.setup();

    renderPage();
    await user.type(
      await screen.findByRole("textbox", {
        name: "Search loaded deploys and commits",
      }),
      "startup",
    );
    fireEvent.keyDown(
      screen.getByRole("combobox", { name: "Filter by status" }),
      { key: "ArrowDown" },
    );
    await user.click(
      await screen.findByRole("option", { name: "Build Failed" }),
    );

    await waitFor(() => {
      expect(statusCalls.at(-1)).toEqual(["build_failed"]);
    });
    expect(screen.getByText(/Broken startup/)).toBeInTheDocument();
  });

  // w4/702: while the first page loads, the card draws the ready table's
  // frame — its column headers, a full page of rows at a ready row's line
  // heights, the Load more slot and the count line's slot — and the route
  // skeleton draws the same card, so neither swap moves the page below.
  it("frames a full first page while it loads, as the route skeleton does (w4/702)", async () => {
    state.deploys = Array.from({ length: 20 }, (_, i) =>
      row({ id: `dep-${i}` }),
    );
    state.hasMore = true;
    const ready = renderPage();
    await screen.findByText("20 deploys loaded");
    const readyHeaders = headers(ready.container);
    expect(ready.container.querySelectorAll("tbody tr")).toHaveLength(20);
    const readyRow = ready.container.querySelector("tbody tr")!;
    const responsiveColumns = [...readyRow.classList].filter((name) =>
      name.includes("grid-cols-"),
    );
    expect(responsiveColumns.length).toBeGreaterThan(1);
    const cellClasses = readyRow.querySelector("td")!.className;
    const readyLoadMore = screen.getByRole("button", { name: "Load more" });
    expect(readyLoadMore.parentElement).toHaveClass("mt-4");
    ready.unmount();

    state.deploys = [];
    state.hasMore = false;
    state.loading = true;
    const pending = renderPage();
    const frame = await waitFor(() => {
      const found = pending.container.querySelector(
        "[data-testid='deploys-table-frame']",
      );
      expect(found).not.toBeNull();
      return found!;
    });
    expect(headers(pending.container)).toEqual(readyHeaders);
    const placeholders = frame.querySelectorAll(
      "tbody tr[data-testid='deploy-row-placeholder']",
    );
    expect(placeholders).toHaveLength(20);
    for (const placeholder of placeholders) {
      expect(placeholder).toHaveClass(...responsiveColumns);
      expect(placeholder.querySelector("td")!.className).toBe(cellClasses);
      expect(placeholder.children).toHaveLength(4);
    }
    // The Load more slot sits where the ready button does, at its size.
    const loadMoreSlot = frame.lastElementChild!;
    expect(loadMoreSlot).toHaveClass("mt-4");
    expect(loadMoreSlot).toHaveTextContent("Load more");
    expect(loadMoreSlot.querySelector("[data-slot='skeleton']")).not.toBeNull();
    // The count line keeps its text-xs line in the title row.
    expect(screen.getByTestId("deploys-count-slot")).toHaveClass("text-xs");
    // The search and filter stay usable while the list loads.
    expect(
      screen.getByRole("textbox", {
        name: "Search loaded deploys and commits",
      }),
    ).toBeInTheDocument();
    pending.unmount();

    const skeleton = render(<DeploysListSkeleton />);
    expect(headers(skeleton.container)).toEqual(readyHeaders);
    expect(
      skeleton.container.querySelectorAll(
        "tbody tr[data-testid='deploy-row-placeholder']",
      ),
    ).toHaveLength(20);
    expect(
      skeleton.container.querySelector("[data-testid='deploys-count-slot']"),
    ).not.toBeNull();
    // The tab's name is the detail layout's heading, not the list's.
    expect(skeleton.container.querySelector("h2")).toBeNull();
  });

  it("frames an image-backed service's rows without a commit line (w4/702)", async () => {
    serverState.repo = null;
    state.loading = true;
    const { container } = renderPage();
    const placeholder = await waitFor(() => {
      const found = container.querySelector(
        "[data-testid='deploy-row-placeholder'] td",
      );
      expect(found).not.toBeNull();
      return found!;
    });
    // Image-backed rows reserve a single headline for the deploy ID at all widths.
    expect(placeholder.children[1]).toHaveClass("min-h-5");
    expect(placeholder.children[1]).not.toHaveClass("@max-md/deploys:min-h-10");
    expect(placeholder.children[1].children).toHaveLength(1);
  });

  it("keeps a long rollback trigger from crowding out its duration on narrow screens", async () => {
    state.deploys = [
      row({
        id: "dep-d4fk2l0gjchc73a1b2c0",
        trigger: "rollback",
        rollbackOf: "dep-d4fk2l0gjchc73a1b2c9",
        preDeployStatus: "failed",
      }),
    ];
    renderPage();
    const id = await screen.findByTitle("dep-d4fk2l0gjchc73a1b2c0");
    const cell = id.closest("td")!;
    const trigger = within(cell).getByText(
      "Rollback to dep-d4fk2l0gjchc73a1b2c9",
    );
    expect(trigger).toHaveClass("truncate");
    expect(trigger).toHaveAttribute("title", trigger.textContent);
    expect(trigger.parentElement).toHaveClass("min-w-0", "whitespace-nowrap");
    expect(within(cell).getByText("1m 30s")).toHaveClass("shrink-0");
    expect(
      within(cell).getByText("Pre-deploy command failed"),
    ).toBeInTheDocument();
  });

  it("uses a complete count only after pagination is exhausted", async () => {
    state.deploys = [row()];
    state.hasMore = false;

    renderPage();

    expect(await screen.findByText("1 deploy")).toBeInTheDocument();
    expect(screen.queryByText(/loaded/)).not.toBeInTheDocument();
  });
});

// w4/702 (as w4/686): the loading frame's reserved text sits in a <p>, so
// every bar there must be phrasing content or a direct load fails hydration.
it("hydrates the route's loading frame without a mismatch (w4/702)", () => {
  const probe = hydrateAcrossBoundary(<DeploysListSkeleton />, {
    serverNow: NOW,
    clientNow: NOW,
  });
  expect(probe.html).toContain("deploys-table-frame");
  expect(probe.recovered).toEqual([]);
});

// w4/m219 (a841): a query typed into the server-rendered history search before
// handlers attach is wiped by hydration.
it("keeps the history search uneditable until hydration (w4/m219)", async () => {
  state.deploys = [
    row(),
    row({ id: "dep-failed", commitMessage: "Broken startup" }),
  ];
  const router = pageRouter();
  await router.load();
  const { container, hydrate, getByRole } = serverRendered(
    <RouterProvider router={router} />,
  );
  const search = () =>
    getByRole<HTMLInputElement>("textbox", {
      name: "Search loaded deploys and commits",
    });
  expect(search().disabled).toBe(true);
  await hydrate();
  expect(search().disabled).toBe(false);
  await userEvent.type(search(), "broken");
  expect(search().value).toBe("broken");
  expect(container).toHaveTextContent("dep-failed");
  expect(container).not.toHaveTextContent("dep-live");
});

// w3/m555: Render's Deploys page reads "Search deploys and commits" for every
// type, and a repo-backed cron job builds from it with Manual Build.
describe("DeploysListPage search and Manual Build", () => {
  it("reads Render's search placeholder on every Deploys page", async () => {
    state.deploys = [row()];
    renderPage();
    expect(
      await screen.findByRole("textbox", {
        name: "Search loaded deploys and commits",
      }),
    ).toHaveAttribute("placeholder", "Search deploys and commits");
  });

  it("builds a repo-backed cron job from Manual Build and opens the new build", async () => {
    serverState.type = "cron_job";
    deployActions = mockAllowedResourceActions("web").useDeployActions();
    state.deploys = [row()];
    const user = userEvent.setup();
    renderPage();

    const button = await screen.findByRole("button", { name: "Manual Build" });
    expect(button).toBeEnabled();
    await user.click(button);

    expect(triggerDeploy).toHaveBeenCalledExactlyOnceWith("web");
    // No confirm or menu: the build's page opens at once.
    expect(await screen.findByText("deploy detail")).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("stays on the list when the build is refused", async () => {
    serverState.type = "cron_job";
    deployActions = mockAllowedResourceActions("web").useDeployActions();
    triggerDeploy.mockResolvedValueOnce(null as never);
    state.deploys = [row()];
    const user = userEvent.setup();
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Manual Build" }),
    );
    expect(triggerDeploy).toHaveBeenCalledOnce();
    expect(screen.queryByText("deploy detail")).toBeNull();
    expect(screen.getByRole("button", { name: "Manual Build" })).toBeEnabled();
  });

  it("disables Manual Build with the permission's reason", async () => {
    serverState.type = "cron_job";
    deployActions = mockAllowedResourceActions("web", {
      deploy: { outcome: "denied" },
    }).useDeployActions();
    state.deploys = [row()];
    const user = userEvent.setup();
    renderPage();

    const button = await screen.findByRole("button", { name: "Manual Build" });
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can’t perform this action. Ask a workspace admin.",
    );
    expect(triggerDeploy).not.toHaveBeenCalled();
  });

  // Render's `h_e` (w3/108): with the workspace's pipeline minutes spent the
  // button is disabled with that notice, ahead of any other reason.
  it("disables Manual Build with Render's notice while pipeline minutes are spent", async () => {
    serverState.type = "cron_job";
    deployActions = mockAllowedResourceActions("web", {
      deploy: { outcome: "denied" },
    }).useDeployActions();
    vi.mocked(usePipelineMinutesExceeded).mockReturnValue(true);
    state.deploys = [row()];
    const user = userEvent.setup();
    renderPage();

    const button = await screen.findByRole("button", { name: "Manual Build" });
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    const notice = await screen.findByRole("tooltip");
    expect(notice).toHaveTextContent(
      "You’ve used up the maximum number of pipeline minutes specified by your spend limit.Learn more.",
    );
    expect(notice).not.toHaveTextContent("Your role");
    expect(
      within(notice).getByRole("link", { name: "pipeline minutes" }),
    ).toHaveAttribute("href", "/workspace/settings#build-pipeline");
    expect(
      within(notice).getByRole("link", { name: "Learn more" }),
    ).toHaveAttribute(
      "href",
      "https://render.com/docs/build-pipeline#pipeline-minutes",
    );
    await user.click(button);
    expect(triggerDeploy).not.toHaveBeenCalled();
  });

  it.each([
    ["an image-backed cron job", "cron_job", null],
    ["a web service", "web_service", "https://github.com/acme/web.git"],
    ["a background worker", "background_worker", "https://github.com/acme/w"],
  ])("offers no Manual Build on %s", async (_, type, repo) => {
    serverState.type = type;
    serverState.repo = repo;
    state.deploys = [row()];
    renderPage();
    await screen.findByRole("textbox", {
      name: "Search loaded deploys and commits",
    });
    expect(screen.queryByRole("button", { name: "Manual Build" })).toBeNull();
  });

  it("reserves Manual Build where the ready page draws it", async () => {
    serverState.type = "cron_job";
    deployActions = mockAllowedResourceActions("web").useDeployActions();
    state.deploys = [row()];
    const ready = renderPage();
    const button = await screen.findByRole("button", { name: "Manual Build" });
    const slot = button.closest("[data-deploys-action]")!;

    const skeleton = render(<DeploysListSkeleton manualBuild />);
    const reserved = skeleton.container.querySelector("[data-deploys-action]")!;
    expect(reserved).not.toBeNull();
    expect(reserved.className).toBe(slot.className);
    // The same button box: its variant, size and touch height.
    const box = reserved.querySelector(".invisible")!;
    for (const name of ["h-8", "max-lg:h-11", "border"]) {
      expect(box).toHaveClass(name);
      expect(button).toHaveClass(name);
    }
    expect(box).toHaveTextContent("Manual Build");
    expect(
      render(<DeploysListSkeleton />).container.querySelector(
        "[data-deploys-action]",
      ),
    ).toBeNull();
    ready.unmount();
  });
});
