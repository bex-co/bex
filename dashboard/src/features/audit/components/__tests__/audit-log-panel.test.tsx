import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuditLogPanel } from "@/features/audit/components/audit-log-panel";
import type { AuditEvent } from "@/features/audit/types";

vi.mock("@/features/team/hooks/use-current-workspace", () => ({
  useCurrentWorkspace: () => ({
    workspace: { id: "tea-1", name: "acme", plan: "pro", role: "ADMIN" },
    loading: false,
    error: undefined,
  }),
}));

const auditState: {
  events: AuditEvent[];
  loading: boolean;
  loadingMore: boolean;
  error: Error | undefined;
  forbidden: boolean;
  unavailable: boolean;
  hasMore: boolean;
} = {
  events: [],
  loading: false,
  loadingMore: false,
  error: undefined,
  forbidden: false,
  unavailable: false,
  hasMore: false,
};
const loadMore = vi.fn();
vi.mock("@/features/audit/hooks/use-audit-log", () => ({
  useAuditLog: () => ({ ...auditState, loadMore }),
}));

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
    const { container } = render(<AuditLogPanel />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows a loading skeleton on the initial fetch", () => {
    auditState.loading = true;
    const { container } = render(<AuditLogPanel />);
    expect(screen.getByText("Audit Log")).toBeInTheDocument();
    expect(
      container.querySelectorAll("[data-slot='skeleton']").length,
    ).toBeGreaterThan(0);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an explanatory state when the audit store isn't configured (503)", () => {
    auditState.unavailable = true;
    render(<AuditLogPanel />);
    expect(screen.getByText("Audit log not configured")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows a generic error state for any other failure", () => {
    auditState.error = new Error("boom");
    render(<AuditLogPanel />);
    expect(screen.getByText("Couldn't load the audit log")).toBeInTheDocument();
  });

  it("shows an empty state with no events", () => {
    render(<AuditLogPanel />);
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
      },
      {
        id: "ev-2",
        timestamp: "2026-07-10T00:00:00Z",
        actor: "",
        actorMethod: "",
        action: "delete",
        status: "denied",
        resource: "service:srv-1",
      },
    ];
    auditState.hasMore = true;
    render(<AuditLogPanel />);

    const rows = screen.getAllByRole("row").slice(1); // drop the header row
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Allowed");
    expect(rows[1]).toHaveTextContent("Denied");
    expect(rows[1]).toHaveTextContent("Unknown"); // empty actor placeholder

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
      },
    ];
    auditState.hasMore = false;
    render(<AuditLogPanel />);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });
});
