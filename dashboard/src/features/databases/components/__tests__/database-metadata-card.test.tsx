import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { DatabaseDetailView } from "@/features/databases/types";

// The card reaches Apollo only through the instance-type catalog; the name row
// and the version control own their own hooks and their own tests, so both are
// stubbed at the module boundary (the pattern the route test uses).
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [
      {
        id: "basic-1gb",
        name: "Basic 1 GB",
        cpu: "0.5",
        memory: "1 GB",
        storageGB: 16,
      },
    ],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/databases/components/database-name-row", () => ({
  DatabaseNameRow: ({ database }: { database: { name: string } }) => (
    <div>{database.name}</div>
  ),
}));
vi.mock("@/features/databases/components/database-version-control", () => ({
  DatabaseVersionControl: () => <div data-testid="version-control" />,
}));

import { DatabaseMetadataCard } from "@/features/databases/components/database-metadata-card";

function db(overrides: Partial<DatabaseDetailView> = {}): DatabaseDetailView {
  return {
    id: "dpg-orders",
    name: "orders",
    status: "available",
    plan: "basic-1gb",
    version: "16",
    diskSizeGB: 16,
    createdAt: null,
    public: false,
    suspended: "not_suspended",
    databaseName: "orders",
    databaseUser: "orders_user",
    highAvailabilityEnabled: false,
    diskAutoscalingEnabled: false,
    readReplicas: [],
    externalHost: null,
    backupsEnabled: false,
    ...overrides,
  };
}

function renderCard(database: DatabaseDetailView) {
  render(
    <DatabaseMetadataCard
      database={database}
      onVersionChanged={vi.fn()}
      onRenamed={vi.fn()}
    />,
  );
}

describe("DatabaseMetadataCard", () => {
  it("reads the suspended status, not a stale ready label (w1/m159)", () => {
    // deriveStatus prefers the suspended flag; with w5/061 the wire status is
    // also "suspended", so either input must keep this row aligned with the badge.
    renderCard(db({ suspended: "suspended" }));

    expect(screen.getByText("Suspended")).toBeInTheDocument();
    expect(screen.queryByText("available")).not.toBeInTheDocument();
  });

  it("reads the plan's display name, not its id (w1/m159)", () => {
    renderCard(db());

    expect(screen.getByText("Basic 1 GB")).toBeInTheDocument();
    expect(screen.queryByText("basic-1gb")).not.toBeInTheDocument();
  });

  it("keeps a running instance's status readable", () => {
    renderCard(db());

    expect(screen.getByText("Available")).toBeInTheDocument();
  });
});
