import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NonStaticRoute } from "../non-static-route";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

// Mounts the guard on a runtime tab with the static Events landing as the
// redirect target (w5/m57), so the Navigate has a real destination to resolve.
function renderGuarded() {
  const rootRoute = createRootRoute();
  const shellRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/shell",
    component: () => (
      <NonStaticRoute serviceId="srv-1">
        <p>runtime tab content</p>
      </NonStaticRoute>
    ),
  });
  const staticEventsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/static/$serviceId/events",
    component: () => <p>static events landing</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([shellRoute, staticEventsRoute]),
    history: createMemoryHistory({
      initialEntries: ["/services/srv-1/shell"],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = null;
  serverState.loading = false;
});

describe("NonStaticRoute (w5/m48 — static sites have no runtime tabs)", () => {
  it("renders the tab for a web service", async () => {
    serverState.service = { id: "srv-1", type: "web_service" } as ServiceView;
    renderGuarded();

    expect(await screen.findByText("runtime tab content")).toBeInTheDocument();
  });

  it("redirects a static site's direct URL to its /static Events landing (w5/m57)", async () => {
    serverState.service = { id: "srv-1", type: "static_site" } as ServiceView;
    renderGuarded();

    expect(
      await screen.findByText("static events landing"),
    ).toBeInTheDocument();
    expect(screen.queryByText("runtime tab content")).not.toBeInTheDocument();
  });

  it("keeps rendering while the type is still loading (no flash for web services)", async () => {
    serverState.service = null;
    serverState.loading = true;
    renderGuarded();

    expect(await screen.findByText("runtime tab content")).toBeInTheDocument();
  });
});
