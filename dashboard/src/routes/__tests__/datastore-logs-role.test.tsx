import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor } from "@testing-library/react";
import { Route as DatabaseLogsRoute } from "../databases.$databaseId.logs";
import { Route as KeyValueLogsRoute } from "../keyvalue.$keyValueId.logs";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  renderDatabasePages,
  renderKeyValuePages,
} from "@/test/datastore-pages";
import {
  LOGS_REASON,
  billingCapabilities,
  contributorCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// w9/145: Render's role table gives neither a Viewer nor a Billing member
// "View service logs", and a Contributor has it. A Postgres database's or a Key
// Value instance's Logs page (w3/m641, w3/m649; its sidebar entry left out by
// database-sidebar.test.tsx and key-value-sidebar.test.tsx) shows such a
// member the role's reason under its heading, with no log read. The real
// layout and Logs routes run; the data hooks and the header are stubbed.

const { stub, historyReads } = vi.hoisted(() => ({
  stub: () => null,
  historyReads: [] as string[],
}));

// The header's plan badge names the plan from the catalogue (w3/m675), an
// Apollo read this tree has no provider for.
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [],
    loading: false,
    error: undefined,
    retry: vi.fn(),
    retrying: false,
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({
    instanceTypes: [],
    loading: false,
    error: undefined,
    retry: vi.fn(),
    retrying: false,
  }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
vi.mock("@/features/workspaces/hooks/use-adopt-resource-workspace", () => ({
  useAdoptResourceWorkspace: () => false,
}));
// The datastore lifecycle gate reads its projection through Apollo (w4/m578).
vi.mock("@/features/capabilities/hooks/use-lifecycle-gate", async () => {
  const { mockAllowedLifecycleGate } =
    await import("@/test/mocks/lifecycle-gate");
  return mockAllowedLifecycleGate();
});

// --- the Logs page's reads ---
vi.mock("@/features/logs/hooks/use-log-history", () => ({
  useLogHistory: (resource: string) => {
    historyReads.push(resource);
    return {
      lines: [],
      loading: false,
      error: undefined,
      storeUnavailable: false,
      timedOut: false,
      hasMore: false,
      loadingOlder: false,
      loadOlder: () => undefined,
    };
  },
}));
vi.mock("@/features/logs/hooks/use-log-retention", () => ({
  useLogRetentionBound: () => undefined,
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelValues: () => [],
}));
vi.mock("@/features/logs/hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
  useNewLogsWatch: () => ({ available: false, dismiss: () => undefined }),
}));

// --- Postgres ---
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: (id: string) => ({
    database: {
      id,
      name: "db-name",
      ownerId: "tea-1",
      status: "available",
      plan: "free",
      version: "16",
      diskSizeGB: 1,
      createdAt: null,
      public: false,
      suspended: false,
      highAvailabilityEnabled: false,
      databaseName: "app",
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/databases/hooks/use-database-lifecycle", () => ({
  useDatabaseLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/databases/components/database-metadata-card", () => ({
  DatabaseMetadataCard: stub,
}));
vi.mock("@/features/databases/components/database-status-badge", () => ({
  DatabaseStatusBadge: stub,
}));
vi.mock("@/features/databases/components/database-row-actions", () => ({
  DatabaseRowActions: stub,
}));
vi.mock("@/features/databases/components/database-danger-actions", () => ({
  DatabaseDangerActions: stub,
}));
vi.mock("@/features/databases/components/connection-info-panel", () => ({
  ConnectionInfoPanel: stub,
}));
vi.mock("@/common/components/deferred-mount", () => ({
  DeferredMount: () => null,
}));

// --- Key Value ---
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: (id: string) => ({
    keyValue: {
      id,
      name: "kv-name",
      ownerId: "tea-1",
      status: "available",
      plan: "free",
      version: null,
      createdAt: null,
      externalHost: null,
      public: false,
      suspended: false,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/keyvalue/components/key-value-status-badge", () => ({
  KeyValueStatusBadge: stub,
}));

const MOUNTS = [
  {
    name: "Postgres",
    render: () =>
      renderDatabasePages(
        { "/logs": DatabaseLogsRoute },
        "/databases/dpg-main/logs",
      ),
    resource: "dpg-main",
  },
  {
    name: "Key Value",
    render: () =>
      renderKeyValuePages(
        { "/logs": KeyValueLogsRoute },
        "/keyvalue/red-cache/logs",
      ),
    resource: "red-cache",
  },
] as const;

beforeEach(() => {
  historyReads.length = 0;
});

describe.each(MOUNTS)(
  "$name Logs page per role (w9/145, w3/m641, w3/m649)",
  (mount) => {
    it.each([
      ["a Viewer", viewerCapabilities],
      ["a Billing member", billingCapabilities],
    ])(
      "shows %s the role's reason under the page's heading, reading no log",
      async (_, capabilities) => {
        vi.mocked(useCapabilities).mockReturnValue(capabilities());
        await mount.render();

        expect(
          await screen.findByRole("heading", { level: 2, name: "Logs" }),
        ).toBeVisible();
        expect(await screen.findByText(LOGS_REASON)).toBeVisible();
        expect(historyReads).toEqual([]);
      },
    );

    it("reads a Contributor's logs", async () => {
      vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
      await mount.render();

      await screen.findByRole("heading", { level: 2, name: "Logs" });
      await waitFor(() => expect(historyReads).toContain(mount.resource));
      expect(screen.queryByText(LOGS_REASON)).not.toBeInTheDocument();
    });
  },
);
