import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DashboardBreadcrumbs } from "../dashboard-breadcrumbs";
import { GlobalSearch } from "../global-search";
import type { ServiceView } from "@/features/services/types";

const mocks = vi.hoisted(() => ({
  service: {
    id: "srv-api",
    name: "storefront-api",
    type: "web_service",
  },
  services: [
    { id: "srv-api", name: "storefront-api", type: "web_service" },
    { id: "srv-worker", name: "queue-worker", type: "background_worker" },
    { id: "srv-billing", name: "billing-api", type: "web_service" },
  ],
  projects: [
    {
      id: "prj-storefront",
      name: "Storefront",
      ownerId: "tea-one",
      serviceIds: ["srv-api", "srv-worker"],
      databaseIds: [],
      keyValueIds: [],
    },
    {
      id: "prj-billing",
      name: "Billing",
      ownerId: "tea-one",
      serviceIds: ["srv-billing"],
      databaseIds: [],
      keyValueIds: [],
    },
  ],
  environments: [
    {
      id: "env-production",
      projectId: "prj-storefront",
      name: "Production",
      ownerId: "tea-one",
      createdAt: null,
      serviceIds: ["srv-api"],
      databaseIds: [],
      keyValueIds: [],
      envGroupIds: [],
      protectedStatus: "unprotected",
      networkIsolationEnabled: false,
      ipAllowListEntries: [],
    },
  ],
}));

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: mocks.service as ServiceView,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));

vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: mocks.services as ServiceView[],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));

vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: mocks.projects,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));

vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: mocks.environments,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));

vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({
    databases: [{ id: "dpg-main", name: "primary-db" }],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({
    keyValues: [{ id: "red-cache", name: "session-cache" }],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: [{ id: "evg-shared", name: "Shared secrets" }],
    loading: false,
    error: undefined,
  }),
}));

function buildRouter(pathname: string, component: () => React.ReactNode) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const index = createRoute({
    getParentRoute: () => root,
    path: "/",
    component,
  });
  const service = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/$",
    component,
  });
  const project = createRoute({
    getParentRoute: () => root,
    path: "/project/$projectId",
    component: () => null,
  });
  const database = createRoute({
    getParentRoute: () => root,
    path: "/databases/$databaseId",
    component: () => null,
  });
  const keyValue = createRoute({
    getParentRoute: () => root,
    path: "/keyvalue/$keyValueId",
    component: () => null,
  });
  const envGroup = createRoute({
    getParentRoute: () => root,
    path: "/env-groups/$groupId",
    component: () => null,
  });
  return createRouter({
    routeTree: root.addChildren([
      index,
      service,
      project,
      database,
      keyValue,
      envGroup,
    ]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: {} as never,
  });
}

beforeEach(() => {
  mocks.service.id = "srv-api";
  mocks.service.name = "storefront-api";
});

describe("dashboard topbar navigation", () => {
  it("shows a switchable project / environment / service hierarchy", async () => {
    const user = userEvent.setup();
    const router = buildRouter(
      "/services/srv-api/settings",
      DashboardBreadcrumbs,
    );
    render(<RouterProvider router={router} />);

    expect(
      await screen.findByRole("navigation", { name: "Breadcrumbs" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Storefront/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Production/ }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /storefront-api/ }));
    expect(
      await screen.findByRole("menuitem", { name: /queue-worker/ }),
    ).toHaveAttribute("href", "/services/srv-worker");
    expect(
      screen.queryByRole("menuitem", { name: /billing-api/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "All resources" }),
    ).toHaveAttribute("href", "/");
  });

  it("opens workspace-wide search with the keyboard and navigates to a resource", async () => {
    const user = userEvent.setup();
    const router = buildRouter("/", GlobalSearch);
    render(<RouterProvider router={router} />);

    await screen.findByRole("button", { name: "Search" });
    fireEvent.keyDown(document, { key: "k", metaKey: true });
    expect(
      await screen.findByPlaceholderText("Search pages and resources…"),
    ).toBeInTheDocument();
    expect(screen.getByText("primary-db")).toBeInTheDocument();
    expect(screen.getByText("session-cache")).toBeInTheDocument();

    await user.click(screen.getByText("storefront-api"));
    expect(router.state.location.pathname).toBe("/services/srv-api");
  });
});
