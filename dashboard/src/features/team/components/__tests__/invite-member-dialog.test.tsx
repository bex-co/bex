import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InviteMemberDialog } from "@/features/team/components/invite-member-dialog";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const invite = vi.fn();
const clearRefusal = vi.fn();
const inviteState: {
  busy: boolean;
  planLimit: string | null;
  refusal: string | null;
} = {
  busy: false,
  planLimit: null,
  refusal: null,
};
vi.mock("@/features/team/hooks/use-invite-member", () => ({
  useInviteMember: () => ({ invite, clearRefusal, ...inviteState }),
}));

const onInvited = vi.fn();

beforeEach(() => {
  mockNavigate.mockReset();
  invite.mockReset();
  onInvited.mockReset();
  clearRefusal.mockReset();
  inviteState.busy = false;
  inviteState.planLimit = null;
  inviteState.refusal = null;
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

    await user.type(
      screen.getByLabelText("Email address"),
      "carol@example.com",
    );
    await user.click(screen.getByRole("button", { name: "Send invite" }));

    expect(invite).toHaveBeenCalledWith("carol@example.com", "DEVELOPER");
    expect(onInvited).toHaveBeenCalled();
  });

  // w2/m28 — the CTA renders off the PLAN_LIMIT error code, not a substring of
  // the English message. The hook (use-invite-member.ts) builds a localized
  // string from params; the dialog renders whatever planLimit string it receives.
  describe("the plan-limit refusal", () => {
    it("shows the plan-limit message inline with a change-plan CTA", async () => {
      // Localized copy from the hook — deliberately does NOT contain "plan" to
      // prove the dialog does no string-matching of its own.
      inviteState.planLimit = "Upgrade to invite more workspace members.";
      const user = userEvent.setup();
      await openDialog(user);

      expect(
        screen.getByText("Upgrade to invite more workspace members."),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Change plan/ }),
      ).toBeInTheDocument();
    });

    it("the CTA routes to the plan section and opens the change-plan dialog", async () => {
      inviteState.planLimit =
        "The hobby plan is limited to 1 workspace member(s). Upgrade to invite more.";
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

// w1/m82 — the dialog used to enable Send for any non-empty string, so an
// obviously-malformed address only failed after a round trip, and the refusal
// came back as a generic "Couldn't invite {email}" toast that never said why.
describe("InviteMemberDialog validation and refusals", () => {
  it("keeps Send disabled and explains a malformed email without calling the mutation", async () => {
    const user = userEvent.setup();
    await openDialog(user);

    await user.type(screen.getByLabelText("Email address"), "not-an-email");
    expect(screen.getByText(/Enter a valid email address/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send invite" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Send invite" }));
    expect(invite).not.toHaveBeenCalled();

    // Completing the address clears the hint and arms Send.
    await user.clear(screen.getByLabelText("Email address"));
    await user.type(screen.getByLabelText("Email address"), "ok@example.com");
    expect(
      screen.queryByText(/Enter a valid email address/),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send invite" })).toBeEnabled();
  });

  it("shows the server's own refusal inline instead of a generic message", async () => {
    // e.g. inviting someone who already belongs to the workspace (w1/m82).
    inviteState.refusal =
      "Teammate@example.com is already a member of this workspace; change their role instead of inviting them again";
    const user = userEvent.setup();
    await openDialog(user);

    expect(screen.getByRole("alert")).toHaveTextContent(
      /already a member of this workspace/,
    );
  });

  it("clears a standing refusal when the address is edited", async () => {
    inviteState.refusal = "Already a member";
    const user = userEvent.setup();
    await openDialog(user);

    await user.type(screen.getByLabelText("Email address"), "a");
    expect(clearRefusal).toHaveBeenCalled();
  });
});
