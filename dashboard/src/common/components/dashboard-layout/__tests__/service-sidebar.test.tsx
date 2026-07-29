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

    // Navigation: top items {Deploys, Settings}, Monitor {Events, Logs,
    // Metrics}, Manage {Environment, Shell, Scaling, Plan}.
    expect(screen.getByText("Monitor")).toBeInTheDocument();
    expect(screen.getByText("Manage")).toBeInTheDocument();
    for (const [name, href, iconClass] of [
      ["Deploys", "/services/srv-1/deploys", "lucide-rocket"],
      ["Settings", "/services/srv-1/settings", "lucide-settings"],
      ["Events", "/services/srv-1/events", "lucide-activity"],
      ["Logs", "/services/srv-1/logs", "lucide-scroll-text"],
      ["Metrics", "/services/srv-1/metrics", "lucide-chart-no-axes-combined"],
      ["Environment", "/services/srv-1/env", "lucide-braces"],
      ["Shell", "/services/srv-1/shell", "lucide-square-terminal"],
      ["Scaling", "/services/srv-1/scaling", "lucide-scaling"],
      ["Plan", "/services/srv-1/plan", "lucide-credit-card"],
    ] as const) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      expect(link.querySelector("svg")).toHaveClass(iconClass);
    }

    expect(
      screen
        .getAllByRole("link")
        .map((link) => link.textContent)
        .filter(Boolean),
    ).toEqual([
      "Dashboard",
      "Deploys",
      "Settings",
      "Events",
      "Logs",
      "Metrics",
      "Environment",
      "Shell",
      "Scaling",
      "Plan",
    ]);
  });

  it("hides the service nav (not the back link) for a service the caller can't see", async () => {
    serverState.service = null;
    renderAt("/services/srv-1/logs");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Events" }),
    ).not.toBeInTheDocument();
    // The header still names what was asked for (the id, no resolved name).
    expect(screen.getByText("srv-1")).toBeInTheDocument();
  });

  it("keeps the nav while the service is still loading (no empty flash)", async () => {
    serverState.service = null;
    serverState.loading = true;
    renderAt("/services/srv-1/logs");

    expect(
      await screen.findByRole("link", { name: "Events" }),
    ).toBeInTheDocument();
    // Type-specific entries wait for the type: a static site must never flash
    // Shell/Scaling/Plan (w5/m48/t001).
    expect(
      screen.queryByRole("link", { name: "Shell" }),
    ).not.toBeInTheDocument();
  });

  it("type-gates a static site: Events-first, no Deploys/Logs/Shell/Scaling/Plan, adds Redirects + Headers (w5/m48, w5/m57)", async () => {
    serverState.service = {
      id: "srv-1",
      name: "docs-site",
      type: "static_site",
    } as ServiceView;
    renderAt("/services/srv-1/settings");

    expect(
      await screen.findByRole("link", { name: "Redirects/Rewrites" }),
    ).toHaveAttribute("href", "/services/srv-1/redirects");
    expect(screen.getByRole("link", { name: "Headers" })).toHaveAttribute(
      "href",
      "/services/srv-1/headers",
    );

    // Render's static sidebar leads with Events and has no Deploys tab (deploy
    // history/detail is reached from the Events feed) nor any runtime-instance
    // tab (w5/m57).
    expect(
      screen.queryByRole("link", { name: "Deploys" }),
    ).not.toBeInTheDocument();
    expect(
      screen
        .getAllByRole("link")
        .map((link) => link.textContent)
        .filter(Boolean),
    ).toEqual([
      "Dashboard",
      "Events",
      "Settings",
      "Metrics",
      "Environment",
      "Redirects/Rewrites",
      "Headers",
    ]);
  });
});
