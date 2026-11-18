import type { ReactNode } from "react";
import { render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Route } from "../workspace.settings";
import Cookies from "js-cookie";
import { WorkspaceSettingsPageSkeleton } from "@/common/components/route-skeletons";
import {
  persistWorkspaceShape,
  workspaceShape,
  WORKSPACE_SHAPE_KEY,
} from "@/features/workspaces/lib/selection";
import { useWorkspaceSettingsSkeletonSections } from "@/features/workspaces/hooks/use-workspace-settings-skeleton-sections";
import type { WorkspaceView } from "@/features/workspaces/types";

const workspaceState: {
  currentWorkspace: WorkspaceView | null;
  currentWorkspaceId: string | null;
  workspaces: WorkspaceView[];
  loading: boolean;
} = {
  currentWorkspace: null,
  currentWorkspaceId: null,
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
  WorkspaceDetailsCard: ({ changePlanOpen }: { changePlanOpen: boolean }) => (
    <div>
      Workspace details card
      {changePlanOpen ? <span>Change plan dialog open</span> : null}
    </div>
  ),
}));

// Inbound IP Rules (w9/m388): the card's own behavior is pinned in its test;
// here only its placement and the change-plan wiring.
vi.mock("@/features/workspaces/components/workspace-ip-rules-card", () => ({
  WorkspaceIPRulesCard: ({
    workspace,
    onChangePlan,
  }: {
    workspace: WorkspaceView;
    onChangePlan: () => void;
  }) => (
    <div>
      Inbound IP rules card {workspace.id}
      <button type="button" onClick={onChangePlan}>
        Upgrade from IP rules
      </button>
    </div>
  ),
}));

// Two-factor authentication (w22/m402) and Google login (w7/m465): both cards
// are every member's; their own admin/non-admin behavior is tested beside them.
vi.mock(
  "@/features/workspaces/components/workspace-login-requirement-card",
  () => ({
    WorkspaceLoginRequirementCard: ({
      requirement,
      workspace,
    }: {
      requirement: "twoFactor" | "googleLogin";
      workspace: WorkspaceView;
    }) => (
      <div>
        {requirement === "twoFactor" ? "Two-factor" : "Google login"} card{" "}
        {workspace.id}
      </div>
    ),
  }),
);

// Workspace privacy (w7/m464): the card's own behavior is pinned in its test;
// here only who sees it and where.
vi.mock("@/features/workspaces/components/workspace-privacy-card", () => ({
  WorkspacePrivacyCard: ({ workspace }: { workspace: WorkspaceView }) => (
    <div>Workspace privacy card {workspace.id}</div>
  ),
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

vi.mock(
  "@/features/agent-environment/components/agent-repo-environments-card",
  () => ({
    AgentRepoEnvironmentsCard: ({ ownerId }: { ownerId: string }) => (
      <div>Repository environments card {ownerId}</div>
    ),
  }),
);

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
    RegistryCredentialsPanel: () => (
      <div id="registry-credentials">Registry credentials panel</div>
    ),
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
// Workspace privacy (w7/m464) is an admin's (`can_manage`).
const capabilityState = { canManageBilling: true, canManage: true };
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => capabilityState,
}));
vi.mock("@/features/usage/components/build-pipeline-card", () => ({
  BuildPipelineCard: () => <div>Build pipeline card</div>,
}));
// Overlapping Deploy Policy (w4/653): the card's behavior is pinned in its
// own test; here only its placement.
vi.mock(
  "@/features/workspaces/components/overlapping-deploy-policy-card",
  () => ({
    OverlappingDeployPolicyCard: ({ workspaceId }: { workspaceId: string }) => (
      <div>Overlapping deploy policy card {workspaceId}</div>
    ),
  }),
);

/** The rail's section anchors, in order. */
function railSections(container: HTMLElement): string[] {
  return [...container.querySelectorAll("nav a[href^='#']")].map((link) =>
    link.getAttribute("href")!.slice(1),
  );
}

/** A pending frame's sections: its outermost regions below the header and
 *  the rail, in order. */
function frameSections(container: HTMLElement): string[] {
  return [...container.querySelectorAll<HTMLElement>("[data-skeleton-region]")]
    .filter(
      (region) => !region.parentElement?.closest("[data-skeleton-region]"),
    )
    .map((region) => region.dataset.skeletonRegion!)
    .filter((name) => name !== "page-header" && name !== "navigation");
}

const primaryWorkspace: WorkspaceView = {
  id: "tea-primary",
  name: "primary",
  slug: "primary",
  plan: "hobby",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
};

async function renderPage(WorkspaceSettingsPage = Route.options.component) {
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

/** primaryWorkspace as a secondary of an organization (w7/m464). */
const orgWorkspace: WorkspaceView = {
  ...primaryWorkspace,
  plan: "scale",
  organization: {
    id: "org-1",
    name: "Acme",
    primaryWorkspaceId: "tea-hq",
    requireTwoFactor: false,
    requireGoogleLogin: false,
  },
  primary: false,
  privacy: "invite_only",
};

beforeEach(() => {
  capabilityState.canManageBilling = true;
  capabilityState.canManage = true;
  auditState.forbidden = false;
  workspaceState.currentWorkspace = primaryWorkspace;
  workspaceState.currentWorkspaceId = primaryWorkspace.id;
  workspaceState.workspaces = [primaryWorkspace];
  workspaceState.loading = false;
  Cookies.remove(WORKSPACE_SHAPE_KEY, { path: "/" });
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
    // ADR109: the repository environments sit beside the secrets their
    // setup steps run with.
    const secrets = screen.getByText("Agent secrets card tea-primary");
    const environments = screen.getByText(
      "Repository environments card tea-primary",
    );
    expect(
      secrets.compareDocumentPosition(environments) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // ADR099 D6: and the workspace skills.
    expect(
      screen.getByText("Agent skills card tea-primary"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Agents" })).toHaveAttribute(
      "href",
      "#agents",
    );
  });

  it("renders the Two-factor authentication card before Team, for every role (w22/m402)", async () => {
    workspaceState.currentWorkspace = {
      ...primaryWorkspace,
      role: "developer",
    };
    await renderPage();

    const card = screen.getByText("Two-factor card tea-primary");
    expect(card).toBeInTheDocument();
    expect(card.closest("section")).toHaveAttribute("id", "two-factor");
    expect(
      card.compareDocumentPosition(screen.getByText("Team panel")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "Two-factor authentication" }),
    ).toHaveAttribute("href", "#two-factor");
  });

  it("renders the Google login card after Two-factor and before Team, for every role (w7/m465)", async () => {
    workspaceState.currentWorkspace = {
      ...primaryWorkspace,
      role: "developer",
    };
    await renderPage();

    const card = screen.getByText("Google login card tea-primary");
    expect(card.closest("section")).toHaveAttribute("id", "google-login");
    expect(
      screen
        .getByText("Two-factor card tea-primary")
        .compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      card.compareDocumentPosition(screen.getByText("Team panel")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByRole("link", { name: "Google login" })).toHaveAttribute(
      "href",
      "#google-login",
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

  // w9/m388: Render's workspace-level inbound IP rules sit in Workspace
  // Settings for every member (Admins edit, others read), after General and
  // Build Pipeline and before Team.
  it("renders Inbound IP Rules between Build Pipeline and Team, with its link", async () => {
    await renderPage();

    const card = screen.getByText(/Inbound IP rules card tea-primary/);
    expect(card.closest("section")).toHaveAttribute("id", "inbound-ip-rules");
    const pipeline = screen.getByText("Build pipeline card");
    const team = screen.getByText("Team panel");
    expect(
      pipeline.compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      card.compareDocumentPosition(team) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "Inbound IP Rules" }),
    ).toHaveAttribute("href", "#inbound-ip-rules");
  });

  it("renders Overlapping Deploy Policy after Build Pipeline, as on Render, with its link (w4/653)", async () => {
    await renderPage();

    const card = screen.getByText(/Overlapping deploy policy card tea-primary/);
    expect(card.closest("section")).toHaveAttribute(
      "id",
      "overlapping-deploy-policy",
    );
    const pipeline = screen.getByText("Build pipeline card");
    const ipRules = screen.getByText(/Inbound IP rules card tea-primary/);
    expect(
      pipeline.compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      card.compareDocumentPosition(ipRules) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "Overlapping Deploy Policy" }),
    ).toHaveAttribute("href", "#overlapping-deploy-policy");
  });

  it("opens the change-plan dialog from the Inbound IP Rules upgrade prompt", async () => {
    const user = userEvent.setup();
    await renderPage();
    expect(
      screen.queryByText("Change plan dialog open"),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Upgrade from IP rules" }),
    );

    expect(
      await screen.findByText("Change plan dialog open"),
    ).toBeInTheDocument();
  });

  it("keeps an Inbound IP Rules skeleton region in the loading branch", async () => {
    workspaceState.currentWorkspace = null;
    workspaceState.loading = true;
    const { container } = await renderPage();

    const regions = [
      ...container.querySelectorAll<HTMLElement>("[data-skeleton-region]"),
    ]
      .map((element) => element.dataset.skeletonRegion)
      .filter((name) =>
        ["general", "build-pipeline", "inbound-ip-rules", "team"].includes(
          name ?? "",
        ),
      );
    expect(regions).toEqual([
      "general",
      "build-pipeline",
      "inbound-ip-rules",
      "team",
    ]);
    expect(
      screen.getByTestId("workspace-ip-rules-skeleton"),
    ).toBeInTheDocument();
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

  // w7/m464: Render's Public / Invite-only setting, which workspace admins
  // change from the workspace's Settings page.
  it("shows Workspace privacy to an admin of an org workspace, after General, with its link", async () => {
    workspaceState.currentWorkspace = orgWorkspace;
    workspaceState.workspaces = [orgWorkspace];
    await renderPage();

    const card = screen.getByText("Workspace privacy card tea-primary");
    expect(card.closest("section")).toHaveAttribute("id", "privacy");
    expect(
      screen.getByText("Workspace details card").compareDocumentPosition(card) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getByRole("link", { name: "Workspace privacy" }),
    ).toHaveAttribute("href", "#privacy");
    expect(railSections(document.body).slice(0, 2)).toEqual([
      "general",
      "privacy",
    ]);
  });

  it.each([
    ["a developer of an org workspace", orgWorkspace, false],
    ["an admin of a standalone workspace", primaryWorkspace, true],
  ] as const)(
    "hides Workspace privacy and its link from %s",
    async (_, workspace, canManage) => {
      capabilityState.canManage = canManage;
      workspaceState.currentWorkspace = workspace;
      workspaceState.workspaces = [workspace];
      await renderPage();

      expect(
        screen.queryByText(/Workspace privacy card/),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: "Workspace privacy" }),
      ).not.toBeInTheDocument();
    },
  );

  it("previews Workspace privacy in the pending frame exactly when the page shows it", async () => {
    for (const [workspace, canManage] of [
      [orgWorkspace, true],
      [orgWorkspace, false],
      [primaryWorkspace, true],
    ] as const) {
      capabilityState.canManage = canManage;
      workspaceState.currentWorkspace = workspace;
      workspaceState.workspaces = [workspace];
      const ready = await renderPage();
      const shown = ready.container.querySelector("#privacy") !== null;
      ready.unmount();

      const pending = await renderPage(WorkspaceSettingsPageSkeleton);
      expect(frameSections(pending.container).includes("privacy")).toBe(shown);
      expect(railSections(pending.container).includes("privacy")).toBe(shown);
      expect(
        pending.container.querySelector(
          '[data-testid="workspace-privacy-skeleton"]',
        ) !== null,
      ).toBe(shown);
      pending.unmount();
    }
  });

  // w3/m305: the production side-by-side check found the pending rail
  // without Build Pipeline (the inline one also without Audit Log), and the
  // pending danger zone without the Delete card the ready page draws.
  it.each([
    [
      "a billing-manager admin with another workspace",
      "admin",
      true,
      true,
      false,
    ],
    ["a member with one workspace", "developer", false, false, false],
    ["an org admin with another workspace", "admin", true, true, true],
  ] as const)(
    "previews the ready sections, rail and danger zone for %s",
    async (_, role, billing, another, org) => {
      const workspace = { ...(org ? orgWorkspace : primaryWorkspace), role };
      workspaceState.currentWorkspace = workspace;
      workspaceState.workspaces = another
        ? [workspace, { ...workspace, id: "tea-secondary", name: "secondary" }]
        : [workspace];
      capabilityState.canManageBilling = billing;
      const ready = await renderPage();
      const readyRail = railSections(ready.container);
      for (const id of readyRail) {
        expect(ready.container.querySelector(`#${id}`)).not.toBeNull();
      }
      const readyDanger =
        ready.container.querySelector("#danger-zone")!.children.length;
      ready.unmount();

      const pending = await renderPage(WorkspaceSettingsPageSkeleton);
      expect(railSections(pending.container)).toEqual(readyRail);
      expect(frameSections(pending.container)).toEqual(readyRail);
      expect(
        pending.container.querySelector('[data-skeleton-region="danger-zone"]')!
          .children,
      ).toHaveLength(readyDanger);
    },
  );

  it("lists in the loading branch's rail exactly the sections it draws", async () => {
    workspaceState.currentWorkspace = null;
    workspaceState.workspaces = [];
    workspaceState.loading = true;
    const { container } = await renderPage();

    expect(railSections(container)).toEqual(frameSections(container));
    expect(railSections(container)).toEqual(
      expect.arrayContaining(["build-pipeline", "audit-logs"]),
    );
  });
});

/** Every region a pending frame draws, nested ones included, in order. */
function allRegions(container: HTMLElement): string[] {
  return [
    ...container.querySelectorAll<HTMLElement>("[data-skeleton-region]"),
  ].map((region) => region.dataset.skeletonRegion!);
}

// w3/073: on a direct load the frame is drawn before the workspace list
// arrives. The production check (w3/m305) found an org admin's frame without
// Workspace privacy and the Delete card, which the list would have drawn.
describe("a direct load's pending frame (w3/073)", () => {
  const other: WorkspaceView = {
    ...primaryWorkspace,
    id: "tea-secondary",
    name: "secondary",
  };

  /** The frames once the list is known: the route's and its rail. */
  async function warmFrame(workspace: WorkspaceView, list: WorkspaceView[]) {
    workspaceState.currentWorkspace = workspace;
    workspaceState.currentWorkspaceId = workspace.id;
    workspaceState.workspaces = list;
    const warm = await renderPage(WorkspaceSettingsPageSkeleton);
    const frame = {
      regions: allRegions(warm.container),
      sections: frameSections(warm.container),
      rail: railSections(warm.container),
      drawn: drawnSections(),
    };
    warm.unmount();
    return frame;
  }

  /** Everything the frames draw from, but the name they print once known. */
  function drawnSections() {
    const { result, unmount } = renderHook(() =>
      useWorkspaceSettingsSkeletonSections(),
    );
    unmount();
    return { ...result.current, workspaceName: undefined };
  }

  /** The shell before the list arrives: the selection cookie's id only. */
  function coldLoad(selectedId: string) {
    workspaceState.currentWorkspace = null;
    workspaceState.currentWorkspaceId = selectedId;
    workspaceState.workspaces = [];
    workspaceState.loading = true;
  }

  it("draws what the last visit saw for an org admin with another workspace", async () => {
    const list = [orgWorkspace, other];
    const warm = await warmFrame(orgWorkspace, list);
    expect(warm.sections).toContain("privacy");
    // What the provider stored on that visit, beside the selection.
    persistWorkspaceShape(orgWorkspace.id, workspaceShape(orgWorkspace, list));
    coldLoad(orgWorkspace.id);

    expect(drawnSections()).toEqual(warm.drawn);
    const route = await renderPage(WorkspaceSettingsPageSkeleton);
    expect(allRegions(route.container)).toEqual(warm.regions);
    expect(railSections(route.container)).toEqual(warm.rail);
    route.unmount();

    const inline = await renderPage();
    expect(frameSections(inline.container)).toEqual(warm.sections);
    expect(railSections(inline.container)).toEqual(warm.rail);
    expect(
      screen.getByRole("link", { name: "Workspace privacy" }),
    ).toHaveAttribute("href", "#privacy");
    expect(
      screen.getByTestId("workspace-privacy-skeleton"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("delete-workspace-skeleton")).toBeInTheDocument();
  });

  it("keeps the standalone default on a first visit", async () => {
    coldLoad(orgWorkspace.id);
    const { container } = await renderPage();

    expect(frameSections(container)).not.toContain("privacy");
    expect(railSections(container)).not.toContain("privacy");
    expect(
      screen.queryByTestId("delete-workspace-skeleton"),
    ).not.toBeInTheDocument();
  });

  it("ignores a shape stored for another workspace", async () => {
    persistWorkspaceShape(
      other.id,
      workspaceShape({ ...orgWorkspace, id: other.id }, [orgWorkspace, other]),
    );
    coldLoad(orgWorkspace.id);
    const { container } = await renderPage();

    expect(frameSections(container)).not.toContain("privacy");
    expect(
      screen.queryByTestId("delete-workspace-skeleton"),
    ).not.toBeInTheDocument();
  });

  it("draws a standalone workspace's frame from its stored shape too", async () => {
    const list = [primaryWorkspace, other];
    const warm = await warmFrame(primaryWorkspace, list);
    persistWorkspaceShape(
      primaryWorkspace.id,
      workspaceShape(primaryWorkspace, list),
    );
    coldLoad(primaryWorkspace.id);

    // Hobby: the 2FA and Google login cards carry their plan notice.
    expect(warm.drawn).toMatchObject({ belowPro: true, deleteWorkspace: true });
    expect(drawnSections()).toEqual(warm.drawn);
    const route = await renderPage(WorkspaceSettingsPageSkeleton);
    expect(allRegions(route.container)).toEqual(warm.regions);
    expect(railSections(route.container)).toEqual(warm.rail);
    expect(screen.getByTestId("delete-workspace-skeleton")).toBeInTheDocument();
  });
});
