import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LeaveWorkspaceCard } from "@/features/team/components/leave-workspace-card";
import type { MemberView } from "@/features/team/types";
import type { WorkspaceView } from "@/features/workspaces/types";

const members: { value: MemberView[] } = { value: [] };
vi.mock("@/features/team/hooks/use-team", () => ({
  useTeam: () => ({ members: members.value, refetch: vi.fn() }),
}));

const leave = vi.fn();
const leaveState: { busy: boolean; error: string | null } = {
  busy: false,
  error: null,
};
vi.mock("@/features/team/hooks/use-leave-workspace", () => ({
  useLeaveWorkspace: () => ({
    leave,
    busy: leaveState.busy,
    error: leaveState.error,
  }),
}));

const setCurrentWorkspaceId = vi.fn();
const refetchWorkspaces = vi.fn();
const workspaceState: { workspaces: WorkspaceView[] } = { workspaces: [] };
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: workspaceState.workspaces,
    setCurrentWorkspaceId,
    refetch: refetchWorkspaces,
  }),
}));

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

function member(overrides: Partial<MemberView> = {}): MemberView {
  return {
    subject: "subj",
    userId: "own-1",
    email: "a@example.com",
    role: "DEVELOPER",
    createdAt: null,
    mfaEnabled: false,
    identityResolved: true,
    isOwner: false,
    isSelf: false,
    ...overrides,
  };
}

const workspace = {
  id: "tea-1",
  name: "acme",
} as unknown as WorkspaceView;

function renderCard() {
  return render(<LeaveWorkspaceCard workspace={workspace} />);
}

describe("LeaveWorkspaceCard (w5/m102)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    leaveState.busy = false;
    leaveState.error = null;
    workspaceState.workspaces = [
      workspace,
      { id: "tea-2", name: "other" } as unknown as WorkspaceView,
    ];
    members.value = [
      member({ subject: "me", isSelf: true }),
      member({ subject: "admin", role: "ADMIN" }),
    ];
  });

  it("leaves after confirmation and lands the caller in a workspace they kept", async () => {
    leave.mockResolvedValue(true);
    renderCard();

    await userEvent.click(screen.getByRole("button", { name: /Leave/ }));
    // The confirmation names the workspace — leaving the wrong one is exactly
    // the mistake a bare button invites. (The card's own description names it
    // too, so scope the assertion to the dialog.)
    const dialog = screen.getByRole("alertdialog");
    expect(within(dialog).getByText(/acme/)).toBeInTheDocument();
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Leave workspace" }),
    );

    expect(leave).toHaveBeenCalledTimes(1);
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
    expect(navigate).toHaveBeenCalledWith({ to: "/", replace: true });
  });

  it("does not switch workspaces when the server refuses", async () => {
    leave.mockResolvedValue(false);
    renderCard();

    await userEvent.click(screen.getByRole("button", { name: /Leave/ }));
    await userEvent.click(
      screen.getByRole("button", { name: "Leave workspace" }),
    );

    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("disables the action for the workspace owner and says why", () => {
    members.value = [
      member({ subject: "me", isSelf: true, isOwner: true, role: "ADMIN" }),
      member({ subject: "admin", role: "ADMIN" }),
    ];
    renderCard();

    expect(screen.getByRole("button", { name: /Leave/ })).toBeDisabled();
    expect(
      document.querySelector('[data-slot="tooltip-trigger"]'),
    ).not.toBeNull();
  });

  it("disables the action for the last admin", () => {
    members.value = [
      member({ subject: "me", isSelf: true, role: "ADMIN" }),
      member({ subject: "dev", role: "DEVELOPER" }),
    ];
    renderCard();

    expect(screen.getByRole("button", { name: /Leave/ })).toBeDisabled();
  });

  it("renders nothing when the caller is not a member", () => {
    members.value = [member({ subject: "someone-else" })];
    const { container } = renderCard();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the server's refusal inline", () => {
    leaveState.error = "you own this workspace and cannot leave it";
    renderCard();
    expect(
      screen.getByText(/you own this workspace and cannot leave it/),
    ).toBeInTheDocument();
  });
});
