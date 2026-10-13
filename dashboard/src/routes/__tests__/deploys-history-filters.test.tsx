import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Link,
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Route as ServiceDeploysRoute } from "../services.$serviceId.deploys.index";
import { Route as StaticDeploysRoute } from "../static.$serviceId.deploys.index";
import type { DeployRow } from "@/features/deploys/hooks/use-deploys";

// w4/284: the deploy history kept its search and status in component state, so
// opening a deploy and pressing Back returned to an unfiltered list. Both
// production mounts are driven through their real route components and
// validators.

const statusCalls = vi.hoisted(
  () => [] as { serviceId: string; statuses: string[] }[],
);
const rows = vi.hoisted(() => ({ deploys: [] as unknown[] }));

vi.mock("@/features/deploys/hooks/use-deploys", () => ({
  useDeploys: (serviceId: string, statuses: string[]) => {
    statusCalls.push({ serviceId, statuses });
    return {
      deploys: rows.deploys,
      loading: false,
      loadingMore: false,
      error: undefined,
      hasMore: false,
      loadMore: vi.fn(),
    };
  },
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDeployActions: () => ({ status: "checking", refresh: vi.fn() }),
}));
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({ service: { repo: null }, loading: false }),
}));
vi.mock("@/features/deploys/components/deploy-actions", () => ({
  DeployActions: () => null,
}));

function row(id: string, status: string): DeployRow {
  return {
    id,
    status,
    trigger: "api",
    image: "registry.example.com/web:1",
    rollbackOf: "",
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-07-16T00:00:00Z",
    updatedAt: "2026-07-16T00:01:30Z",
    startedAt: "2026-07-16T00:00:00Z",
    finishedAt: "2026-07-16T00:01:30Z",
    preDeployStatus: "",
    failureReason: "",
    cancelReason: "",
    stallReason: "",
  } as DeployRow;
}

const MOUNTS = [
  { name: "service", route: ServiceDeploysRoute, base: "/services" },
  { name: "static site", route: StaticDeploysRoute, base: "/static" },
] as const;

function renderAt(mount: (typeof MOUNTS)[number], href: string) {
  const root = createRootRoute({
    component: () => (
      <>
        <Link to="/services/$serviceId/deploys" params={{ serviceId: "web" }}>
          Deploys nav
        </Link>
        <Outlet />
      </>
    ),
  });
  const list = createRoute({
    getParentRoute: () => root,
    path: `${mount.base}/$serviceId/deploys`,
    validateSearch: mount.route.options.validateSearch,
    component: mount.route.options.component,
  });
  // The list's row links always target the /services detail route.
  const detail = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => <h1>Deploy detail</h1>,
  });
  const routes =
    mount.base === "/services"
      ? [list, detail]
      : [
          list,
          detail,
          createRoute({
            getParentRoute: () => root,
            path: "/services/$serviceId/deploys",
            component: () => <h1>Service deploys</h1>,
          }),
        ];
  const router = createRouter({
    routeTree: root.addChildren(routes),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

const searchBox = () =>
  screen.findByRole("textbox", { name: "Search loaded deploys and commits" });
const statusFilter = () =>
  screen.getByRole("combobox", { name: "Filter by status" });
async function pickStatus(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) {
  fireEvent.keyDown(statusFilter(), { key: "ArrowDown" });
  await user.click(await screen.findByRole("option", { name }));
}

beforeEach(() => {
  statusCalls.length = 0;
  rows.deploys = [row("dep-old", "deactivated"), row("dep-new", "live")];
});

describe.each(MOUNTS)("$name deploy history filters (w4/284)", (mount) => {
  const list = `${mount.base}/web/deploys`;

  it("returns to the filtered history on Back from a deploy, and to the deploy on Forward", async () => {
    const user = userEvent.setup();
    const router = renderAt(mount, list);
    await user.type(await searchBox(), "DEP-OLD");
    await pickStatus(user, "Deactivated");
    await waitFor(() =>
      expect(router.state.location.search).toEqual({
        status: "deactivated",
        q: "DEP-OLD",
      }),
    );
    expect(screen.queryByText("dep-new")).not.toBeInTheDocument();
    // Filter edits replace the entry: Back must leave the list, not undo typing.
    expect(router.history.length).toBe(1);

    await user.click(screen.getByText("dep-old"));
    expect(
      await screen.findByRole("heading", { name: "Deploy detail" }),
    ).toBeInTheDocument();

    act(() => router.history.back());
    expect(await searchBox()).toHaveValue("DEP-OLD");
    expect(statusFilter()).toHaveTextContent("Deactivated");
    expect(statusCalls.at(-1)?.statuses).toEqual(["deactivated"]);
    expect(screen.getByText("dep-old")).toBeInTheDocument();
    expect(screen.queryByText("dep-new")).not.toBeInTheDocument();

    act(() => router.history.forward());
    expect(
      await screen.findByRole("heading", { name: "Deploy detail" }),
    ).toBeInTheDocument();
  });

  it("opens a copied filtered link filtered, and a plain link unfiltered without touching the URL", async () => {
    const filtered = renderAt(mount, `${list}?status=live&q=dep-new`);
    expect(await searchBox()).toHaveValue("dep-new");
    expect(statusFilter()).toHaveTextContent("Live");
    expect(statusCalls[0]?.statuses).toEqual(["live"]);
    expect(filtered.state.location.search).toEqual({
      status: "live",
      q: "dep-new",
    });
  });

  it("ignores a status the history does not know and keeps numeric search text", async () => {
    const router = renderAt(mount, `${list}?status=bogus&q=123`);
    expect(await searchBox()).toHaveValue("123");
    expect(statusFilter()).toHaveTextContent("All statuses");
    expect(statusCalls.every((call) => call.statuses.length === 0)).toBe(true);
    expect(router.state.location.search).toEqual({ q: "123" });
  });

  it("clears a filter's key from the URL when the filter is cleared", async () => {
    const user = userEvent.setup();
    const router = renderAt(mount, `${list}?status=live&q=dep`);
    await user.clear(await searchBox());
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ status: "live" }),
    );
    await pickStatus(user, "All statuses");
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(statusCalls.at(-1)?.statuses).toEqual([]);
  });
});

describe("service deploy history navigation (w4/284)", () => {
  const mount = MOUNTS[0];

  it("drops its filters when a plain link to the same history is followed", async () => {
    const user = userEvent.setup();
    const router = renderAt(mount, "/services/web/deploys?status=live&q=dep");
    expect(await searchBox()).toHaveValue("dep");
    await user.click(screen.getByText("Deploys nav"));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    await waitFor(async () => expect(await searchBox()).toHaveValue(""));
    expect(statusFilter()).toHaveTextContent("All statuses");
    expect(statusCalls.at(-1)?.statuses).toEqual([]);
  });

  it("keeps typing made faster than the URL can echo it", async () => {
    const user = userEvent.setup();
    const router = renderAt(mount, "/services/web/deploys");
    await user.type(await searchBox(), "dep-old-and-more");
    expect(await searchBox()).toHaveValue("dep-old-and-more");
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ q: "dep-old-and-more" }),
    );
    expect(await searchBox()).toHaveValue("dep-old-and-more");
  });

  it("does not carry one service's filters to another service's history", async () => {
    const router = renderAt(mount, "/services/web/deploys?status=live&q=dep");
    expect(await searchBox()).toHaveValue("dep");
    await act(() =>
      router.navigate({
        to: "/services/$serviceId/deploys",
        params: { serviceId: "api" },
      }),
    );
    await waitFor(() => expect(statusCalls.at(-1)?.serviceId).toBe("api"));
    expect(await searchBox()).toHaveValue("");
    expect(statusCalls.at(-1)?.statuses).toEqual([]);
  });
});
