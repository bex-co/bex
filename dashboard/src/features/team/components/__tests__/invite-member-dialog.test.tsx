import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InviteMemberDialog } from "@/features/team/components/invite-member-dialog";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const invite = vi.fn();
const inviteState: { busy: boolean; planLimit: string | null } = {
  busy: false,
  planLimit: null,
};
vi.mock("@/features/team/hooks/use-invite-member", () => ({
  useInviteMember: () => ({ invite, ...inviteState }),
}));

const onInvited = vi.fn();

beforeEach(() => {
  mockNavigate.mockReset();
  invite.mockReset();
  onInvited.mockReset();
  inviteState.busy = false;
  inviteState.planLimit = null;
});

/** Opens the dialog (its trigger button lives in the team panel's header). */
async function openDialog(user: ReturnType<typeof userEvent.setup>) {
  render(<InviteMemberDialog workspaceId="tea-1" onInvited={onInvited} />);
  await user.click(screen.getByRole("button", { name: "Invite" }));
}

describe("InviteMemberDialog", () => {
  it("sends the invite and closes on success", async () => {
    invite.mockResolvedValue(true);
    const user = userEvent.setup();
    await openDialog(user);

    await user.type(screen.getByLabelText("Email address"), "carol@example.com");
    await user.click(screen.getByRole("button", { name: "Send invite" }));

    expect(invite).toHaveBeenCalledWith("carol@example.com", "DEVELOPER");
    expect(onInvited).toHaveBeenCalled();
  });

  // w6/m15/t001 — the gap: a plan-capped invite used to dead-end in a toast that
  // named the upgrade without offering one.
  describe("the plan-limit refusal (w6/m15/t001)", () => {
    it("shows the backend's reason inline with a change-plan CTA", async () => {
      inviteState.planLimit =
        "the hobby plan is limited to 1 workspace member(s); upgrade to invite more";
      const user = userEvent.setup();
      await openDialog(user);

      // The backend's exact copy, not a client-side paraphrase of the rule.
      expect(
        screen.getByText(/the hobby plan is limited to 1 workspace member/),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Change plan/ }),
      ).toBeInTheDocument();
    });

    it("the CTA routes to the plan section and opens the change-plan dialog", async () => {
      inviteState.planLimit = "the hobby plan is limited to 1 workspace member(s)";
      const user = userEvent.setup();
      await openDialog(user);

      await user.click(screen.getByRole("button", { name: /Change plan/ }));

      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/workspace/settings",
        search: { plan: "change" },
      });
    });

    it("shows no CTA while the invite has not been refused", async () => {
      const user = userEvent.setup();
      await openDialog(user);

      expect(
        screen.queryByRole("button", { name: /Change plan/ }),
      ).not.toBeInTheDocument();
    });
  });
});
