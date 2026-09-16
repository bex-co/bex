import { formatInstantDetails } from "@/common/lib/format";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { DeploysListPage } from "../deploys-list-page";
import type { DeployRow, UseDeploysResult } from "../../hooks/use-deploys";

const state: UseDeploysResult = {
  deploys: [],
  loading: false,
  loadingMore: false,
  error: undefined,
  hasMore: false,
  loadMore: vi.fn(),
};
const statusCalls: string[][] = [];

vi.mock("../../hooks/use-deploys", () => ({
  useDeploys: (_serviceId: string, statuses: string[]) => {
    statusCalls.push(statuses);
    return state;
  },
}));
// The service read that turns each row's SHA into a commit link; the repo is
// per-test state so both the linked and the plain-text shapes are covered.
const serverState: { repo: string | null } = { repo: null };
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: { repo: serverState.repo },
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
    rollbackOf: "",
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
    ...overrides,
  };
}

function renderPage() {
  const root = createRootRoute();
  const list = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys",
    component: () => <DeploysListPage serviceId="web" />,
  });
  const detail = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => null,
  });
  const router = createRouter({
    routeTree: root.addChildren([list, detail]),
    history: createMemoryHistory({
      initialEntries: ["/services/web/deploys"],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

// Row times are elapsed ("Deployed 3 hours ago"), so every test pins the
// clock: the row() defaults finish at 00:01:30, three hours before NOW.
const NOW = Date.parse("2026-07-16T03:00:00Z");

beforeEach(() => {
  vi.spyOn(Date, "now").mockReturnValue(NOW);
  serverState.repo = "https://github.com/acme/web.git";
  state.deploys = [];
  state.loading = false;
  state.loadingMore = false;
  state.error = undefined;
  state.hasMore = false;
  state.loadMore = vi.fn();
  statusCalls.length = 0;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("DeploysListPage", () => {
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
    ];

    renderPage();

    expect((await screen.findAllByText("first deploy")).length).toBeGreaterThan(
      0,
    );
    expect(screen.getAllByText("manual deploy").length).toBeGreaterThan(0);
    expect(screen.getAllByText("deploy hook").length).toBeGreaterThan(0);
    expect(screen.getAllByText("blueprint sync").length).toBeGreaterThan(0);
    expect(screen.getAllByText("rollback to dep-a").length).toBeGreaterThan(0);
    expect(screen.getAllByText("new commit").length).toBeGreaterThan(0);
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

  it("reveals the exact instant — local, UTC, Unix — on hover, selectable for copying", async () => {
    state.deploys = [row({ id: "dep-shipped", status: "live" })];
    const user = userEvent.setup();

    renderPage();
    await user.hover(await screen.findByText("Deployed 3 hours ago"));

    const tooltip = await screen.findByRole("tooltip");
    const details = formatInstantDetails("2026-07-16T00:01:30Z")!;
    expect(tooltip).toHaveTextContent(`Local${details.local}`);
    expect(tooltip).toHaveTextContent("UTCJuly 16, 2026 at 12:01:30 AM UTC");
    expect(tooltip).toHaveTextContent("Timestamp1784160090");
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

    expect(screen.getByText("1 deploy")).toBeInTheDocument();
    expect(screen.getByText("dep-failed")).toBeInTheDocument();
    expect(screen.queryByText("dep-live")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "def5678");
    expect(screen.getByText("dep-live")).toBeInTheDocument();
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

  it("uses a complete count only after pagination is exhausted", async () => {
    state.deploys = [row()];
    state.hasMore = false;

    renderPage();

    expect(await screen.findByText("1 deploy")).toBeInTheDocument();
    expect(screen.queryByText(/loaded/)).not.toBeInTheDocument();
  });
});
