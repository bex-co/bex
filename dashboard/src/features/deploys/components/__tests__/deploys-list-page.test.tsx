import { formatDateTime } from "@/common/lib/format";
import { beforeEach, describe, expect, it, vi } from "vitest";
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

beforeEach(() => {
  state.deploys = [];
  state.loading = false;
  state.loadingMore = false;
  state.error = undefined;
  state.hasMore = false;
  state.loadMore = vi.fn();
  statusCalls.length = 0;
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

  it("labels rows by terminal state — Canceled/Failed rows never read 'Deployed' (w6/051)", async () => {
    state.deploys = [
      // createdAt 00:00, finishedAt 00:01:30 from the row() defaults.
      row({ id: "dep-shipped", status: "live" }),
      row({
        id: "dep-canceled",
        status: "canceled",
        finishedAt: "2026-07-16T00:00:45Z",
        preDeployStatus: "",
      }),
      row({
        id: "dep-broken",
        status: "build_failed",
        finishedAt: "2026-07-16T00:02:09Z",
        preDeployStatus: "",
      }),
      row({
        id: "dep-waiting",
        status: "queued",
        startedAt: null,
        finishedAt: null,
        preDeployStatus: "",
      }),
    ];

    renderPage();

    // The live deploy is stamped with its finish time, not createdAt.
    const deployedAt = formatDateTime("2026-07-16T00:01:30Z")!;
    expect(
      await screen.findByText(`Deployed ${deployedAt}`),
    ).toBeInTheDocument();
    expect(
      screen.getByText(`Canceled ${formatDateTime("2026-07-16T00:00:45Z")!}`),
    ).toBeInTheDocument();
    expect(
      screen.getByText(`Failed ${formatDateTime("2026-07-16T00:02:09Z")!}`),
    ).toBeInTheDocument();
    // The queued deploy hasn't finished — it shows when it was created.
    expect(
      screen.getByText(`Created ${formatDateTime("2026-07-16T00:00:00Z")!}`),
    ).toBeInTheDocument();
    // Exactly one row earned the "Deployed" verb.
    expect(screen.getAllByText(/^Deployed /)).toHaveLength(1);
  });

  it("falls back to createdAt for a live deploy without a stored finish time", async () => {
    state.deploys = [
      row({ id: "dep-legacy", status: "live", finishedAt: null }),
    ];

    renderPage();

    expect(
      await screen.findByText(
        `Deployed ${formatDateTime("2026-07-16T00:00:00Z")!}`,
      ),
    ).toBeInTheDocument();
  });

  it("keeps the row action button outside the deploy-detail link (action-click isolation)", async () => {
    state.deploys = [row()];

    renderPage();

    const link = await screen.findByRole("link");
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
