import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Route } from "../workspace.settings";
import type { WorkspaceView } from "@/features/workspaces/types";

const workspaceState: {
  currentWorkspace: WorkspaceView | null;
  workspaces: WorkspaceView[];
  loading: boolean;
} = {
  currentWorkspace: null,
  workspaces: [],
  loading: false,
};

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => workspaceState,
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

vi.mock("@/features/workspaces/components/workspace-details-card", () => ({
  WorkspaceDetailsCard: () => <div>Workspace details card</div>,
}));

vi.mock("@/features/workspaces/components/delete-workspace-card", () => ({
  DeleteWorkspaceCard: () => <div>Delete workspace card</div>,
}));

vi.mock("@/features/team/components/team-panel", () => ({
  TeamPanel: () => <div>Team panel</div>,
}));

const primaryWorkspace: WorkspaceView = {
  id: "tea-primary",
  name: "primary",
  plan: "hobby",
  role: "admin",
  createdAt: null,
};

function renderPage() {
  const WorkspaceSettingsPage = Route.options.component;
  if (!WorkspaceSettingsPage) {
    throw new Error("workspace settings route component is missing");
  }
  const rootRoute = createRootRoute();
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workspace/settings",
    component: WorkspaceSettingsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([settingsRoute]),
    history: createMemoryHistory({
      initialEntries: ["/workspace/settings"],
    }),
    context: { client: {} as never, session: null },
  });

  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  workspaceState.currentWorkspace = primaryWorkspace;
  workspaceState.workspaces = [primaryWorkspace];
  workspaceState.loading = false;
});

describe("WorkspaceSettingsPage", () => {
  it("hides the delete section and navigation link for the user's only workspace", async () => {
    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Workspace settings" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Delete workspace card")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Danger Zone" }),
    ).not.toBeInTheDocument();
  });

  it("shows the delete section and navigation link when another workspace exists", async () => {
    workspaceState.workspaces = [
      primaryWorkspace,
      { ...primaryWorkspace, id: "tea-secondary", name: "secondary" },
    ];

    renderPage();

    expect(
      await screen.findByText("Delete workspace card"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Danger Zone" })).toHaveAttribute(
      "href",
      "#danger-zone",
    );
  });
});
