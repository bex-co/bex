import { render, screen } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";
import { DashboardBreadcrumbs } from "../dashboard-breadcrumbs";

// w4/144: a Postgres or Key Value page showed only "Key Value red-…" even when
// the store sat in a project and environment.
const detail = vi.hoisted(() => ({
  database: { id: "dpg-main", name: "primary-db" },
  keyValue: { id: "red-cache", name: "session-cache" },
}));

vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: () => ({ database: detail.database }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: () => ({ keyValue: detail.keyValue }),
}));

vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [
      {
        id: "prj-storefront",
        name: "Storefront",
        ownerId: "tea-one",
        serviceIds: [],
        databaseIds: ["dpg-main"],
        keyValueIds: [],
      },
    ],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: (projectId: string | null) => ({
    environments: projectId
      ? [
          {
            id: "env-staging",
            projectId: "prj-storefront",
            name: "Staging",
            serviceIds: [],
            databaseIds: ["dpg-main"],
            keyValueIds: [],
          },
        ]
      : [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

function renderAt(pathname: string) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const routes = ["/databases/$databaseId", "/keyvalue/$keyValueId"].map(
    (path) =>
      createRoute({
        getParentRoute: () => root,
        path,
        component: DashboardBreadcrumbs,
      }),
  );
  const router = createRouter({
    routeTree: root.addChildren(routes),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: {} as never,
  });
  render(<RouterProvider router={router} />);
}

describe("datastore breadcrumbs (w4/144)", () => {
  it("places a grouped Postgres under its project and environment, by name", async () => {
    renderAt("/databases/dpg-main");

    const nav = await screen.findByRole("navigation", { name: "Breadcrumbs" });
    expect(
      screen.getByRole("button", { name: /Storefront/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Staging/ })).toBeInTheDocument();
    expect(nav).toHaveTextContent("primary-db");
    expect(nav).not.toHaveTextContent("dpg-main");
  });

  it("keeps an ungrouped Key Value's type crumb, with its name as the leaf", async () => {
    renderAt("/keyvalue/red-cache");

    const nav = await screen.findByRole("navigation", { name: "Breadcrumbs" });
    expect(nav).toHaveTextContent("Key Value");
    expect(nav).toHaveTextContent("session-cache");
    expect(nav).not.toHaveTextContent("red-cache");
    expect(screen.queryByRole("button", { name: /Storefront/ })).toBeNull();
  });
});
