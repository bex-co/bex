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
import type { UseKeyValueResult } from "@/features/keyvalue/hooks/use-key-value";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  billingCapabilities,
  contributorCapabilities,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// w3/m649: a Key Value instance's pages swap the workspace nav for Render's
// Key Value sidebar. DashboardSidebar picks it on the route's `keyValueId`; it
// reads the same instance the layout reads, so the two agree on whether the
// instance exists.
const keyValueState: UseKeyValueResult = {
  keyValue: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(),
};
const keyValueReads = vi.hoisted(() => [] as { id: string; poll?: boolean }[]);
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: (id: string, options?: { poll?: boolean }) => {
    keyValueReads.push({ id, poll: options?.poll });
    return keyValueState;
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
  const sidebar = () => (
    <SidebarProvider>
      <DashboardSidebar />
    </SidebarProvider>
  );
  const routes = [
    "/keyvalue/$keyValueId/$",
    "/keyvalue/$keyValueId",
    "/new/redis",
  ].map((path) =>
    createRoute({ getParentRoute: () => rootRoute, path, component: sidebar }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
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
  keyValueState.keyValue = {
    id: "red-1",
    name: "sessions-cache",
  } as KeyValueView;
  keyValueState.loading = false;
  keyValueReads.length = 0;
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("KeyValueSidebar (w3/m649, Render's Key Value sidebar)", () => {
  it("draws the back link, the name, then Info / Monitor: Logs, Metrics", async () => {
    renderAt("/keyvalue/red-1/logs");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toHaveAttribute("href", "/");
    expect(screen.getByText("sessions-cache")).toBeInTheDocument();
    expect(screen.getByText("Monitor")).toBeInTheDocument();
    for (const [name, href, iconClass] of [
      ["Info", "/keyvalue/red-1/info", "lucide-info"],
      ["Logs", "/keyvalue/red-1/logs", "lucide-scroll-text"],
      ["Metrics", "/keyvalue/red-1/metrics", "lucide-chart-no-axes-combined"],
    ] as const) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      expect(link.querySelector("svg")).toHaveClass(iconClass);
    }
    expect(linkNames()).toEqual(["Dashboard", "Info", "Logs", "Metrics"]);
    // The workspace nav is gone.
    expect(screen.queryByRole("link", { name: "Projects" })).toBeNull();
    // The layout's own read, served from Apollo's cache with no second poll.
    expect(keyValueReads.length).toBeGreaterThan(0);
    expect(keyValueReads.every((read) => read.id === "red-1")).toBe(true);
    expect(keyValueReads.every((read) => read.poll === false)).toBe(true);
  });

  it.each([
    ["/keyvalue/red-1", "Info"],
    ["/keyvalue/red-1/info", "Info"],
    ["/keyvalue/red-1/logs", "Logs"],
    ["/keyvalue/red-1/metrics", "Metrics"],
  ])("highlights the open page on %s", async (path, open) => {
    renderAt(path);
    await screen.findByRole("link", { name: "Info" });
    expect(activeNames()).toEqual([open]);
  });

  it("draws every entry while the instance loads, so nothing shifts", async () => {
    keyValueState.keyValue = null;
    keyValueState.loading = true;
    renderAt("/keyvalue/red-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual(["Dashboard", "Info", "Logs", "Metrics"]);
    // The name waits for the read; the id stands in.
    expect(screen.getByText("red-1")).toBeInTheDocument();
  });

  it("hides the instance nav (not the back link) for an instance the caller can't see", async () => {
    keyValueState.keyValue = null;
    renderAt("/keyvalue/red-1/info");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Info" })).toBeNull();
  });

  it("leaves the create page on the workspace sidebar", async () => {
    renderAt("/new/redis");

    expect(
      await screen.findByRole("link", { name: "Projects" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Info" })).toBeNull();
    expect(keyValueReads).toEqual([]);
  });
});

// Render drops Logs without `viewRedisLogs` and Metrics without
// `viewRedisMetrics`; bex's Viewer and Billing roles are denied can_view_logs
// (w9/145), and Billing can_view_activity (w9/m621).
describe("KeyValueSidebar per role (w3/m649)", () => {
  it("drops Logs alone for a Viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    renderAt("/keyvalue/red-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual(["Dashboard", "Info", "Metrics"]);
  });

  it("drops the whole Monitor group for a Billing member", async () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    renderAt("/keyvalue/red-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual(["Dashboard", "Info"]);
    expect(screen.queryByText("Monitor")).toBeNull();
  });

  it("keeps every entry for a Contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    renderAt("/keyvalue/red-1/info");

    await screen.findByRole("link", { name: "Info" });
    expect(linkNames()).toEqual(["Dashboard", "Info", "Logs", "Metrics"]);
  });
});
