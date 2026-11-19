import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { ProjectSidebar } from "../project-sidebar";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { SavedSearchType } from "@/graphql/definitions";

const projectsState: {
  projects: ProjectView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  projects: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => undefined),
};
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => projectsState,
}));

// The project's observability views (w3/m505): Render's three defaults.
const emptyDocument = { filters: [], version: 1 };
const view = (id: string, name: string, type: SavedSearchType) =>
  ({
    __typename: "SavedSearch",
    id,
    name,
    type,
    filters: emptyDocument,
  }) satisfies SavedSearch;
const savedSearchesState: {
  views: SavedSearch[];
  loading: boolean;
  error: Error | undefined;
} = { views: [], loading: false, error: undefined };
const savedSearchesSpy = vi.fn();
vi.mock("@/features/projects/hooks/use-saved-searches", () => ({
  useSavedSearches: (projectId: string) => {
    savedSearchesSpy(projectId);
    return savedSearchesState;
  },
}));

function renderAt(pathname: string) {
  const rootRoute = createRootRoute();
  const overviewRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId/",
    component: () => (
      <SidebarProvider>
        <ProjectSidebar projectId="prj-1" />
      </SidebarProvider>
    ),
  });
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId/settings",
    component: () => (
      <SidebarProvider>
        <ProjectSidebar projectId="prj-1" />
      </SidebarProvider>
    ),
  });
  const viewRoutes = ["application", "request"].map((kind) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path: `/project/$projectId/observability/logs/${kind}`,
      component: () => (
        <SidebarProvider>
          <ProjectSidebar projectId="prj-1" />
        </SidebarProvider>
      ),
    }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      overviewRoute,
      settingsRoute,
      ...viewRoutes,
    ]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  savedSearchesSpy.mockClear();
  savedSearchesState.loading = false;
  savedSearchesState.views = [
    view("svs-app", "Application Logs", SavedSearchType.ApplicationLogs),
    view("svs-req", "Request Logs", SavedSearchType.RequestLogs),
    view("svs-metrics", "Service Metrics", SavedSearchType.Metrics),
  ];
  projectsState.projects = [
    {
      id: "prj-1",
      name: "storefront",
      ownerId: "tea-1",
      serviceIds: [],
      databaseIds: [],
      keyValueIds: [],
    },
  ];
});

describe("ProjectSidebar", () => {
  it("shows the project name and a back link to the workspace Overview from the project's own page", async () => {
    renderAt("/project/prj-1");

    expect(await screen.findByText("storefront")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Dashboard/ })).toHaveAttribute(
      "href",
      "/",
    );
    expect(screen.getByRole("link", { name: "Overview" })).toHaveAttribute(
      "data-active",
      "true",
    );
  });

  it("shows a back link to the project's Overview from the Settings page", async () => {
    renderAt("/project/prj-1/settings");

    expect(await screen.findByText("storefront")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Project/ })).toHaveAttribute(
      "href",
      "/project/prj-1",
    );
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute(
      "data-active",
      "true",
    );
  });

  it("lists the project's log views between Overview and Manage, without Service Metrics (w3/m505)", async () => {
    renderAt("/project/prj-1");

    expect(await screen.findByText("Observability")).toBeInTheDocument();
    expect(savedSearchesSpy).toHaveBeenCalledWith("prj-1");
    expect(
      screen.getByRole("link", { name: "Application Logs" }),
    ).toHaveAttribute(
      "href",
      "/project/prj-1/observability/logs/application?savedSearchId=svs-app",
    );
    expect(screen.getByRole("link", { name: "Request Logs" })).toHaveAttribute(
      "href",
      "/project/prj-1/observability/logs/request?savedSearchId=svs-req",
    );
    // Render keeps the METRICS view but does not list it.
    expect(screen.queryByText("Service Metrics")).not.toBeInTheDocument();
    const labels = screen
      .getAllByText(/^(Observability|Manage)$/)
      .map((el) => el.textContent);
    expect(labels).toEqual(["Observability", "Manage"]);
  });

  it("marks the open view active, not Overview, and links back to the project", async () => {
    renderAt(
      "/project/prj-1/observability/logs/request?savedSearchId=svs-req&r=4h",
    );

    expect(
      await screen.findByRole("link", { name: "Request Logs" }),
    ).toHaveAttribute("data-active", "true");
    expect(
      screen.getByRole("link", { name: "Application Logs" }),
    ).toHaveAttribute("data-active", "false");
    expect(screen.getByRole("link", { name: "Overview" })).toHaveAttribute(
      "data-active",
      "false",
    );
    expect(screen.getByRole("link", { name: /Project/ })).toHaveAttribute(
      "href",
      "/project/prj-1",
    );
  });

  it("reserves the two default views while the list loads", async () => {
    savedSearchesState.loading = true;
    savedSearchesState.views = [];
    const { container } = renderAt("/project/prj-1");

    expect(await screen.findByText("Observability")).toBeInTheDocument();
    expect(
      container.querySelectorAll('[data-sidebar="menu-skeleton"]'),
    ).toHaveLength(2);
    expect(
      screen.queryByRole("link", { name: "Application Logs" }),
    ).not.toBeInTheDocument();
  });
});
