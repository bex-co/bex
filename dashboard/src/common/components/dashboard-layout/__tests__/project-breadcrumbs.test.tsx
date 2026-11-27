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

// w1/m683: an environment's settings page reads Project › Environment, as
// Render's trail does; the project page at an environment's URL keeps the
// project alone, as Render's does.
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [
      {
        id: "prj-shop",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ],
  }),
}));

const environmentsFor = vi.hoisted(() => vi.fn());
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: (projectId: string | null) => {
    environmentsFor(projectId);
    return {
      environments: projectId
        ? [{ id: "evm-staging", projectId, name: "Staging" }]
        : [],
    };
  },
}));

function renderAt(pathname: string) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const routes = [
    "/project/$projectId/environment/$environmentId",
    "/project/$projectId/environment/$environmentId/settings",
  ].map((path) =>
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

describe("project breadcrumbs", () => {
  it("adds the environment on its settings page", async () => {
    renderAt("/project/prj-shop/environment/evm-staging/settings");

    expect(
      await screen.findByRole("button", { name: /storefront/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Staging/ })).toBeInTheDocument();
  });

  it("keeps the project alone at the environment's URL", async () => {
    renderAt("/project/prj-shop/environment/evm-staging");

    expect(
      await screen.findByRole("button", { name: /storefront/ }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Staging/ })).toBeNull();
    // Nor does it read the environments it would not show.
    expect(environmentsFor).toHaveBeenLastCalledWith(null);
  });
});
