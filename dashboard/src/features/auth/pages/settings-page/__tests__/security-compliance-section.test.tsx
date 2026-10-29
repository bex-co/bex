import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SecurityComplianceSection } from "@/features/auth/pages/settings-page/security-compliance-section";
import type { UseAuditLogResult } from "@/features/audit/hooks/use-audit-log";
import type { ReactNode } from "react";

const loadMore = vi.fn();
const auditState: UseAuditLogResult = {
  events: [],
  loading: false,
  loadingMore: false,
  error: undefined,
  forbidden: false,
  unavailable: false,
  planRequired: false,
  hasMore: false,
  loadMore,
};
vi.mock("@/features/audit/hooks/use-audit-log", () => ({
  useAuditLog: () => auditState,
}));

// The plan notice links to Billing; render the router Link as its anchor.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({ to, children }: { to: string; children: ReactNode }) => (
    <a href={to}>{children}</a>
  ),
}));

vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));

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

beforeEach(() => {
  auditState.events = [];
  auditState.loading = false;
  auditState.error = undefined;
  auditState.forbidden = false;
  auditState.unavailable = false;
  auditState.planRequired = false;
  auditState.hasMore = false;
  loadMore.mockReset();
});

describe("SecurityComplianceSection", () => {
  it("renders the Audit Log card *inside* the Security & Compliance section", () => {
    render(<SecurityComplianceSection />);

    // The section is an accessible region named by its heading; the Audit Log
    // card must live within it, not as a bare sibling elsewhere on the page.
    const region = screen.getByRole("region", {
      name: /security & compliance/i,
    });
    expect(within(region).getByText("Audit Log")).toBeInTheDocument();
  });

  it("also renders Connected Agents and Active Sessions inside the section", () => {
    render(<SecurityComplianceSection />);

    const region = screen.getByRole("region", {
      name: /security & compliance/i,
    });
    expect(within(region).getByText("Connected agents")).toBeInTheDocument();
    expect(within(region).getByText("Active sessions")).toBeInTheDocument();
  });

  it("keeps rendering the section (and the member-visible cards) for a forbidden non-admin, hiding only Audit Log", () => {
    auditState.forbidden = true;
    render(<SecurityComplianceSection />);

    const region = screen.getByRole("region", {
      name: /security & compliance/i,
    });
    expect(within(region).queryByText("Audit Log")).not.toBeInTheDocument();
    expect(within(region).getByText("Connected agents")).toBeInTheDocument();
    expect(within(region).getByText("Active sessions")).toBeInTheDocument();
  });

  it("keeps the store-less (503) state under the section heading", () => {
    auditState.unavailable = true;
    render(<SecurityComplianceSection />);

    const region = screen.getByRole("region", {
      name: /security & compliance/i,
    });
    expect(
      within(region).getByText("Audit log not configured"),
    ).toBeInTheDocument();
  });

  it("shows the Pro-plan notice with a Billing link on a Hobby workspace (w8/m138)", () => {
    auditState.planRequired = true;
    render(<SecurityComplianceSection />);

    const region = screen.getByRole("region", {
      name: /security & compliance/i,
    });
    expect(
      within(region).getByText("Audit logs require the Pro plan"),
    ).toBeInTheDocument();
    expect(
      within(region).getByRole("link", { name: "Open Billing" }),
    ).toHaveAttribute("href", "/billing");
    expect(
      within(region).queryByText("Couldn't load the audit log"),
    ).not.toBeInTheDocument();
  });
});
