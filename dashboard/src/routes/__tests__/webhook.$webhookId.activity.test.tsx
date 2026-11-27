import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouteComponent,
} from "@tanstack/react-router";

// w3/m707: Render's webhook Activity is `/webhook/<id>/activity`, its
// sidebar's Activity entry links there, and the bare URL opens it too.

vi.mock("@/features/webhooks/hooks/use-webhook", () => ({
  useWebhook: () => ({
    endpoint: {
      id: "whk-1",
      name: "Deploy hook",
      url: "https://example.com/hook",
      enabled: true,
      ownerId: "tea-mine",
      createdBy: "",
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/workspaces/hooks/use-adopt-resource-workspace", () => ({
  useAdoptResourceWorkspace: () => false,
}));
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: () => null,
}));
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
vi.mock("@/features/webhooks/components/webhook-detail-header", () => ({
  WebhookDetailHeader: () => <header>webhook header</header>,
}));
vi.mock("@/features/webhooks/components/webhook-deliveries-card", () => ({
  WebhookDeliveriesCard: ({ endpointId }: { endpointId: string }) => (
    <p>deliveries of {endpointId}</p>
  ),
}));

import { Route as WebhookRoute } from "../webhook.$webhookId";
import { Route as IndexRoute } from "../webhook.$webhookId.index";
import { Route as ActivityRoute } from "../webhook.$webhookId.activity";

async function open(url: string) {
  const root = createRootRoute();
  const shell = createRoute({
    getParentRoute: () => root,
    path: "/webhook/$webhookId",
    component: WebhookRoute.options.component as RouteComponent,
    loader: () => ({ state: "ready" }),
  });
  const router = createRouter({
    routeTree: root.addChildren([
      shell.addChildren([
        createRoute({
          getParentRoute: () => shell,
          path: "/",
          component: IndexRoute.options.component as RouteComponent,
        }),
        createRoute({
          getParentRoute: () => shell,
          path: "activity",
          component: ActivityRoute.options.component as RouteComponent,
        }),
        createRoute({
          getParentRoute: () => shell,
          path: "settings",
          component: () => <p>settings tab</p>,
        }),
      ]),
    ]),
    history: createMemoryHistory({ initialEntries: [url] }),
  });
  render(<RouterProvider router={router} />);
  await router.load();
  return router;
}

describe("a webhook's Activity at Render's path (w3/m707)", () => {
  it.each(["/webhook/whk-1/activity", "/webhook/whk-1"])(
    "opens Activity at %s, its tab selected and linking Render's path",
    async (url) => {
      const router = await open(url);

      expect(await screen.findByText("deliveries of whk-1")).toBeVisible();
      const activity = screen.getByRole("link", { name: "Activity" });
      expect(activity).toHaveAttribute("href", "/webhook/whk-1/activity");
      expect(activity).toHaveAttribute("aria-current", "page");
      expect(activity).toHaveClass("border-primary");
      const settings = screen.getByRole("link", { name: "Settings" });
      expect(settings).not.toHaveAttribute("aria-current");
      expect(settings).not.toHaveClass("border-primary");
      expect(router.state.location.pathname).toBe(url);
    },
  );

  it("selects Settings, not Activity, on the Settings tab", async () => {
    await open("/webhook/whk-1/settings");

    expect(await screen.findByText("settings tab")).toBeVisible();
    const activity = screen.getByRole("link", { name: "Activity" });
    expect(activity).not.toHaveAttribute("aria-current");
    expect(activity).not.toHaveClass("border-primary");
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });
});
