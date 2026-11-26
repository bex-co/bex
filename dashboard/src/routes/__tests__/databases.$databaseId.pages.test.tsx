import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { screen, waitFor } from "@testing-library/react";
import { Route as IndexRoute } from "../databases.$databaseId.index";
import { Route as InfoRoute } from "../databases.$databaseId.info";
import { Route as LogsRoute } from "../databases.$databaseId.logs";
import { Route as MetricsRoute } from "../databases.$databaseId.metrics";
import { Route as PlanRoute } from "../databases.$databaseId.plan";
import { Route as AppsRoute } from "../databases.$databaseId.apps";
import { Route as RecoveryRoute } from "../databases.$databaseId.recovery";
import { renderDatabasePages } from "@/test/datastore-pages";

// w3/m641: a Postgres database's one long Overview, its section rail and its
// Overview/Logs tab bar became Render's pages, each at its own URL under one
// layout. Each page opens with its heading and holds exactly the cards its
// Render counterpart holds (docs/render-artifacts/postgres-dashboard.md §
// Page map), and no other page's. The real layout and page routes run; the
// data hooks are stubbed and every card stands in as a named marker.

const { card } = vi.hoisted(() => ({
  card: (name: string) => () => <div data-card={name} />,
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
vi.mock("@/features/workspaces/hooks/use-adopt-resource-workspace", () => ({
  useAdoptResourceWorkspace: () => false,
}));
vi.mock("@/common/components/deferred-mount", () => ({
  DeferredMount: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: (id: string) => ({
    database: {
      id,
      name: "orders-db",
      ownerId: "tea-1",
      status: "available",
      plan: "basic-1gb",
      version: "16",
      diskSizeGB: 10,
      createdAt: null,
      public: false,
      suspended: "not_suspended",
      highAvailabilityEnabled: false,
      readReplicas: [],
      databaseName: "orders",
      backupsEnabled: true,
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/databases/hooks/use-database-lifecycle", () => ({
  useDatabaseLifecycle: () => ({ pending: null, run: vi.fn() }),
}));
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({ instanceTypes: [] }),
}));
vi.mock("@/features/databases/components/database-status-badge", () => ({
  DatabaseStatusBadge: () => null,
}));
vi.mock("@/features/databases/components/database-row-actions", () => ({
  DatabaseRowActions: () => null,
}));
vi.mock("@/features/databases/components/database-metadata-card", () => ({
  DatabaseMetadataCard: card("details"),
}));
vi.mock("@/features/databases/components/connection-info-panel", () => ({
  ConnectionInfoPanel: card("connections"),
}));
vi.mock("@/features/databases/components/access-control-panel", () => ({
  AccessControlPanel: card("access-control"),
}));
vi.mock("@/features/databases/components/sql-console", () => ({
  SQLConsole: card("sql-console"),
}));
vi.mock("@/features/databases/components/database-danger-actions", () => ({
  DatabaseDangerActions: card("danger-zone"),
}));
vi.mock("@/features/logs/components/datastore-log-viewer", () => ({
  DatastoreLogViewer: card("logs"),
}));
vi.mock("@/features/metrics/components/datastore-metrics-panel", () => ({
  DatastoreMetricsPanel: card("metrics"),
}));
vi.mock("@/features/databases/components/insights-panel", () => ({
  InsightsPanel: card("insights"),
}));
vi.mock("@/features/databases/components/database-plan-section", () => ({
  DatabasePlanSection: card("plan"),
}));
vi.mock("@/features/databases/components/ha-panel", () => ({
  HAPanel: card("high-availability"),
}));
vi.mock("@/features/databases/components/admin-apps-panel", () => ({
  AdminAppsPanel: card("apps"),
}));
vi.mock("@/features/databases/components/recovery-panel", () => ({
  RecoveryPanel: card("recovery"),
}));

const ROUTES = {
  "/": IndexRoute,
  "/info": InfoRoute,
  "/logs": LogsRoute,
  "/metrics": MetricsRoute,
  "/plan": PlanRoute,
  "/apps": AppsRoute,
  "/recovery": RecoveryRoute,
};

const INFO = [
  "details",
  "connections",
  "access-control",
  "sql-console",
  "danger-zone",
];

const cards = () =>
  [...document.querySelectorAll<HTMLElement>("[data-card]")].map(
    (element) => element.dataset.card,
  );

beforeEach(() => {
  document.body.innerHTML = "";
});

describe("a Postgres database's pages (w3/m641)", () => {
  it.each([
    ["/databases/dpg-main", "Info", INFO],
    ["/databases/dpg-main/info", "Info", INFO],
    ["/databases/dpg-main/logs", "Logs", ["logs"]],
    ["/databases/dpg-main/metrics", "Metrics", ["metrics", "insights"]],
    ["/databases/dpg-main/plan", "Plan", ["plan", "high-availability"]],
    ["/databases/dpg-main/apps", "Apps", ["apps"]],
    ["/databases/dpg-main/recovery", "Recovery", ["recovery"]],
  ])("%s opens %s with only its cards", async (href, heading, expected) => {
    await renderDatabasePages(ROUTES, href);

    expect(
      await screen.findByRole("heading", { level: 1, name: "orders-db" }),
    ).toBeVisible();
    expect(
      await screen.findByRole("heading", { level: 2, name: heading }),
    ).toBeVisible();
    await waitFor(() => expect(cards()).toEqual(expected));
    // The old tab bar and section rail are gone.
    expect(screen.queryByRole("button", { name: "Overview" })).toBeNull();
    expect(screen.queryByRole("navigation")).toBeNull();
  });

  // A full load of an old Overview anchor: the router keeps the server's
  // match on hydration, so Info itself sends a moved card's anchor on.
  it("sends a moved card's anchor on from Info, the hash kept", async () => {
    const router = await renderDatabasePages(
      { "/info": InfoRoute, "/recovery": RecoveryRoute },
      "/databases/dpg-main/info#recovery",
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe(
        "/databases/dpg-main/recovery",
      ),
    );
    expect(router.state.location.hash).toBe("recovery");
    await waitFor(() => expect(cards()).toEqual(["recovery"]));
  });

  it("keeps an anchor of a card still on Info", async () => {
    const router = await renderDatabasePages(
      { "/info": InfoRoute },
      "/databases/dpg-main/info#sql-console",
    );

    await waitFor(() => expect(cards()).toEqual(INFO));
    expect(router.state.location.pathname).toBe("/databases/dpg-main/info");
    expect(document.getElementById("sql-console")).not.toBeNull();
  });
});
