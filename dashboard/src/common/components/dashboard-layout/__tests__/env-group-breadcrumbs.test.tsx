import { render, screen } from "@testing-library/react";
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

// w4/212: an environment group's topbar read "Environment Groups > evg-…",
// the id hidden on mobile, with no project even when the group was scoped to
// a project's environment.
const state = vi.hoisted(() => ({
  group: null as null | {
    id: string;
    name: string;
    environmentId: string | null;
  },
  indexReady: true,
}));

vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroup: () => ({ group: state.group }),
}));

vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => {
  const environment = {
    id: "evm-staging",
    projectId: "prj-storefront",
    name: "Staging",
    serviceIds: [],
    databaseIds: [],
    keyValueIds: [],
  };
  return {
    useEnvGroupScopeIndex: () => ({
      projects: state.indexReady
        ? [
            {
              id: "prj-storefront",
              name: "Storefront",
              ownerId: "tea-one",
              serviceIds: [],
              databaseIds: [],
              keyValueIds: [],
            },
          ]
        : [],
      environments: state.indexReady ? [environment] : [],
      byId: new Map(state.indexReady ? [[environment.id, environment]] : []),
      ready: state.indexReady,
    }),
  };
});

function renderAt(pathname: string) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const route = createRoute({
    getParentRoute: () => root,
    path: "/env-groups/$groupId",
    component: DashboardBreadcrumbs,
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: {} as never,
  });
  render(<RouterProvider router={router} />);
}

beforeEach(() => {
  state.indexReady = true;
  state.group = null;
});

describe("environment-group breadcrumbs (w4/212)", () => {
  it("places a scoped group under its project and environment, by name", async () => {
    state.group = {
      id: "evg-1",
      name: "shared-config",
      environmentId: "evm-staging",
    };
    renderAt("/env-groups/evg-1");

    const nav = await screen.findByRole("navigation", { name: "Breadcrumbs" });
    expect(
      screen.getByRole("button", { name: /Storefront/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Staging/ })).toBeInTheDocument();
    expect(nav).toHaveTextContent("shared-config");
    expect(nav).not.toHaveTextContent("evg-1");
  });

  it("keeps a workspace group under Environment Groups, by name", async () => {
    state.group = { id: "evg-2", name: "workspace-clone", environmentId: null };
    renderAt("/env-groups/evg-2");

    const nav = await screen.findByRole("navigation", { name: "Breadcrumbs" });
    expect(
      screen.getByRole("link", { name: /Environment Groups/ }),
    ).toHaveAttribute("href", "/env-groups");
    expect(nav).toHaveTextContent("workspace-clone");
    expect(nav).not.toHaveTextContent("evg-2");
  });

  it("never guesses a project or shows the id while placement or name is unresolved", async () => {
    state.indexReady = false;
    state.group = {
      id: "evg-1",
      name: "shared-config",
      environmentId: "evm-staging",
    };
    renderAt("/env-groups/evg-1");
    const nav = await screen.findByRole("navigation", { name: "Breadcrumbs" });
    expect(screen.queryByRole("button", { name: /Storefront/ })).toBeNull();
    expect(nav).toHaveTextContent("shared-config");

    state.group = null;
    renderAt("/env-groups/evg-1");
    const navs = await screen.findAllByRole("navigation", {
      name: "Breadcrumbs",
    });
    expect(navs.at(-1)).not.toHaveTextContent("evg-1");
  });
});
