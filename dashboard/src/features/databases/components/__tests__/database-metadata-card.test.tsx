import { afterEach, describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
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
    poolerEnabled: false,
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
  // w4/m170: "Unavailable" is never a dead end — the reason sits beside it.
  it("shows why an unavailable database is unavailable, and nothing otherwise", () => {
    renderCard(
      db({
        status: "unavailable",
        statusReason:
          "The database cluster rejected its configuration, so the latest change could not be applied.",
      }),
    );
    expect(screen.getByText(/rejected its configuration/)).toBeInTheDocument();
  });

  // w5/079: the reason is translated by its code; bex-api's sentence is the
  // fallback for a code this dashboard does not know.
  describe("unavailable reason by code", () => {
    afterEach(async () => {
      await i18n.changeLanguage("en");
    });

    it("translates a known code", async () => {
      await i18n.changeLanguage("zh");
      renderCard(
        db({
          status: "unavailable",
          statusReason: "The connection pooler could not be provisioned.",
          statusReasonCode: "PoolerFailed",
        }),
      );
      expect(screen.getByText("无法预配连接池。")).toBeInTheDocument();
      expect(screen.queryByText(/connection pooler/)).not.toBeInTheDocument();
    });

    it("names the allocated size a storage shrink was refused at", () => {
      renderCard(
        db({
          status: "unavailable",
          diskSizeGB: 20,
          statusReason:
            "Postgres storage is grow-only: requested 10 GB is below the allocated 20 GB",
          statusReasonCode: "StorageShrinkRejected",
        }),
      );
      expect(
        screen.getByText(
          "Postgres storage only grows: the disk can't go below its allocated 20 GB.",
        ),
      ).toBeInTheDocument();
    });

    // The list query refreshes a cached database's status without its reason,
    // so one that recovered can still hold the old one (w5/m129).
    it("shows no reason beside a healthy database's status, even a stale one", () => {
      renderCard(
        db({
          status: "available",
          statusReason: "The connection pooler could not be provisioned.",
          statusReasonCode: "PoolerFailed",
        }),
      );
      expect(screen.queryByText(/connection pooler/)).not.toBeInTheDocument();
    });

    it("falls back to bex-api's sentence for an unknown code", async () => {
      await i18n.changeLanguage("zh");
      renderCard(
        db({
          status: "unavailable",
          statusReason: "The database failed to reconcile (SomethingNew).",
          statusReasonCode: "SomethingNew",
        }),
      );
      expect(
        screen.getByText("The database failed to reconcile (SomethingNew)."),
      ).toBeInTheDocument();
    });
  });

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
