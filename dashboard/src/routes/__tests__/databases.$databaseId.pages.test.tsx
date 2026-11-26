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

const { card, storage, catalog } = vi.hoisted(() => ({
  card: (name: string) => () => <div data-card={name} />,
  // The almost-full warning's usage read and the plan catalog (w3/m656).
  storage: {
    current: null as {
      storageUsedPercent: number | null;
      storageTotal: string | null;
    } | null,
  },
  catalog: {
    current: [] as { id: string; storageGB: number; maxStorageGB: number }[],
  },
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
  useDatabaseInstanceTypes: () => ({ instanceTypes: catalog.current }),
}));
vi.mock("@/features/databases/hooks/use-database-storage", () => ({
  useDatabaseStorage: () => storage.current,
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
  storage.current = null;
  catalog.current = [];
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

// w3/m656: Render's almost-full warning leads Info and Plan, above Info's
// title, and Recovery under its title; no other page draws it.
describe("a Postgres database's almost-full warning (w3/m656)", () => {
  const GIB = 1024 ** 3;
  const ALMOST_FULL = {
    storageUsedPercent: 90,
    storageTotal: String(10 * GIB),
  };
  const WARNING = /^Your database is almost full\./;

  const alert = () => screen.queryByRole("alert");
  const precedes = (first: Element, second: Element) =>
    Boolean(
      first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING,
    );

  it.each([
    ["/databases/dpg-main", "Info"],
    ["/databases/dpg-main/info", "Info"],
    ["/databases/dpg-main/plan", "Plan"],
  ])("leads %s, above its heading", async (href, heading) => {
    storage.current = ALMOST_FULL;
    await renderDatabasePages(ROUTES, href);

    const title = await screen.findByRole("heading", {
      level: 2,
      name: heading,
    });
    const warning = await screen.findByRole("alert");
    expect(warning).toHaveTextContent(WARNING);
    expect(precedes(warning, title)).toBe(true);
  });

  it("leads Recovery's card, under its heading", async () => {
    storage.current = ALMOST_FULL;
    await renderDatabasePages(ROUTES, "/databases/dpg-main/recovery");

    const title = await screen.findByRole("heading", {
      level: 2,
      name: "Recovery",
    });
    const warning = await screen.findByRole("alert");
    await waitFor(() => expect(cards()).toEqual(["recovery"]));
    const recovery = document.querySelector("[data-card=recovery]")!;
    expect(precedes(title, warning) && precedes(warning, recovery)).toBe(true);
  });

  it.each(["logs", "metrics", "apps"])("is not on %s", async (page) => {
    storage.current = ALMOST_FULL;
    await renderDatabasePages(ROUTES, `/databases/dpg-main/${page}`);

    await screen.findByRole("heading", { level: 2 });
    await waitFor(() => expect(cards().length).toBeGreaterThan(0));
    expect(alert()).toBeNull();
  });

  it.each([
    [
      "below Render's line",
      { storageUsedPercent: 80, storageTotal: String(10 * GIB) },
    ],
    [
      "with 5 GiB or more left",
      { storageUsedPercent: 95, storageTotal: String(200 * GIB) },
    ],
    [
      "without a usage read",
      { storageUsedPercent: null, storageTotal: String(10 * GIB) },
    ],
    ["before the read answers", null],
  ])("does not show %s", async (_name, usage) => {
    storage.current = usage;
    await renderDatabasePages(ROUTES, "/databases/dpg-main/info");

    await waitFor(() => expect(cards()).toEqual(INFO));
    expect(alert()).toBeNull();
  });

  it("links the Plan page's storage control, Render's /plan#storage", async () => {
    storage.current = ALMOST_FULL;
    catalog.current = [{ id: "basic-1gb", storageGB: 5, maxStorageGB: 16384 }];
    await renderDatabasePages(ROUTES, "/databases/dpg-main/info");

    const link = await screen.findByRole("link", {
      name: "increase its storage",
    });
    expect(link).toHaveAttribute("href", "/databases/dpg-main/plan#storage");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Your database is almost full. This may lead to errors reading from or writing to your database and can prevent automated backups. Consider clearing some space in your database or increase its storage.",
    );
  });

  it("links a fixed plan's picker instead", async () => {
    storage.current = ALMOST_FULL;
    catalog.current = [{ id: "basic-1gb", storageGB: 1, maxStorageGB: 1 }];
    await renderDatabasePages(ROUTES, "/databases/dpg-main/plan");

    const link = await screen.findByRole("link", {
      name: "upgrade to a compute plan with more storage",
    });
    expect(link).toHaveAttribute("href", "/databases/dpg-main/plan");
  });

  it("asks support once the storage is at its plan's maximum", async () => {
    storage.current = ALMOST_FULL;
    catalog.current = [{ id: "basic-1gb", storageGB: 5, maxStorageGB: 10 }];
    await renderDatabasePages(ROUTES, "/databases/dpg-main/info");

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /or contact support to create a custom compute plan\.$/,
    );
    expect(screen.queryByRole("link", { name: /storage|plan/ })).toBeNull();
  });
});
