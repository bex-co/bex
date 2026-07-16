import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuditLogPanel } from "@/features/audit/components/audit-log-panel";
import type { UseAuditLogResult } from "@/features/audit/hooks/use-audit-log";

// The panel is presentational (w4/m15): its owner (SecurityComplianceSection)
// runs the query and passes the result down, so tests inject `state` directly.
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

function renderPanel(overrides: Partial<UseAuditLogResult> = {}) {
  return render(<AuditLogPanel state={{ ...auditState, ...overrides }} />);
}

beforeEach(() => {
  auditState.events = [];
  auditState.loading = false;
  auditState.loadingMore = false;
  auditState.error = undefined;
  auditState.forbidden = false;
  auditState.unavailable = false;
  auditState.hasMore = false;
  loadMore.mockReset();
});

describe("AuditLogPanel", () => {
  it("renders nothing for a non-admin (forbidden) caller", () => {
    auditState.forbidden = true;
    const { container } = renderPanel();
    expect(container).toBeEmptyDOMElement();
  });

  it("shows a loading skeleton on the initial fetch", () => {
    auditState.loading = true;
    const { container } = renderPanel();
    expect(screen.getByText("Audit Log")).toBeInTheDocument();
    expect(
      container.querySelectorAll("[data-slot='skeleton']").length,
    ).toBeGreaterThan(0);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an explanatory state when the audit store isn't configured (503)", () => {
    auditState.unavailable = true;
    renderPanel();
    expect(screen.getByText("Audit log not configured")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows a generic error state for any other failure", () => {
    auditState.error = new Error("boom");
    renderPanel();
    expect(screen.getByText("Couldn't load the audit log")).toBeInTheDocument();
  });

  it("shows an empty state with no events", () => {
    renderPanel();
    expect(screen.getByText("No audit events yet")).toBeInTheDocument();
  });

  it("renders events newest-first with allowed/denied badges, and 'Load more' pages further", () => {
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "user:alice",
        actorMethod: "session",
        action: "update",
        status: "success",
        resource: "workspace:tea-1",
        targetName: "my-api",
      },
      {
        id: "ev-2",
        timestamp: "2026-07-10T00:00:00Z",
        actor: "",
        actorMethod: "",
        action: "delete",
        status: "denied",
        resource: "service:srv-1",
        targetName: "",
      },
    ];
    auditState.hasMore = true;
    renderPanel();

    const rows = screen.getAllByRole("row").slice(1); // drop the header row
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Allowed");
    expect(rows[1]).toHaveTextContent("Denied");
    expect(rows[1]).toHaveTextContent("Unknown"); // empty actor placeholder
    // Friendly target name (w10/m5): shown alongside the raw id when the row
    // carries one; a pre-0038 row (empty targetName) keeps the id-only cell.
    expect(rows[0]).toHaveTextContent("my-api");
    expect(rows[0]).toHaveTextContent("workspace:tea-1");
    expect(rows[1]).not.toHaveTextContent("my-api");
    expect(rows[1]).toHaveTextContent("service:srv-1");

    const loadMoreButton = screen.getByRole("button", { name: "Load more" });
    expect(loadMoreButton).toBeInTheDocument();
    loadMoreButton.click();
    expect(loadMore).toHaveBeenCalled();
  });

  it("hides 'Load more' once the last page is exhausted", () => {
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "user:alice",
        actorMethod: "session",
        action: "update",
        status: "success",
        resource: "workspace:tea-1",
        targetName: "",
      },
    ];
    auditState.hasMore = false;
    renderPanel();
    expect(
      screen.queryByRole("button", { name: "Load more" }),
    ).not.toBeInTheDocument();
  });
});
