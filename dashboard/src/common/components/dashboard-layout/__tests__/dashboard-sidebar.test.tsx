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

function renderAt(pathname: string, defaultOpen = true) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: pathname,
    component: () => (
      <SidebarProvider defaultOpen={defaultOpen}>
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
      ["Billing", "/billing"],
      ["Settings", "/workspace/settings"],
    ] as const) {
      expect(screen.getByRole("link", { name })).toHaveAttribute("href", href);
    }
  });

  it("groups Webhooks/Notifications under Integrations and Billing/Settings under Workspace", async () => {
    renderAt("/");

    const integrations = (await screen.findByText("Integrations")).closest(
      '[data-slot="sidebar-group"]',
    ) as HTMLElement;
    expect(within(integrations).getByText("Webhooks")).toBeInTheDocument();
    expect(within(integrations).getByText("Notifications")).toBeInTheDocument();

    const workspace = screen
      .getByText("Workspace")
      .closest('[data-slot="sidebar-group"]') as HTMLElement;
    expect(within(workspace).getByText("Billing")).toBeInTheDocument();
    expect(within(workspace).getByText("Settings")).toBeInTheDocument();
  });

  it("keeps collapsed group labels from intercepting the preceding icon links", async () => {
    renderAt("/", false);

    const labels = await screen.findAllByText(/^(Integrations|Workspace)$/);
    expect(labels).toHaveLength(2);
    for (const label of labels) {
      expect(label).toHaveClass(
        "group-data-[collapsible=icon]:pointer-events-none",
      );
    }
  });
});
