import type { ReactNode } from "react";
import { render } from "@testing-library/react";
import {
  type RouteComponent,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Route } from "../workspace.settings";
import { WorkspaceSettingsPageSkeleton } from "@/common/components/route-skeletons";
import { ALLOW_ALL_PAIR_ENTRIES } from "@/common/lib/ip-allow-list";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import type { WorkspaceView } from "@/features/workspaces/types";
import type { MemberView } from "@/features/team/types";

// w7/094: a non-admin's Workspace Settings pending frame drew the Admin's
// Overlapping Deploy Policy Save, Inbound IP Rules editor, Team Invite and
// Audit Logs, and none of the Admin-only notes the ready page shows them, so
// every card below landed lower than on the ready page. Here the real cards
// render their ready state (their reads answered) beside the route's pending
// frame, for each role, and each role-dependent card must be drawn as the
// same kind with the same note.

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

// The cards whose frames do not depend on the role, or are pinned elsewhere
// (2FA and Google login: w7/089 in workspace.settings.test.tsx).
vi.mock("@/features/workspaces/components/workspace-details-card", () => ({
  WorkspaceDetailsCard: () => <div>Workspace details card</div>,
}));
vi.mock("@/features/workspaces/components/workspace-privacy-card", () => ({
  WorkspacePrivacyCard: () => <div>Workspace privacy card</div>,
}));
vi.mock("@/features/usage/components/build-pipeline-card", () => ({
  BuildPipelineCard: () => <div>Build pipeline card</div>,
}));
vi.mock(
  "@/features/workspaces/components/workspace-login-requirement-card",
  () => ({ WorkspaceLoginRequirementCard: () => <div>Login card</div> }),
);
vi.mock(
  "@/features/notifications/components/notification-overrides-card",
  () => ({ NotificationOverridesCard: () => <div>Overrides card</div> }),
);
vi.mock(
  "@/features/registry-credentials/components/registry-credentials-panel",
  () => ({ RegistryCredentialsPanel: () => <div>Registry credentials</div> }),
);
vi.mock("@/features/audit/hooks/use-audit-log", () => ({
  useAuditLog: () => ({ forbidden: false }),
}));
vi.mock("@/features/audit/components/audit-log-panel", () => ({
  AuditLogPanel: () => <div>Audit log panel</div>,
}));
vi.mock("@/features/team/components/leave-workspace-card", () => ({
  LeaveWorkspaceCard: () => <div>Leave workspace card</div>,
}));
vi.mock("@/features/workspaces/components/delete-workspace-card", () => ({
  DeleteWorkspaceCard: () => <div>Delete workspace card</div>,
}));

// The role-dependent cards render for real, on answered reads.
vi.mock(
  "@/features/workspaces/hooks/use-overlapping-deploy-policy",
  async (importOriginal) => ({
    ...(await importOriginal<object>()),
    useOverlappingDeployPolicy: () => ({
      policy: "wait",
      loading: false,
      saving: false,
      save: vi.fn(),
    }),
  }),
);
vi.mock("@/features/workspaces/hooks/use-workspace-ip-allow-list", () => ({
  useWorkspaceIPAllowList: () => ({
    entries: ALLOW_ALL_PAIR_ENTRIES,
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/workspaces/hooks/use-set-workspace-ip-allow-list", () => ({
  useSetWorkspaceIPAllowList: () => ({
    save: vi.fn(),
    saving: false,
    refusal: null,
  }),
}));

const team = { canManage: true };
const SELF: MemberView = {
  subject: "kratos-self",
  userId: "usr-self",
  email: "me@example.com",
  role: "DEVELOPER",
  createdAt: null,
  mfaEnabled: false,
  identityResolved: true,
  isSelf: true,
};
vi.mock("@/features/team/hooks/use-team", () => ({
  useTeam: () => ({
    members: [{ ...SELF, role: team.canManage ? "ADMIN" : "DEVELOPER" }],
    invites: [],
    seats: { used: 1, limit: 0 },
    loading: false,
    error: undefined,
    canManage: team.canManage,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/team/hooks/use-change-role", () => ({
  useChangeRole: () => ({ changeRole: vi.fn(), changing: null }),
}));
vi.mock("@/features/team/hooks/use-remove-member", () => ({
  useRemoveMember: () => ({
    removeMember: vi.fn(),
    revokeInvite: vi.fn(),
    removing: null,
  }),
}));
vi.mock("@/features/team/hooks/use-resend-invite", () => ({
  useResendInvite: () => ({ resendInvite: vi.fn(), resending: null }),
}));
vi.mock("@/features/team/hooks/use-invite-member", () => ({
  useInviteMember: () => ({
    invite: vi.fn(),
    busy: false,
    refusal: null,
    clearRefusal: vi.fn(),
    planLimit: null,
  }),
}));

vi.mock(
  "@/features/notifications/hooks/use-owner-notification-settings",
  () => ({
    useOwnerNotificationSettings: (ownerId: string) => ({
      settings: {
        ownerId,
        emailEnabled: true,
        notificationsToSend: "failure",
        slackEnabled: false,
        previewNotificationsEnabled: false,
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      update: vi.fn(),
      saving: false,
    }),
  }),
);

const listRead = { loading: false, error: undefined, refetch: vi.fn() };
vi.mock("@/features/agent-environment/hooks/use-agent-environment", () => ({
  useAgentEnvironment: () => ({
    ...listRead,
    environment: agentEnvironment(),
    update: vi.fn(),
    saving: false,
  }),
}));
vi.mock("@/features/agent-environment/hooks/use-agent-secrets", () => ({
  useAgentSecrets: () => ({
    ...listRead,
    secrets: [],
    set: vi.fn(),
    remove: vi.fn(),
    saving: false,
  }),
}));
vi.mock(
  "@/features/agent-environment/hooks/use-agent-repo-environments",
  () => ({
    useAgentRepoEnvironments: () => ({
      ...listRead,
      environments: [],
      set: vi.fn(),
      remove: vi.fn(),
      saving: false,
    }),
  }),
);
vi.mock("@/features/agent-environment/hooks/use-agent-skills", () => ({
  useAgentSkills: () => ({
    ...listRead,
    skills: [],
    read: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    saving: false,
  }),
}));

/** The ready strings that tell a card's Admin kind from a member's. */
const ROLE_LINES: Record<string, string[]> = {
  "overlapping-deploy-policy": [
    "Only workspace Admins can change this policy.",
  ],
  "inbound-ip-rules": [
    "Add source",
    "Only workspace Admins can change these rules.",
    "Inbound IP rules are available on Scale and Enterprise plans.",
    "Change plan",
  ],
  team: ["Invite"],
  notifications: ["Only workspace admins can change these defaults."],
  agents: [
    "Only workspace admins can change agent settings.",
    "Saving an existing name replaces its value. Sessions pick up changes from their next turn.",
    "Only workspace admins can add, change or delete agent secrets.",
    "Saving replaces the repository's whole environment. Sessions pick up changes from their next turn.",
    "Only workspace admins can add, change or delete repository environments.",
    "Members can use skills; only admins can add or change them.",
    "Only workspace admins can add or change agent skills.",
  ],
};

/** What a section draws for its role: which role lines it holds (ready
 *  copy, or the frame's reserved copy) and, for Team, whether the caller's
 *  member row has the Admin's Remove control. */
function roleShape(section: Element) {
  const name =
    section.getAttribute("id") ?? section.getAttribute("data-skeleton-region")!;
  const text = section.textContent ?? "";
  return {
    lines: ROLE_LINES[name]?.filter((line) => text.includes(line)) ?? [],
    removeControl:
      name === "team" && section.querySelector(".lucide-trash-2") !== null,
  };
}

const SECTIONS = Object.keys(ROLE_LINES);

async function renderAt(component: RouteComponent) {
  const rootRoute = createRootRoute();
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workspace/settings",
    component,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([settingsRoute]),
    history: createMemoryHistory({ initialEntries: ["/workspace/settings"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

/** The rail's section anchors, in order. */
function railSections(container: HTMLElement): string[] {
  return [...container.querySelectorAll("nav a[href^='#']")].map((link) =>
    link.getAttribute("href")!.slice(1),
  );
}

const scaleWorkspace: WorkspaceView = {
  id: "tea-pub",
  name: "pub",
  slug: "pub",
  plan: "scale",
  // The list says "admin" for every member; capabilities say who manages.
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: {
    id: "org-1",
    name: "Acme",
    primaryWorkspaceId: "tea-hq",
    requireTwoFactor: false,
    requireGoogleLogin: false,
  },
  primary: false,
  privacy: "public",
};
const hobbyWorkspace: WorkspaceView = {
  ...scaleWorkspace,
  id: "tea-hobby",
  plan: "hobby",
  organization: null,
  privacy: null,
};

beforeEach(() => {
  workspaceState.loading = false;
});

describe("Workspace Settings pending frame by role (w7/094)", () => {
  it.each([
    ["a Developer of a Scale org workspace", scaleWorkspace, false],
    ["an Admin of a Scale org workspace", scaleWorkspace, true],
    ["a Developer of a Hobby workspace", hobbyWorkspace, false],
    ["an Admin of a Hobby workspace", hobbyWorkspace, true],
  ] as const)(
    "draws each role-dependent card as %s will see it",
    async (_who, workspace, canManage) => {
      workspaceState.currentWorkspace = workspace;
      workspaceState.currentWorkspaceId = workspace.id;
      workspaceState.workspaces = [workspace];
      team.canManage = canManage;
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({
          role: canManage ? "ADMIN" : "DEVELOPER",
          canManage,
          canManageBilling: canManage,
        }),
      );

      const ready = await renderAt(Route.options.component!);
      const readyRail = railSections(ready.container);
      const readyShapes = SECTIONS.map((id) =>
        roleShape(ready.container.querySelector(`section#${id}`)!),
      );
      ready.unmount();

      const pending = await renderAt(WorkspaceSettingsPageSkeleton);
      expect(railSections(pending.container)).toEqual(readyRail);
      expect(readyRail.includes("audit-logs")).toBe(canManage);
      const pendingShapes = SECTIONS.map((id) =>
        roleShape(
          pending.container.querySelector(`[data-skeleton-region="${id}"]`)!,
        ),
      );
      expect(pendingShapes).toEqual(readyShapes);
      // Every card above was told which kind it is: a member reads every
      // Admin-only note, an Admin none.
      const notes = readyShapes
        .flatMap((shape) => shape.lines)
        .filter((line) => line.startsWith("Only workspace"));
      expect(notes.length > 0).toBe(!canManage);
      pending.unmount();
    },
  );

  // An unknown role (capabilities still checking) keeps the Admin's frame,
  // with both help lines held where the Admin's and a member's differ.
  it("keeps the Admin's frame while capabilities are checking", async () => {
    workspaceState.currentWorkspace = scaleWorkspace;
    workspaceState.currentWorkspaceId = scaleWorkspace.id;
    workspaceState.workspaces = [scaleWorkspace];
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        loading: true,
        loaded: false,
        canManage: false,
        canManageBilling: false,
      }),
    );
    const pending = await renderAt(WorkspaceSettingsPageSkeleton);
    const shapes = Object.fromEntries(
      SECTIONS.map((id) => [
        id,
        roleShape(
          pending.container.querySelector(`[data-skeleton-region="${id}"]`)!,
        ),
      ]),
    );
    expect(shapes["overlapping-deploy-policy"].lines).toEqual([]);
    expect(shapes["inbound-ip-rules"].lines).toEqual(["Add source"]);
    expect(shapes.team).toEqual({ lines: ["Invite"], removeControl: true });
    expect(shapes.notifications.lines).toEqual([]);
    expect(shapes.agents.lines).toEqual(ROLE_LINES.agents.slice(1));
    expect(railSections(pending.container)).toContain("audit-logs");
  });
});
