import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
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
});
