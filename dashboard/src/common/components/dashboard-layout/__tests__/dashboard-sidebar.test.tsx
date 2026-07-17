import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { DashboardSidebar } from "../dashboard-sidebar";

function renderAt(pathname: string) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: pathname,
    component: () => (
      <SidebarProvider>
        <DashboardSidebar />
      </SidebarProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

// Render's global sidebar grouping (w1/m45, live capture 2026-07-16 —
// docs/render-artifacts/dashboard-routes.md § Sidebar navigation): an
// unlabeled top trio, then Integrations and Workspace groups. Render's
// Networking group is omitted (both entries are non-goals).
describe("DashboardSidebar (w1/m45 — Render's nav grouping)", () => {
  it("renders the three groups with their entries", async () => {
    renderAt("/");

    expect(await screen.findByText("Integrations")).toBeInTheDocument();
    expect(screen.getByText("Workspace")).toBeInTheDocument();
    expect(screen.queryByText("Networking")).not.toBeInTheDocument();

    for (const [name, href] of [
      ["Projects", "/"],
      ["Blueprints", "/blueprints"],
      ["Environment Groups", "/env-groups"],
      ["Webhooks", "/webhooks"],
      ["Notifications", "/notifications"],
      ["Usage", "/usage"],
      ["Settings", "/workspace/settings"],
    ] as const) {
      expect(screen.getByRole("link", { name })).toHaveAttribute("href", href);
    }
  });

  it("groups Webhooks/Notifications under Integrations and Usage/Settings under Workspace", async () => {
    renderAt("/");

    const integrations = (await screen.findByText("Integrations")).closest(
      '[data-slot="sidebar-group"]',
    ) as HTMLElement;
    expect(within(integrations).getByText("Webhooks")).toBeInTheDocument();
    expect(within(integrations).getByText("Notifications")).toBeInTheDocument();

    const workspace = screen
      .getByText("Workspace")
      .closest('[data-slot="sidebar-group"]') as HTMLElement;
    expect(within(workspace).getByText("Usage")).toBeInTheDocument();
    expect(within(workspace).getByText("Settings")).toBeInTheDocument();
  });
});
