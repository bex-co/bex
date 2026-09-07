import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ResourceTable } from "../resource-table";
import type { ResourceRow } from "../../types";
import { formatRelativeAge } from "@/features/services/lib/format";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => (
    <a href="#resource">{children}</a>
  ),
}));
vi.mock("@/features/services/components/service-status-badge", () => ({
  ServiceStatusBadge: () => <span>Running</span>,
}));
vi.mock("@/features/services/components/service-row-actions", () => ({
  ServiceRowActions: () => <button>service options</button>,
}));
vi.mock("@/features/databases/components/database-status-badge", () => ({
  DatabaseStatusBadge: () => <span>Available</span>,
}));
vi.mock("@/features/databases/components/database-row-actions", () => ({
  DatabaseRowActions: () => <button>database options</button>,
  DatabaseRowActionsWithCapabilities: () => <button>database options</button>,
}));
vi.mock("@/features/keyvalue/components/key-value-status-badge", () => ({
  KeyValueStatusBadge: () => <span>Available</span>,
}));
vi.mock("@/features/keyvalue/components/key-value-row-actions", () => ({
  KeyValueRowActions: () => <button>key value options</button>,
  KeyValueRowActionsWithCapabilities: () => <button>key value options</button>,
}));

const createdAt = "2020-01-01T00:00:00Z";
const updatedAt = "2026-07-01T00:00:00Z";
const rows: ResourceRow[] = [
  {
    kind: "service",
    id: "srv-api",
    name: "API",
    createdAt,
    updatedAt,
    runtime: "Node",
    region: "fsn1",
    service: { id: "srv-api", name: "API" } as never,
  },
  {
    kind: "database",
    id: "dpg-main",
    name: "Main DB",
    createdAt,
    updatedAt: null,
    runtime: "PostgreSQL 16",
    region: null,
    database: { id: "dpg-main", name: "Main DB", status: "available" } as never,
  },
  {
    kind: "keyvalue",
    id: "red-cache",
    name: "Cache",
    createdAt,
    updatedAt,
    runtime: "Valkey 8.1",
    region: "fsn1",
    keyValue: { id: "red-cache", name: "Cache", status: "available" } as never,
  },
  {
    kind: "envgroup",
    id: "evg-shared",
    name: "Shared config",
    createdAt,
    updatedAt,
    runtime: null,
    region: null,
    envGroup: { id: "evg-shared", name: "Shared config" } as never,
  },
];

const commonProps = {
  servicePending: null,
  onRunServiceAction: vi.fn(),
  onDatabaseDeleted: vi.fn(),
  onKeyValueDeleted: vi.fn(),
};

describe("ResourceTable", () => {
  it("renders truthful Runtime, Region, and authoritative Updated metadata", () => {
    render(<ResourceTable rows={rows} projectMetadata {...commonProps} />);

    expect(
      screen.getByRole("columnheader", { name: "Runtime" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Region" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Updated" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Node")).toBeInTheDocument();
    expect(screen.getByText("PostgreSQL 16")).toBeInTheDocument();
    expect(screen.getByText("Valkey 8.1")).toBeInTheDocument();
    expect(screen.getAllByLabelText("Not available").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText(formatRelativeAge(updatedAt)).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByText(formatRelativeAge(createdAt)),
    ).not.toBeInTheDocument();
  });

  // w4/047: the Type column must name each service's SPECIFIC kind (matching
  // the detail header + Render), not collapse every service to "Service".
  it("renders each service's specific type label, not a generic Service badge", () => {
    const typed: ResourceRow[] = [
      {
        kind: "service",
        id: "srv-web",
        name: "Web",
        createdAt,
        updatedAt,
        runtime: "Node",
        region: "fsn1",
        service: { id: "srv-web", name: "Web", type: "web_service" } as never,
      },
      {
        kind: "service",
        id: "srv-cron",
        name: "Cron",
        createdAt,
        updatedAt,
        runtime: "Node",
        region: "fsn1",
        service: { id: "srv-cron", name: "Cron", type: "cron_job" } as never,
      },
      {
        kind: "service",
        id: "srv-priv",
        name: "Priv",
        createdAt,
        updatedAt,
        runtime: "Node",
        region: "fsn1",
        service: {
          id: "srv-priv",
          name: "Priv",
          type: "private_service",
        } as never,
      },
    ];
    render(<ResourceTable rows={typed} {...commonProps} />);

    // Each specific label appears (desktop column + mobile-stacked = two nodes).
    expect(screen.getAllByText("Web Service").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Cron Job").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Private Service").length).toBeGreaterThan(0);
    // The generic collapse must be gone for these typed rows.
    expect(screen.queryByText("Service")).not.toBeInTheDocument();
  });

  it("selects one row and the full visible set with stable kind:id identity", async () => {
    const user = userEvent.setup();
    function SelectableTable() {
      const [selected, setSelected] = useState<Set<string>>(() => new Set());
      return (
        <>
          <output aria-label="selection">
            {[...selected].sort().join(",")}
          </output>
          <ResourceTable
            rows={rows.slice(0, 2)}
            projectMetadata
            selectedKeys={selected}
            onSelectedKeysChange={setSelected}
            {...commonProps}
          />
        </>
      );
    }
    render(<SelectableTable />);

    await user.click(screen.getByRole("checkbox", { name: "Select API" }));
    expect(screen.getByLabelText("selection")).toHaveTextContent(
      "service:srv-api",
    );
    await user.click(
      screen.getByRole("checkbox", { name: "Select all visible resources" }),
    );
    expect(screen.getByLabelText("selection")).toHaveTextContent(
      "database:dpg-main,service:srv-api",
    );
  });
});
