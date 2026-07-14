import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ProjectSettingsPage } from "../project.$projectId.settings";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

const projectsState: {
  projects: ProjectView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = { projects: [], loading: false, error: undefined, refetch: vi.fn(async () => undefined) };
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => projectsState,
}));

const rename = vi.fn();
vi.mock("@/features/projects/hooks/use-rename-project", () => ({
  useRenameProject: () => ({ rename, busy: false }),
}));

const remove = vi.fn();
vi.mock("@/features/projects/hooks/use-delete-project", () => ({
  useDeleteProject: () => ({ remove, deleting: null }),
}));

function renderSettingsPage(projectId = "prj-1") {
  const rootRoute = createRootRoute();
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId/settings",
    component: ProjectSettingsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([settingsRoute]),
    history: createMemoryHistory({ initialEntries: [`/project/${projectId}/settings`] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  projectsState.projects = [];
  projectsState.loading = false;
  projectsState.error = undefined;
  rename.mockReset();
  remove.mockReset();
});

describe("ProjectSettingsPage", () => {
  it("renders the current project name", async () => {
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

    renderSettingsPage();

    expect(await screen.findByRole("heading", { name: "Project settings" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("storefront")).toBeInTheDocument();
  });

  it("shows a not-found state for an unknown project id", async () => {
    renderSettingsPage("prj-missing");

    expect(await screen.findByText("Project not found.")).toBeInTheDocument();
  });

  it("renames the project through the inline Edit control", async () => {
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
    rename.mockResolvedValue(true);
    const user = userEvent.setup();

    renderSettingsPage();

    await screen.findByDisplayValue("storefront");
    await user.click(screen.getByRole("button", { name: "Edit" }));

    const input = screen.getByDisplayValue("storefront");
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(rename).toHaveBeenCalledWith("prj-1", "new-name");
  });

  it("deletes the project through the danger-zone confirm dialog", async () => {
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
    remove.mockResolvedValue(true);
    const user = userEvent.setup();

    renderSettingsPage();

    await screen.findByDisplayValue("storefront");
    await user.click(screen.getByRole("button", { name: "Delete Project" }));

    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Delete Project" }));

    expect(remove).toHaveBeenCalledWith("prj-1", "storefront");
  });
});
