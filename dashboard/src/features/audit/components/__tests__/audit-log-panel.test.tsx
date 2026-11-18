import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { AuditLogPanel } from "@/features/audit/components/audit-log-panel";
import { AuditLogPanelSkeleton } from "@/features/audit/components/audit-log-panel-skeleton";
import { formatDateTime } from "@/common/lib/format";
import type { UseAuditLogResult } from "@/features/audit/hooks/use-audit-log";
import type { ReactNode } from "react";

vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string, known?: string | null) =>
    known || subject,
}));

vi.mock("@/features/audit/components/audit-log-export", () => ({
  AuditLogExport: ({ subjectId }: { subjectId: string }) => (
    <div>Export control {subjectId}</div>
  ),
}));

// The current workspace's plan: a loading panel previews the state it settles
// to from it (w4/684).
let workspacePlan = "pro";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    currentWorkspace: { id: "tea-primary", plan: workspacePlan },
  }),
}));

// The plan notice links to Billing; render the router Link as its anchor.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({ to, children }: { to: string; children: ReactNode }) => (
    <a href={to}>{children}</a>
  ),
}));

// The panel is presentational (w4/m15): its owner (Workspace Settings since
// w3/m305) runs the query and passes the result down, so tests inject `state`.
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

function renderPanel(overrides: Partial<UseAuditLogResult> = {}) {
  return render(
    <AuditLogPanel
      state={{ ...auditState, ...overrides }}
      ownerId="tea-primary"
    />,
  );
}

beforeEach(() => {
  workspacePlan = "pro";
  auditState.events = [];
  auditState.loading = false;
  auditState.loadingMore = false;
  auditState.error = undefined;
  auditState.forbidden = false;
  auditState.unavailable = false;
  auditState.planRequired = false;
  auditState.hasMore = false;
  loadMore.mockReset();
});

describe("AuditLogPanel", () => {
  it("offers Render's CSV export to an admin whose plan has audit logs (w3/m305)", () => {
    renderPanel();
    expect(screen.getByText("Export control tea-primary")).toBeInTheDocument();
  });

  it("shows the Pro-plan notice with a Billing link, and no export, on a Hobby workspace (w8/m138)", () => {
    renderPanel({ planRequired: true });
    expect(
      screen.getByText("Audit logs require the Pro plan"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Billing" })).toHaveAttribute(
      "href",
      "/billing",
    );
    expect(screen.queryByText(/Export control/)).not.toBeInTheDocument();
  });

  it("offers no export when the audit store is not configured", () => {
    renderPanel({ unavailable: true });
    expect(screen.queryByText(/Export control/)).not.toBeInTheDocument();
  });

  it("keeps the export row while a Pro workspace's first page loads (w4/684)", () => {
    renderPanel({ loading: true });
    expect(screen.getByText("Export control tea-primary")).toBeInTheDocument();
  });

  it("previews the plan notice, and no export or table, while a Hobby workspace loads (w4/684)", () => {
    workspacePlan = "hobby";
    renderPanel({ loading: true });
    expect(
      screen.getByText("Audit logs require the Pro plan"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Export control/)).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders nothing for a non-admin (forbidden) caller", () => {
    auditState.forbidden = true;
    const { container } = renderPanel();
    expect(container).toBeEmptyDOMElement();
  });

  // w4/684: the first page's frame is the ready table's — its six headers, a
  // full page of rows and the Load more slot — so nothing below the card
  // moves when an active workspace's 20 events arrive. The route and inline
  // skeleton draw the same frame under their export row.
  it("frames a full first page while it loads, as the page skeleton does (w4/684)", () => {
    auditState.events = Array.from({ length: 20 }, (_, i) => ({
      id: `ev-${i}`,
      timestamp: "2026-07-11T00:00:00Z",
      actor: "user:alice",
      actorMethod: "session",
      actorId: "",
      actorEmail: "",
      actorDeleted: false,
      sourceIp: "",
      userAgent: "",
      instance: "",
      action: "apps.UpdateService",
      status: "success" as const,
      resource: "service:srv-1",
      targetName: "my-api",
      displayNameFrom: "",
      displayNameTo: "",
      domain: "",
      envGroupId: "",
      envGroupName: "",
      envVarKeys: [],
      relation: "",
      oauthClientId: "",
      oauthAudience: "",
      oauthScopes: [],
      oauthGrantScopes: [],
    }));
    auditState.hasMore = true;
    const ready = renderPanel();
    const headers = (root: ParentNode) =>
      [...root.querySelectorAll("thead th")].map((th) => th.textContent);
    const readyHeaders = headers(ready.container);
    const readyRows = ready.container.querySelectorAll("tbody tr").length;
    expect(readyRows).toBe(20);
    expect(screen.getByRole("button", { name: "Load more" })).toBeVisible();
    ready.unmount();

    const pending = renderPanel({ events: [], hasMore: false, loading: true });
    expect(screen.getByText("Export control tea-primary")).toBeInTheDocument();
    expect(headers(pending.container)).toEqual(readyHeaders);
    expect(
      pending.container.querySelectorAll(
        "tbody tr[data-testid='audit-log-row-placeholder']",
      ),
    ).toHaveLength(readyRows);
    // Each placeholder row is two lines tall in its actor, source and
    // resource cells, as an event row with an auth method, an address and
    // agent, and a named target is.
    const firstRow = pending.container.querySelector("tbody tr")!;
    const placeholderCells = firstRow.querySelectorAll("td");
    for (const twoLineCell of [1, 2, 5]) {
      const cell = placeholderCells[twoLineCell];
      expect(cell.querySelectorAll("[data-slot='skeleton']")).toHaveLength(2);
    }
    // The Load more slot follows the table.
    const frame = pending.container.querySelector(
      "[data-testid='audit-log-table-frame']",
    )!;
    expect(frame.lastElementChild).toHaveAttribute("data-slot", "skeleton");
    expect(frame.lastElementChild).toHaveClass("h-8");
    pending.unmount();

    const skeleton = render(<AuditLogPanelSkeleton />);
    expect(
      skeleton.container.querySelector(
        "[data-testid='audit-log-export-frame']",
      ),
    ).not.toBeNull();
    expect(headers(skeleton.container)).toEqual(readyHeaders);
    expect(skeleton.container.querySelectorAll("tbody tr")).toHaveLength(
      readyRows,
    );
  });

  it("previews the plan notice in the page skeleton for a Hobby workspace (w4/684)", () => {
    workspacePlan = "hobby";
    const { container } = render(<AuditLogPanelSkeleton />);
    expect(container).toHaveTextContent("Audit logs require the Pro plan");
    expect(container.querySelector("table")).toBeNull();
    expect(
      container.querySelector("[data-testid='audit-log-export-frame']"),
    ).toBeNull();
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
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "update",
        status: "success",
        resource: "workspace:tea-1",
        targetName: "my-api",
        displayNameFrom: "api",
        displayNameTo: "Checkout API",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: [],
      },
      {
        id: "ev-2",
        timestamp: "2026-07-10T00:00:00Z",
        actor: "user:alice",
        actorMethod: "oauth2",
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "delete",
        status: "denied",
        resource: "service:srv-1",
        targetName: "",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "can_operate",
        oauthClientId: "dcr-agent",
        oauthAudience: "https://api.bex.co/mcp",
        oauthScopes: ["bex.read"],
        oauthGrantScopes: [],
      },
    ];
    auditState.hasMore = true;
    renderPanel();

    const rows = screen.getAllByRole("row").slice(1); // drop the header row
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Allowed");
    expect(rows[1]).toHaveTextContent("Denied");
    // Friendly target name (w10/m5): shown alongside the raw id when the row
    // carries one; a pre-0038 row (empty targetName) keeps the id-only cell.
    expect(rows[0]).toHaveTextContent("my-api");
    expect(rows[0]).toHaveTextContent("workspace:tea-1");
    // A rename names both names (w5/183); other rows carry no change line.
    expect(rows[0]).toHaveTextContent("Name api → Checkout API");
    expect(rows[1]).not.toHaveTextContent("Name ");
    expect(rows[1]).not.toHaveTextContent("my-api");
    expect(rows[1]).toHaveTextContent("service:srv-1");
    expect(rows[1]).toHaveTextContent("dcr-agent");
    expect(rows[1]).toHaveTextContent("bex.read");
    // Timestamps render via the shared formatter ("July 11, 2026 at …") —
    // computed through the helper so this holds in any runner timezone.
    expect(rows[0]).toHaveTextContent(formatDateTime("2026-07-11T00:00:00Z")!);

    const loadMoreButton = screen.getByRole("button", { name: "Load more" });
    expect(loadMoreButton).toBeInTheDocument();
    loadMoreButton.click();
    expect(loadMore).toHaveBeenCalled();
  });

  it("keeps every row at two lines, so the loading frame's row height holds (w4/684)", () => {
    // An OAuth-delegated actor and a renamed, named target: the two cells that
    // once drew a third line under their first.
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "user:alice",
        actorMethod: "oauth2",
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "update",
        status: "success",
        resource: "service:srv-1",
        targetName: "my-api",
        displayNameFrom: "api",
        displayNameTo: "Checkout API",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "429024F5-agent",
        oauthAudience: "https://api.bex.co/mcp",
        oauthScopes: ["bex.read", "bex.write"],
        oauthGrantScopes: [],
      },
    ];
    renderPanel();

    const [row] = screen.getAllByRole("row").slice(1);
    const cells = within(row!).getAllByRole("cell");
    for (const cell of cells) {
      // One text-sm first line plus at most one block (second-line) span.
      expect(cell.querySelectorAll(".block").length).toBeLessThanOrEqual(1);
    }
    expect(cells[1]).toHaveTextContent(
      "oauth2 · 429024F5-agent · bex.read bex.write",
    );
    expect(cells[5]).toHaveTextContent(
      "service:srv-1 · Name api → Checkout API",
    );
  });

  // w9/m450: who acted is named by the email the row carries, and where from
  // by the address and the client in short, the full agent one hover away. A
  // row no request caused (the system's own) has no source.
  it("names who acted and from where (w9/m450)", () => {
    const chrome =
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36";
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "0b7f0e5c-4ad7-4b1f-9d5e-0d1f2a3b4c5d",
        actorMethod: "session",
        actorId: "usr-00000000000000000007",
        actorEmail: "ada@example.com",
        actorDeleted: false,
        sourceIp: "198.51.100.23",
        userAgent: chrome,
        instance: "",
        action: "ViewEnvVarValuesEvent",
        status: "success",
        resource: "service:srv-1",
        targetName: "my-api",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: [],
      },
      {
        id: "ev-2",
        timestamp: "2026-07-10T00:00:00Z",
        actor: "",
        actorMethod: "",
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "UpdateWorkspacePaymentMethodEvent",
        status: "success",
        resource: "workspace:tea-primary",
        targetName: "",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: [],
      },
    ];
    renderPanel();

    const [person, system] = screen.getAllByRole("row").slice(1);
    const cells = within(person!).getAllByRole("cell");
    expect(cells[1]).toHaveTextContent("ada@example.com");
    expect(cells[2]).toHaveTextContent("198.51.100.23");
    expect(within(cells[2]!).getByText("Chrome 131 · macOS")).toHaveAttribute(
      "title",
      chrome,
    );
    // Still two lines at most, the loading frame's row height.
    expect(cells[2]!.querySelectorAll(".block").length).toBeLessThanOrEqual(1);
    expect(within(system!).getAllByRole("cell")[2]).toHaveTextContent(/^—$/);
  });

  it("names the variables an env var row changed, under Render's event (w9/m449)", () => {
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "user:alice",
        actorMethod: "session",
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "UpdateEnvVarsEvent",
        status: "success",
        resource: "service:srv-1",
        targetName: "my-api",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: ["API_KEY", "DB_URL"],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: [],
      },
    ];
    renderPanel();

    const cells = within(screen.getAllByRole("row")[1]!).getAllByRole("cell");
    expect(cells[3]).toHaveTextContent("UpdateEnvVarsEvent");
    expect(cells[5]).toHaveTextContent("Variables API_KEY, DB_URL");
  });

  // w4/m454: a member's grant to an agent names the client and what it may do,
  // in the two lines every row keeps.
  it("names an OAuth access grant's client and granted scopes", () => {
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "user:alice",
        actorMethod: "session",
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "GrantOAuthAccessEvent",
        status: "success",
        resource: "oauth-client:dcr-claude",
        targetName: "Claude Code",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: ["openid", "offline_access", "bex.read"],
      },
    ];
    renderPanel();

    const [row] = screen.getAllByRole("row").slice(1);
    const cells = within(row!).getAllByRole("cell");
    const action = cells.find((cell) =>
      cell.textContent?.includes("GrantOAuthAccessEvent"),
    );
    const resource = cells[cells.length - 1]!;
    expect(action).toBeDefined();
    expect(resource).toHaveTextContent("Claude Code");
    expect(resource).toHaveTextContent(
      "oauth-client:dcr-claude · Scopes openid offline_access bex.read",
    );
    expect(resource.querySelectorAll(".block").length).toBeLessThanOrEqual(1);
  });

  it("names the instance a shell session ran on (w9/m451)", () => {
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-11-16T00:00:00Z",
        actor: "0b7f0e5c-4ad7-4b1f-9d5e-0d1f2a3b4c5d",
        actorMethod: "ssh",
        actorId: "usr-00000000000000000007",
        actorEmail: "ada@example.com",
        actorDeleted: false,
        sourceIp: "198.51.100.23",
        userAgent: "",
        instance: "srv-d5t5d4v8g3c73f5m9peg-web-rs-pod01",
        action: "StartShellEvent",
        status: "success",
        resource: "service:srv-d5t5d4v8g3c73f5m9peg",
        targetName: "",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: [],
      },
    ];
    renderPanel();

    const cells = within(screen.getAllByRole("row")[1]!).getAllByRole("cell");
    expect(cells[3]).toHaveTextContent("StartShellEvent");
    expect(cells[5]).toHaveTextContent(
      "Instance srv-d5t5d4v8g3c73f5m9peg-web-rs-pod01",
    );
  });

  it("hides 'Load more' once the last page is exhausted", () => {
    auditState.events = [
      {
        id: "ev-1",
        timestamp: "2026-07-11T00:00:00Z",
        actor: "user:alice",
        actorMethod: "session",
        actorId: "",
        actorEmail: "",
        actorDeleted: false,
        sourceIp: "",
        userAgent: "",
        instance: "",
        action: "update",
        status: "success",
        resource: "workspace:tea-1",
        targetName: "",
        displayNameFrom: "",
        displayNameTo: "",
        domain: "",
        envGroupId: "",
        envGroupName: "",
        envVarKeys: [],
        relation: "",
        oauthClientId: "",
        oauthAudience: "",
        oauthScopes: [],
        oauthGrantScopes: [],
      },
    ];
    auditState.hasMore = false;
    renderPanel();
    expect(
      screen.queryByRole("button", { name: "Load more" }),
    ).not.toBeInTheDocument();
  });
});
