import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import { SecurityComplianceSection } from "@/features/auth/pages/settings-page/security-compliance-section";

vi.mock("@/features/connected-agents/hooks/use-connected-agents", () => ({
  useConnectedAgents: () => ({
    agents: [],
    loading: false,
    error: false,
    revoke: vi.fn(),
    revoking: null,
    refetch: vi.fn(),
  }),
}));

vi.mock("@/features/sessions/hooks/use-active-sessions", () => ({
  useActiveSessions: () => ({
    sessions: [],
    loading: false,
    error: false,
    revoke: vi.fn(),
    revoking: null,
    signOutOthers: vi.fn(),
    signingOutOthers: false,
    refetch: vi.fn(),
  }),
}));

// The workspace audit log moved to Workspace Settings → Audit Logs (w3/m305);
// this section keeps the account's own security controls.
describe("SecurityComplianceSection", () => {
  it("renders Connected Agents and Active Sessions inside the section, and no audit log", () => {
    render(<SecurityComplianceSection />);

    const region = screen.getByRole("region", { name: "Security" });
    expect(within(region).getByText("Connected agents")).toBeInTheDocument();
    expect(within(region).getByText("Active sessions")).toBeInTheDocument();
    expect(within(region).queryByText("Audit Log")).not.toBeInTheDocument();
  });

  // The CLI's logout warning sends a token it could not revoke to
  // `<dashboard>/settings#cli-tokens` (w32/054): that fragment must land on
  // the card listing the bex CLI's grant, clear of the sticky section bar.
  it("anchors settings#cli-tokens on the Connected agents card", () => {
    render(<SecurityComplianceSection />);

    const target = document.getElementById("cli-tokens");
    expect(target).not.toBeNull();
    expect(within(target!).getByText("Connected agents")).toBeInTheDocument();
    expect(
      within(target!).queryByText("Active sessions"),
    ).not.toBeInTheDocument();
    expect(target!.closest("section")).toHaveAttribute("id", "security");
    expect(target).toHaveClass(...SECTION_NAVIGATION_TARGET_CLASS.split(" "));
  });
});
