import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { DashboardSidebar } from "../dashboard-sidebar";
import type { UseDatabaseResult } from "@/features/databases/hooks/use-database";
import type { DatabaseDetailView } from "@/features/databases/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  billingCapabilities,
  contributorCapabilities,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// w3/m641: a Postgres database's pages swap the workspace nav for Render's
// Postgres sidebar. DashboardSidebar picks it on the route's `databaseId`; it
// reads the same database the layout reads, so the two agree on whether the
// database exists.
const databaseState: UseDatabaseResult = {
  database: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(),
};
const databaseReads = vi.hoisted(() => [] as { id: string; poll?: boolean }[]);
vi.mock("@/features/databases/hooks/use-database", () => ({
  useDatabase: (id: string, options?: { poll?: boolean }) => {
    databaseReads.push({ id, poll: options?.poll });
    return databaseState;
  },
}));
vi.mock(
  "@/common/components/dashboard-layout/use-router-nav-available",
  () => ({
    useRouterNavAvailable: () => false,
  }),
);

function renderAt(pathname: string) {
  const rootRoute = createRootRoute();
  const page = createRoute({
    getParentRoute: () => rootRoute,
    path: "/databases/$databaseId/$",
    component: () => (
      <SidebarProvider>
        <DashboardSidebar />
      </SidebarProvider>
    ),
  });
  const bare = createRoute({
    getParentRoute: () => rootRoute,
    path: "/databases/$databaseId",
    component: () => (
      <SidebarProvider>
        <DashboardSidebar />
      </SidebarProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([page, bare]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

const linkNames = () =>
  screen
    .getAllByRole("link")
    .map((link) => link.textContent)
    .filter(Boolean);
const activeNames = () =>
  screen
    .getAllByRole("link")
    .filter((link) => link.getAttribute("data-active") === "true")
    .map((link) => link.textContent);

beforeEach(() => {
  databaseState.database = {
    id: "dpg-1",
    name: "orders-db",
  } as DatabaseDetailView;
  databaseState.loading = false;
  databaseReads.length = 0;
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("DatabaseSidebar (w3/m641, Render's Postgres sidebar)", () => {
  it("draws the back link, the name, then Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery", async () => {
    renderAt("/databases/dpg-1/plan");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toHaveAttribute("href", "/");
    expect(screen.getByText("orders-db")).toBeInTheDocument();
    expect(screen.getByText("Monitor")).toBeInTheDocument();
    expect(screen.getByText("Manage")).toBeInTheDocument();
    for (const [name, href, iconClass] of [
      ["Info", "/databases/dpg-1/info", "lucide-info"],
      ["Logs", "/databases/dpg-1/logs", "lucide-scroll-text"],
      ["Metrics", "/databases/dpg-1/metrics", "lucide-chart-no-axes-combined"],
      ["Plan", "/databases/dpg-1/plan", "lucide-sliders-horizontal"],
      ["Apps", "/databases/dpg-1/apps", "lucide-app-window"],
      ["Recovery", "/databases/dpg-1/recovery", "lucide-history"],
    ] as const) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      expect(link.querySelector("svg")).toHaveClass(iconClass);
    }
    expect(linkNames()).toEqual([
      "Dashboard",
      "Info",
      "Logs",
      "Metrics",
      "Plan",
      "Apps",
      "Recovery",
    ]);
    // The workspace nav is gone.
    expect(screen.queryByRole("link", { name: "Projects" })).toBeNull();
    // The layout's own read, served from Apollo's cache with no second poll.
    expect(databaseReads.every((read) => read.id === "dpg-1")).toBe(true);
    expect(databaseReads.every((read) => read.poll === false)).toBe(true);
  });

  it.each([
    ["/databases/dpg-1", "Info"],
    ["/databases/dpg-1/info", "Info"],
    ["/databases/dpg-1/logs", "Logs"],
    ["/databases/dpg-1/metrics", "Metrics"],
    ["/databases/dpg-1/plan", "Plan"],
    // Render's plan sub-pages keep Plan open.
    ["/databases/dpg-1/high-availability-upgrade", "Plan"],
    ["/databases/dpg-1/upgrade", "Plan"],
    ["/databases/dpg-1/apps", "Apps"],
    ["/databases/dpg-1/recovery", "Recovery"],
  ])("highlights the open page on %s", async (path, open) => {
    renderAt(path);
    await screen.findByRole("link", { name: "Info" });
    expect(activeNames()).toEqual([open]);
  });

  it("draws every entry while the database loads, so nothing shifts", async () => {
    databaseState.database = null;
    databaseState.loading = true;
    renderAt("/databases/dpg-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual([
      "Dashboard",
      "Info",
      "Logs",
      "Metrics",
      "Plan",
      "Apps",
      "Recovery",
    ]);
    // The name waits for the read; the id stands in.
    expect(screen.getByText("dpg-1")).toBeInTheDocument();
  });

  it("hides the database nav (not the back link) for a database the caller can't see", async () => {
    databaseState.database = null;
    renderAt("/databases/dpg-1/info");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Info" })).toBeNull();
  });
});

// Render drops Logs without `viewDatabaseLogs` and Metrics without
// `viewDatabaseMetrics`; bex's Viewer and Billing roles are denied
// can_view_logs (w9/145), and Billing can_view_activity (w9/m621).
describe("DatabaseSidebar per role (w3/m641)", () => {
  it("drops Logs alone for a Viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    renderAt("/databases/dpg-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual([
      "Dashboard",
      "Info",
      "Metrics",
      "Plan",
      "Apps",
      "Recovery",
    ]);
  });

  it("drops the whole Monitor group for a Billing member", async () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    renderAt("/databases/dpg-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual([
      "Dashboard",
      "Info",
      "Plan",
      "Apps",
      "Recovery",
    ]);
    expect(screen.queryByText("Monitor")).toBeNull();
  });

  it("keeps every entry for a Contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    renderAt("/databases/dpg-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toContain("Logs");
    expect(linkNames()).toContain("Metrics");
  });
});
