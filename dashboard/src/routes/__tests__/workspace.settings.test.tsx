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

// The danger zone gained a permanent resident in w5/m102: Leave workspace is
// available to every member, so the section now exists regardless of how many
// workspaces the caller has — only the DELETE card stays gated on having
// somewhere else to land.
vi.mock("@/features/team/components/leave-workspace-card", () => ({
  LeaveWorkspaceCard: () => <div>Leave workspace card</div>,
}));

const primaryWorkspace: WorkspaceView = {
  id: "tea-primary",
  name: "primary",
  plan: "hobby",
  role: "admin",
  createdAt: null,
};

async function renderPage() {
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

  // Load the split route before starting DOM assertions; a cold import can
  // exceed Testing Library's polling window on a busy CI runner.
  await router.load();
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  workspaceState.currentWorkspace = primaryWorkspace;
  workspaceState.workspaces = [primaryWorkspace];
  workspaceState.loading = false;
});

describe("WorkspaceSettingsPage", () => {
  it("hides the delete card for the user's only workspace but keeps Leave", async () => {
    await renderPage();

    expect(
      screen.getByRole("heading", { name: "Workspace settings" }),
    ).toBeInTheDocument();
    // Nowhere else to land, so deleting is unavailable — but a member of a
    // workspace they do not own must still be able to leave it (w5/m102).
    expect(screen.queryByText("Delete workspace card")).not.toBeInTheDocument();
    expect(screen.getByText("Leave workspace card")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Danger Zone" })).toHaveAttribute(
      "href",
      "#danger-zone",
    );
  });

  it("shows the delete section and navigation link when another workspace exists", async () => {
    workspaceState.workspaces = [
      primaryWorkspace,
      { ...primaryWorkspace, id: "tea-secondary", name: "secondary" },
    ];

    await renderPage();

    expect(screen.getByText("Delete workspace card")).toBeInTheDocument();
    expect(screen.getByText("Leave workspace card")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Danger Zone" })).toHaveAttribute(
      "href",
      "#danger-zone",
    );
  });
});
