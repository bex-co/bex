import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { ServiceSidebar } from "../service-sidebar";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// The sidebar reads the same single-service hook as the detail shell, so the
// two agree on whether the service exists (w1/m45).
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

function renderAt(pathname: string) {
  const rootRoute = createRootRoute();
  const serviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/$",
    component: () => (
      <SidebarProvider>
        <ServiceSidebar serviceId="srv-1" />
      </SidebarProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([serviceRoute]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = {
    id: "srv-1",
    name: "storefront-api",
    type: "web_service",
  } as ServiceView;
  serverState.loading = false;
});

describe("ServiceSidebar (w1/m45 — Render's resource-scoped service nav)", () => {
  it("renders Render's grouping: top items, Monitor, Manage", async () => {
    renderAt("/services/srv-1/logs");

    // Back link + service name replace the global nav.
    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toHaveAttribute("href", "/");
    expect(screen.getByText("storefront-api")).toBeInTheDocument();

    // Render's groups (docs/render-artifacts/dashboard-routes.md § Sidebar
    // navigation): Monitor{Logs, Metrics}, Manage{Environment, Scaling, Plan}.
    // No Deploys entry — the unified Events page IS the deploy history (w1/m47).
    expect(screen.getByText("Monitor")).toBeInTheDocument();
    expect(screen.getByText("Manage")).toBeInTheDocument();
    for (const [name, href] of [
      ["Events", "/services/srv-1/events"],
      ["Settings", "/services/srv-1/settings"],
      ["Logs", "/services/srv-1/logs"],
      ["Metrics", "/services/srv-1/metrics"],
      ["Environment", "/services/srv-1/env"],
      ["Scaling", "/services/srv-1/scaling"],
      ["Plan", "/services/srv-1/plan"],
    ] as const) {
      expect(screen.getByRole("link", { name })).toHaveAttribute("href", href);
    }
    expect(
      screen.queryByRole("link", { name: "Deploys" }),
    ).not.toBeInTheDocument();
  });

  it("hides the service nav (not the back link) for a service the caller can't see", async () => {
    serverState.service = null;
    renderAt("/services/srv-1/logs");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Events" })).not.toBeInTheDocument();
    // The header still names what was asked for (the id, no resolved name).
    expect(screen.getByText("srv-1")).toBeInTheDocument();
  });

  it("keeps the nav while the service is still loading (no empty flash)", async () => {
    serverState.service = null;
    serverState.loading = true;
    renderAt("/services/srv-1/logs");

    expect(await screen.findByRole("link", { name: "Events" })).toBeInTheDocument();
  });
});
