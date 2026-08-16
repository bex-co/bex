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
  const router = createRouter({
    routeTree: rootRoute.addChildren([overviewRoute, settingsRoute]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
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
});
