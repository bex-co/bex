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

vi.mock(
  "@/features/agent-environment/components/agent-environment-card",
  () => ({
    AgentEnvironmentCard: ({ ownerId }: { ownerId: string }) => (
      <div>Agent environment card {ownerId}</div>
    ),
  }),
);

vi.mock("@/features/agent-environment/components/agent-secrets-card", () => ({
  AgentSecretsCard: ({ ownerId }: { ownerId: string }) => (
    <div>Agent secrets card {ownerId}</div>
  ),
}));

vi.mock("@/features/agent-environment/components/agent-skills-card", () => ({
  AgentSkillsCard: ({ ownerId }: { ownerId: string }) => (
    <div>Agent skills card {ownerId}</div>
  ),
}));

vi.mock(
  "@/features/notifications/components/workspace-notifications-card",
  () => ({
    WorkspaceNotificationsCard: ({ ownerId }: { ownerId: string }) => (
      <div>Workspace notifications card {ownerId}</div>
    ),
  }),
);

vi.mock(
  "@/features/notifications/components/notification-overrides-card",
  () => ({
    NotificationOverridesCard: ({ ownerId }: { ownerId: string }) => (
      <div>Notification overrides card {ownerId}</div>
    ),
  }),
);

vi.mock(
  "@/features/registry-credentials/components/registry-credentials-panel",
  () => ({
    RegistryCredentialsPanel: () => <div>Registry credentials panel</div>,
  }),
);

const auditState = { forbidden: false };
vi.mock("@/features/audit/hooks/use-audit-log", () => ({
  useAuditLog: () => auditState,
}));
vi.mock("@/features/audit/components/audit-log-panel", () => ({
  AuditLogPanel: () => <div>Audit log panel</div>,
}));

// The danger zone gained a permanent resident in w5/m102: Leave workspace is
// available to every member, so the section now exists regardless of how many
// workspaces the caller has — only the DELETE card stays gated on having
// somewhere else to land.
vi.mock("@/features/team/components/leave-workspace-card", () => ({
  LeaveWorkspaceCard: () => <div>Leave workspace card</div>,
}));

// Build Pipeline (w6/m309) holds billing settings: billing managers only.
const capabilityState = { canManageBilling: true };
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => capabilityState,
}));
vi.mock("@/features/usage/components/build-pipeline-card", () => ({
  BuildPipelineCard: () => <div>Build pipeline card</div>,
}));

const primaryWorkspace: WorkspaceView = {
  id: "tea-primary",
  name: "primary",
  slug: "primary",
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
  capabilityState.canManageBilling = true;
  auditState.forbidden = false;
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

  it("renders the Agents section for the current workspace", async () => {
    await renderPage();

    expect(
      screen.getByText("Agent environment card tea-primary"),
    ).toBeInTheDocument();
    // ADR098 D8: the workspace agent secrets sit in the same section.
    expect(
      screen.getByText("Agent secrets card tea-primary"),
    ).toBeInTheDocument();
    // ADR099 D6: and the workspace skills.
    expect(
      screen.getByText("Agent skills card tea-primary"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Agents" })).toHaveAttribute(
      "href",
      "#agents",
    );
  });

  it("renders the Notifications section for the current workspace (w3/m271)", async () => {
    await renderPage();

    expect(
      screen.getByText("Workspace notifications card tea-primary"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Notifications" })).toHaveAttribute(
      "href",
      "#notifications",
    );
  });

  // w3/m305: Render keeps workspace-scoped registry credentials, the
  // notification overrides list and the audit log in Workspace Settings.
  it("renders Registry Credentials, Audit Logs and Notification Overrides here", async () => {
    await renderPage();

    expect(screen.getByText("Registry credentials panel")).toBeInTheDocument();
    expect(screen.getByText("Audit log panel")).toBeInTheDocument();
    expect(
      screen.getByText("Notification overrides card tea-primary"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Registry Credentials" }),
    ).toHaveAttribute("href", "#registry-credentials");
    expect(screen.getByRole("link", { name: "Audit Log" })).toHaveAttribute(
      "href",
      "#audit-logs",
    );
  });

  it("hides Audit Logs and its link from a non-admin, before any 403", async () => {
    workspaceState.currentWorkspace = {
      ...primaryWorkspace,
      role: "developer",
    };
    await renderPage();

    expect(screen.queryByText("Audit log panel")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Audit Log" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Registry credentials panel")).toBeInTheDocument();
  });

  it("shows Build Pipeline to billing managers only (w6/m309)", async () => {
    const { unmount } = await renderPage();
    expect(screen.getByText("Build pipeline card")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Build Pipeline" }),
    ).toHaveAttribute("href", "#build-pipeline");
    unmount();

    capabilityState.canManageBilling = false;
    await renderPage();
    expect(screen.queryByText("Build pipeline card")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Build Pipeline" }),
    ).not.toBeInTheDocument();
  });

  it("hides Audit Logs when the audit log answers forbidden", async () => {
    auditState.forbidden = true;
    await renderPage();

    expect(screen.queryByText("Audit log panel")).not.toBeInTheDocument();
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
