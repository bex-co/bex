import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  billingCapabilities,
  contributorCapabilities,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

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

// New View's modal (w3/m507) has its own test; here it shows what it opens.
vi.mock("@/features/logs/components/edit-view-dialog", () => ({
  EditViewDialog: ({
    target,
  }: {
    target: { kind: string; type: string; name: string } | null;
  }) =>
    target ? (
      <div role="dialog">
        {target.kind} {target.type} named {target.name}
      </div>
    ) : null,
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
  const viewRoutes = ["logs/application", "logs/request", "metrics"].map(
    (page) =>
      createRoute({
        getParentRoute: () => rootRoute,
        path: `/project/$projectId/observability/${page}`,
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
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
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

  it("lists the project's views between Overview and Manage, Service Metrics too (w3/m505, w3/m508)", async () => {
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
    // Render's docs list Service Metrics among the default views; its
    // reference workspace hid it on 2026-11-19 (w3/m508).
    expect(
      screen.getByRole("link", { name: "Service Metrics" }),
    ).toHaveAttribute(
      "href",
      "/project/prj-1/observability/metrics?savedSearchId=svs-metrics",
    );
    const labels = screen
      .getAllByText(/^(Observability|Manage)$/)
      .map((el) => el.textContent);
    expect(labels).toEqual(["Observability", "Manage"]);
  });

  // w9/m621, w9/145: Render's role table gives the Billing role neither
  // "View service metrics" nor "View service logs", and a Viewer metrics but
  // no logs; a Contributor has both.
  const viewLinks = () =>
    ["Application Logs", "Request Logs", "Service Metrics"].filter((name) =>
      screen.queryByRole("link", { name }),
    );

  it("lists no view to a Billing member, under the Observability heading", async () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    renderAt("/project/prj-1");

    expect(await screen.findByText("Observability")).toBeInTheDocument();
    expect(viewLinks()).toEqual([]);
  });

  it("lists the Service Metrics view alone to a Viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    renderAt("/project/prj-1");

    expect(
      await screen.findByRole("link", { name: "Service Metrics" }),
    ).toHaveAttribute(
      "href",
      "/project/prj-1/observability/metrics?savedSearchId=svs-metrics",
    );
    expect(viewLinks()).toEqual(["Service Metrics"]);
  });

  it("lists every view to a Contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    renderAt("/project/prj-1");

    await screen.findByRole("link", { name: "Application Logs" });
    expect(viewLinks()).toEqual([
      "Application Logs",
      "Request Logs",
      "Service Metrics",
    ]);
  });

  it("lists every view while capabilities are still checking", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        loading: true,
        loaded: false,
        canViewActivity: false,
        canViewLogs: false,
      }),
    );
    renderAt("/project/prj-1");

    await screen.findByRole("link", { name: "Application Logs" });
    expect(viewLinks()).toEqual([
      "Application Logs",
      "Request Logs",
      "Service Metrics",
    ]);
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

  it("reserves the three default views while the list loads", async () => {
    savedSearchesState.loading = true;
    savedSearchesState.views = [];
    const { container } = renderAt("/project/prj-1");

    expect(await screen.findByText("Observability")).toBeInTheDocument();
    expect(
      container.querySelectorAll('[data-sidebar="menu-skeleton"]'),
    ).toHaveLength(3);
    expect(
      screen.queryByRole("link", { name: "Application Logs" }),
    ).not.toBeInTheDocument();
  });

  it("offers New View by type, opening the editor with the type's free name (w3/m507)", async () => {
    const user = userEvent.setup();
    renderAt("/project/prj-1");

    await user.click(await screen.findByRole("button", { name: "New View" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).getByText("Type")).toBeInTheDocument();
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Application Logs", "Request Logs", "Service Metrics"]);
    await user.click(
      within(menu).getByRole("menuitem", { name: "Application Logs" }),
    );
    // The project already has "Application Logs": the new one is numbered.
    expect(screen.getByRole("dialog")).toHaveTextContent(
      "create APPLICATION_LOGS named Application Logs 2",
    );
    // Service Metrics is a type too (w3/m508).
    await user.click(screen.getByRole("button", { name: "New View" }));
    await user.click(
      within(screen.getByRole("menu")).getByRole("menuitem", {
        name: "Service Metrics",
      }),
    );
    expect(screen.getByRole("dialog")).toHaveTextContent(
      "create METRICS named Service Metrics 2",
    );
  });

  it("disables New View with the role's reason for a member who may not edit", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canCreate: false }),
    );
    const user = userEvent.setup();
    renderAt("/project/prj-1");

    const button = await screen.findByRole("button", { name: "New View" });
    expect(button).toHaveAttribute("aria-disabled", "true");
    // The reason is keyboard-reachable: the button stays focusable.
    act(() => button.focus());
    expect(
      await screen.findAllByText(
        "Your role can’t make this change. Ask a workspace admin.",
      ),
    ).not.toHaveLength(0);
    await user.click(button);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});
