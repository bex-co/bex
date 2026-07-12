import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SecurityComplianceSection } from "@/features/auth/pages/settings-page/security-compliance-section";
import type { UseAuditLogResult } from "@/features/audit/hooks/use-audit-log";

vi.mock("@/features/team/hooks/use-current-workspace", () => ({
  useCurrentWorkspace: () => ({
    workspace: { id: "tea-1", name: "acme", plan: "pro", role: "ADMIN" },
    loading: false,
    error: undefined,
  }),
}));

const loadMore = vi.fn();
const auditState: UseAuditLogResult = {
  events: [],
  loading: false,
  loadingMore: false,
  error: undefined,
  forbidden: false,
  unavailable: false,
  hasMore: false,
  loadMore,
};
vi.mock("@/features/audit/hooks/use-audit-log", () => ({
  useAuditLog: () => auditState,
}));

beforeEach(() => {
  auditState.events = [];
  auditState.loading = false;
  auditState.error = undefined;
  auditState.forbidden = false;
  auditState.unavailable = false;
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

  it("hides the whole section (heading included) for a forbidden non-admin", () => {
    auditState.forbidden = true;
    const { container } = render(<SecurityComplianceSection />);

    expect(
      screen.queryByRole("region", { name: /security & compliance/i }),
    ).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
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
});
