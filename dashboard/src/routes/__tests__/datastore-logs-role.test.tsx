import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouteComponent,
} from "@tanstack/react-router";
import { Route as DatabaseLogsRoute } from "../databases.$databaseId.logs";
import { Route as KeyValueRoute } from "../keyvalue.$keyValueId";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { renderDatabasePages } from "@/test/database-pages";
import {
  LOGS_REASON,
  billingCapabilities,
  contributorCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// w9/145: Render's role table gives neither a Viewer nor a Billing member
// "View service logs", and a Contributor has it. On a Key Value page, such a
// member gets no Logs tab, and a deep link to the tab names it over the role's
// reason, with no log read; a Postgres database's Logs page (w3/m641, its
// sidebar entry left out by database-sidebar.test.tsx) shows the role's
// reason, with no log read. The real route components and their tab rows run;
// the data hooks and the Overview's panels are stubbed.

const { stub, historyReads } = vi.hoisted(() => ({
  stub: () => null,
  historyReads: [] as string[],
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

// --- the Logs tab's reads ---
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
vi.mock("@/features/keyvalue/components/key-value-metadata-card", () => ({
  KeyValueMetadataCard: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-status-badge", () => ({
  KeyValueStatusBadge: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-danger-actions", () => ({
  KeyValueDangerActions: stub,
}));
vi.mock("@/features/keyvalue/components/connection-info-panel", () => ({
  ConnectionInfoPanel: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-networking-panel", () => ({
  KeyValueNetworkingPanel: stub,
}));
vi.mock("@/features/keyvalue/components/key-value-plan-section", () => ({
  KeyValuePlanSection: stub,
}));
vi.mock(
  "@/features/keyvalue/components/key-value-maxmemory-policy-section",
  () => ({ KeyValueMaxmemoryPolicySection: stub }),
);
vi.mock(
  "@/features/keyvalue/components/key-value-persistence-mode-section",
  () => ({ KeyValuePersistenceModeSection: stub }),
);
vi.mock("@/features/keyvalue/components/key-value-detail-navigation", () => ({
  KeyValueDetailNavigation: stub,
}));

const MOUNTS = [
  {
    name: "Key Value",
    route: KeyValueRoute,
    path: "/keyvalue/$keyValueId",
    base: "/keyvalue/red-cache",
    resource: "red-cache",
    nav: "Key Value detail navigation",
    tabs: ["Overview", "Metrics", "Logs"],
  },
] as const;

type Mount = (typeof MOUNTS)[number];

async function renderAt(mount: Mount, href: string) {
  const root = createRootRoute();
  const page = createRoute({
    getParentRoute: () => root,
    path: mount.path,
    validateSearch: mount.route.options.validateSearch,
    component: mount.route.options.component as RouteComponent,
  });
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  render(<RouterProvider router={router} />);
}

beforeEach(() => {
  historyReads.length = 0;
});

describe.each(MOUNTS)("$name Logs tab per role (w9/145)", (mount) => {
  const tabNames = async () =>
    within(await screen.findByRole("navigation", { name: mount.nav }))
      .getAllByRole("button")
      .map((tab) => tab.textContent);
  const withoutLogs = mount.tabs.filter((tab) => tab !== "Logs");

  it.each([
    ["a Viewer", viewerCapabilities],
    ["a Billing member", billingCapabilities],
  ])("leaves the Logs tab out for %s", async (_, capabilities) => {
    vi.mocked(useCapabilities).mockReturnValue(capabilities());
    await renderAt(mount, mount.base);

    expect(await tabNames()).toEqual(withoutLogs);
  });

  it.each([
    ["a Viewer", viewerCapabilities],
    ["a Billing member", billingCapabilities],
  ])(
    "shows %s's deep link to Logs the role's reason under its tab, reading no log",
    async (_, capabilities) => {
      vi.mocked(useCapabilities).mockReturnValue(capabilities());
      await renderAt(mount, `${mount.base}?tab=logs`);

      expect(await tabNames()).toEqual(mount.tabs);
      expect(screen.getByRole("button", { name: "Logs" })).toHaveAttribute(
        "aria-current",
        "page",
      );
      expect(await screen.findByText(LOGS_REASON)).toBeVisible();
      expect(historyReads).toEqual([]);
    },
  );

  it("keeps the Logs tab for a Contributor, whose deep link reads the logs", async () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    await renderAt(mount, `${mount.base}?tab=logs`);

    expect(await tabNames()).toEqual(mount.tabs);
    expect(historyReads).toContain(mount.resource);
    expect(screen.queryByText(LOGS_REASON)).not.toBeInTheDocument();
  });
});

describe("Postgres Logs page per role (w9/145, w3/m641)", () => {
  it.each([
    ["a Viewer", viewerCapabilities],
    ["a Billing member", billingCapabilities],
  ])(
    "shows %s the role's reason under the page's heading, reading no log",
    async (_, capabilities) => {
      vi.mocked(useCapabilities).mockReturnValue(capabilities());
      await renderDatabasePages(
        { "/logs": DatabaseLogsRoute },
        "/databases/dpg-main/logs",
      );

      expect(
        await screen.findByRole("heading", { level: 2, name: "Logs" }),
      ).toBeVisible();
      expect(await screen.findByText(LOGS_REASON)).toBeVisible();
      expect(historyReads).toEqual([]);
    },
  );

  it("reads a Contributor's logs", async () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    await renderDatabasePages(
      { "/logs": DatabaseLogsRoute },
      "/databases/dpg-main/logs",
    );

    await screen.findByRole("heading", { level: 2, name: "Logs" });
    await waitFor(() => expect(historyReads).toContain("dpg-main"));
    expect(screen.queryByText(LOGS_REASON)).not.toBeInTheDocument();
  });
});
