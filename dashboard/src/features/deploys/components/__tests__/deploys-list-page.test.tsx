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
    expect(screen.getAllByText("Duration 1m 30s")).toHaveLength(2);
    expect(
      screen.getByText(/Ship searchable deploy history/),
    ).toBeInTheDocument();
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
