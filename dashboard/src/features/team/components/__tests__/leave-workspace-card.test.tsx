import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LeaveWorkspaceCard } from "@/features/team/components/leave-workspace-card";
import { codedGraphQLError } from "@/test/mocks/apollo";
import type { MemberView } from "@/features/team/types";
import type { WorkspaceView } from "@/features/workspaces/types";

const members: { value: MemberView[]; loading?: boolean } = { value: [] };
vi.mock("@/features/team/hooks/use-team", () => ({
  useTeam: () => ({
    members: members.value,
    loading: members.loading ?? false,
    refetch: vi.fn(),
  }),
}));

// The real useLeaveWorkspace runs (it owns the post-leave switch and
// navigation); only the transport underneath it is faked.
const leave = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [leave],
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
    leave.mockResolvedValue({ data: { leaveWorkspace: true } });
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
    leave.mockRejectedValue(new Error("forbidden"));
    renderCard();

    await userEvent.click(screen.getByRole("button", { name: /Leave/ }));
    await userEvent.click(
      screen.getByRole("button", { name: "Leave workspace" }),
    );

    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("offers the action to an admin while another admin remains", () => {
    members.value = [
      member({ subject: "me", isSelf: true, role: "ADMIN" }),
      member({ subject: "admin", role: "ADMIN" }),
    ];
    renderCard();

    expect(screen.getByRole("button", { name: /Leave/ })).toBeEnabled();
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

  // w9/082: while the members load the card holds its place (same header,
  // disabled button) instead of rendering nothing and popping in.
  it("holds the card's place while the members load", () => {
    members.value = [];
    members.loading = true;
    renderCard();
    const card = screen.getByTestId("leave-workspace-skeleton");
    expect(card).toHaveTextContent("Leave workspace");
    expect(within(card).getByRole("button", { hidden: true })).toBeDisabled();
    members.loading = false;
  });

  it("renders the server's refusal inline", async () => {
    leave.mockRejectedValue(
      codedGraphQLError(
        "CANNOT_LEAVE_OWNED_WORKSPACE",
        {},
        "you own this workspace and cannot leave it",
      ),
    );
    renderCard();

    await userEvent.click(screen.getByRole("button", { name: /Leave/ }));
    await userEvent.click(
      screen.getByRole("button", { name: "Leave workspace" }),
    );

    // The server's own reason is relayed (first letter capitalized), under the
    // alert's title.
    expect(
      await screen.findByText("You own this workspace and cannot leave it"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Could not leave the workspace"),
    ).toBeInTheDocument();
  });
});
