import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamPanel } from "@/features/team/components/team-panel";
import type { MemberView, InviteView } from "@/features/team/types";

const teamState: {
  members: MemberView[];
  invites: InviteView[];
  loading: boolean;
  error: Error | undefined;
  canManage: boolean;
} = {
  members: [],
  invites: [],
  loading: false,
  error: undefined,
  canManage: true,
};
const refetch = vi.fn();

// The panel manages the *switcher's* workspace (WorkspaceProvider), not a
// workspace it resolves itself — see the workspace-scoping cases below.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId, loading: false }),
}));

const useTeamSpy = vi.fn();
vi.mock("@/features/team/hooks/use-team", () => ({
  useTeam: (workspaceId: string | null) => {
    useTeamSpy(workspaceId);
    return { ...teamState, refetch };
  },
}));

const changeRole = vi.fn();
const useChangeRoleSpy = vi.fn();
vi.mock("@/features/team/hooks/use-change-role", () => ({
  useChangeRole: (workspaceId: string) => {
    useChangeRoleSpy(workspaceId);
    return { changeRole, changing: null };
  },
}));

const removeMember = vi.fn();
const revokeInvite = vi.fn();
const useRemoveMemberSpy = vi.fn();
vi.mock("@/features/team/hooks/use-remove-member", () => ({
  useRemoveMember: (workspaceId: string) => {
    useRemoveMemberSpy(workspaceId);
    return { removeMember, revokeInvite, removing: null };
  },
}));

vi.mock("@/features/team/components/invite-member-dialog", () => ({
  InviteMemberDialog: () => <div data-testid="invite-dialog" />,
}));

beforeEach(() => {
  teamState.members = [];
  teamState.invites = [];
  teamState.loading = false;
  teamState.error = undefined;
  teamState.canManage = true;
  currentWorkspaceId = "tea-1";
  refetch.mockReset();
  changeRole.mockReset();
  removeMember.mockReset();
  revokeInvite.mockReset();
  useTeamSpy.mockReset();
  useChangeRoleSpy.mockReset();
  useRemoveMemberSpy.mockReset();
});

/** The workspace id the panel most recently drove its team hooks with. */
function lastWorkspaceId(spy: typeof useTeamSpy): unknown {
  const calls = spy.mock.calls;
  return calls[calls.length - 1]?.[0];
}

describe("TeamPanel", () => {
  it("lists members with their roles (w4/m12/t004)", () => {
    teamState.members = [
      {
        subject: "id-admin",
        userId: "own-1",
        email: "admin@example.com",
        role: "ADMIN",
        createdAt: null,
      },
      {
        subject: "id-bob",
        userId: "own-2",
        email: "",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.getByText("admin@example.com")).toBeInTheDocument();
    expect(screen.getByText("own-2")).toBeInTheDocument();
  });

  it("a member with a resolvable email shows the email, not the subject (w6/m10)", () => {
    teamState.members = [
      {
        subject: "id-admin",
        userId: "own-1",
        email: "admin@example.com",
        role: "ADMIN",
        createdAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.getByText("admin@example.com")).toBeInTheDocument();
    // The raw subject is demoted to a secondary line, not the primary cell.
    expect(screen.getByText("id-admin")).toBeInTheDocument();
  });

  it("a member without email falls back to the own- userId, never blank (w6/m10)", () => {
    teamState.members = [
      {
        subject: "id-bob",
        userId: "own-2",
        email: "",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.getByText("own-2")).toBeInTheDocument();
    expect(screen.queryByText("admin@example.com")).not.toBeInTheDocument();
  });

  it("a member with neither email nor userId still shows the subject (fully degraded)", () => {
    teamState.members = [
      {
        subject: "id-carol",
        userId: "",
        email: "",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.getByText("id-carol")).toBeInTheDocument();
  });

  it("an admin sees the invite dialog and per-member controls", () => {
    teamState.members = [
      {
        subject: "id-bob",
        userId: "own-2",
        email: "",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.getByTestId("invite-dialog")).toBeInTheDocument();
    // The role dropdown (a combobox) and a remove button are present.
    expect(screen.getByRole("combobox")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove" })).toBeInTheDocument();
  });

  it("a read-only (non-admin) caller sees no invite/role/remove controls", () => {
    teamState.canManage = false;
    teamState.members = [
      {
        subject: "id-bob",
        userId: "own-2",
        email: "",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.queryByTestId("invite-dialog")).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Remove" }),
    ).not.toBeInTheDocument();
    // ...but the role is still shown as text.
    expect(screen.getByText("Viewer")).toBeInTheDocument();
  });

  it("shows pending invites to an admin", () => {
    teamState.invites = [
      {
        id: "inv-1",
        email: "carol@example.com",
        role: "DEVELOPER",
        expiresAt: null,
      },
    ];
    render(<TeamPanel />);
    expect(screen.getByText("Pending invites")).toBeInTheDocument();
    expect(screen.getByText("carol@example.com")).toBeInTheDocument();
  });

  it("filters accepted members by email or fallback identity without hiding invites", async () => {
    teamState.members = [
      {
        subject: "id-alice",
        userId: "own-alice",
        email: "alice@example.com",
        role: "ADMIN",
        createdAt: null,
      },
      {
        subject: "id-bob",
        userId: "own-bob",
        email: "bob@example.com",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    teamState.invites = [
      {
        id: "inv-1",
        email: "pending@example.com",
        role: "DEVELOPER",
        expiresAt: null,
      },
    ];
    const user = userEvent.setup();
    render(<TeamPanel />);

    await user.type(
      screen.getByRole("searchbox", { name: "Search members" }),
      "OWN-BOB",
    );

    expect(screen.queryByText("alice@example.com")).not.toBeInTheDocument();
    expect(screen.getByText("bob@example.com")).toBeInTheDocument();
    expect(screen.getByText("pending@example.com")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toBeInTheDocument();
  });

  it("distinguishes an empty team from a search with no matches", async () => {
    const { rerender } = render(<TeamPanel />);
    expect(screen.getByText("No members yet")).toBeInTheDocument();

    teamState.members = [
      {
        subject: "id-alice",
        userId: "own-alice",
        email: "alice@example.com",
        role: "ADMIN",
        createdAt: null,
      },
    ];
    rerender(<TeamPanel />);
    const user = userEvent.setup();
    await user.type(
      screen.getByRole("searchbox", { name: "Search members" }),
      "nobody",
    );

    expect(screen.getByRole("status")).toHaveTextContent("No matching members");
    expect(screen.queryByText("No members yet")).not.toBeInTheDocument();
  });

  it("a confirmed remove calls removeMember (keyed by subject) and refetches on success", async () => {
    teamState.members = [
      {
        subject: "id-bob",
        userId: "own-2",
        email: "bob@example.com",
        role: "VIEWER",
        createdAt: null,
      },
    ];
    removeMember.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<TeamPanel />);

    await user.click(screen.getByRole("button", { name: "Remove" }));
    const dialog = await screen.findByRole("alertdialog");
    // The confirm dialog interpolates the display identity (email), not the raw subject.
    expect(within(dialog).getByText(/bob@example.com/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Remove" }));

    // Mutation keying is unchanged by the enrichment (t001's verdict): still subject.
    expect(removeMember).toHaveBeenCalledWith("id-bob");
    expect(refetch).toHaveBeenCalled();
  });

  it("revoking a pending invite calls revokeInvite", async () => {
    teamState.invites = [
      {
        id: "inv-1",
        email: "carol@example.com",
        role: "DEVELOPER",
        expiresAt: null,
      },
    ];
    revokeInvite.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<TeamPanel />);

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    expect(revokeInvite).toHaveBeenCalledWith("inv-1");
  });

  it("surfaces a load error", () => {
    teamState.error = new Error("boom");
    render(<TeamPanel />);
    expect(screen.getByText("Couldn't load the team")).toBeInTheDocument();
  });

  // w6/m14 — the bug: the panel read `useCurrentWorkspace()` (its own
  // `workspaces` query, always `workspaces[0]` — the account's original
  // auto-provisioned workspace), so after switching it kept managing the wrong
  // workspace's members. The owner must be the switcher's selection, the same
  // fix the audit log got (use-audit-log.ts).
  describe("workspace scoping (w6/m14 regression)", () => {
    it("manages the switcher's selected workspace, not the account's first", () => {
      currentWorkspaceId = "tea-2";
      render(<TeamPanel />);

      expect(lastWorkspaceId(useTeamSpy)).toBe("tea-2");
      expect(lastWorkspaceId(useChangeRoleSpy)).toBe("tea-2");
      expect(lastWorkspaceId(useRemoveMemberSpy)).toBe("tea-2");
    });

    it("re-reads the members of the new workspace after a switch", () => {
      const { rerender } = render(<TeamPanel />);
      expect(lastWorkspaceId(useTeamSpy)).toBe("tea-1");

      // The switcher moves to the other workspace.
      currentWorkspaceId = "tea-2";
      rerender(<TeamPanel />);

      expect(lastWorkspaceId(useTeamSpy)).toBe("tea-2");
      expect(lastWorkspaceId(useRemoveMemberSpy)).toBe("tea-2");
    });

    it("passes no workspace (and hides the invite dialog) until the selection resolves", () => {
      currentWorkspaceId = null;
      render(<TeamPanel />);

      expect(lastWorkspaceId(useTeamSpy)).toBeNull();
      expect(screen.queryByTestId("invite-dialog")).not.toBeInTheDocument();
    });
  });
});
