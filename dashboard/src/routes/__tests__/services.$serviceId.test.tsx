import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceDetailLayout } from "../services.$serviceId";
import { ServiceOverviewPage } from "../services.$serviceId.index";
import { ServiceLogsPage } from "../services.$serviceId.logs";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// The layout + overview both read useServer; the layout also drives lifecycle.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));
const run = vi.fn();
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run }),
}));

// The Logs tab's data layer hits Apollo + SSE; this routing test only cares
// that the viewer mounts under the shared chrome, so stub both to empty.
vi.mock("@/features/logs/hooks/use-log-history", () => ({
  useLogHistory: () => ({ lines: [], loading: false, error: undefined }),
}));
vi.mock("@/features/logs/hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "r1",
    ...overrides,
  };
}

// Rebuilds the real layout + its Overview/Logs children as a router rooted at
// `initialPath`, mirroring the generated nesting (both tabs live under the
// `/services/$serviceId` layout).
function renderAt(initialPath: string) {
  const rootRoute = createRootRoute();
  const layoutRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => <ServiceDetailLayout serviceId="app" />,
  });
  const indexRoute = createRoute({
    getParentRoute: () => layoutRoute,
    path: "/",
    component: () => <ServiceOverviewPage serviceId="app" />,
  });
  const logsRoute = createRoute({
    getParentRoute: () => layoutRoute,
    path: "logs",
    component: () => <ServiceLogsPage serviceId="app" />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      layoutRoute.addChildren([indexRoute, logsRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = svc();
  serverState.loading = false;
  serverState.error = undefined;
  run.mockReset();
});

describe("service-detail layout routing", () => {
  it("renders the Overview tab at /services/$serviceId with the nav targets", async () => {
    renderAt("/services/app");

    // overview child resolved under the layout — the "Phase" field label is
    // unique to the overview panel (the always-present header shows the URL, so
    // the URL can't discriminate which tab rendered).
    expect(await screen.findByText("Phase")).toBeInTheDocument();

    // nav mirrors Render's service sidebar subset with the right deep-links
    expect(screen.getByRole("link", { name: "Overview" })).toHaveAttribute(
      "href",
      "/services/app",
    );
    expect(screen.getByRole("link", { name: "Logs" })).toHaveAttribute(
      "href",
      "/services/app/logs",
    );

    // the logs tab is not what's rendered here (its filter bar is absent)
    expect(
      screen.queryByPlaceholderText("Search logs"),
    ).not.toBeInTheDocument();
  });

  it("shows a not-found state (no service chrome) for an unknown service id", async () => {
    serverState.service = null;
    serverState.loading = false;
    renderAt("/services/app");

    // the shell renders not-found above every tab — no header/nav/outlet
    expect(await screen.findByText("Service not found")).toBeInTheDocument();
    expect(screen.getByText(/app/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Back to services" }),
    ).toHaveAttribute("href", "/");
    // service nav tabs are absent (the Overview child never mounts)
    expect(
      screen.queryByRole("link", { name: "Overview" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Phase")).not.toBeInTheDocument();
  });

  it("renders the Logs viewer tab at /services/$serviceId/logs under the same chrome", async () => {
    renderAt("/services/app/logs");

    // the viewer's filter bar (search box) marks the logs tab as rendered
    expect(
      await screen.findByPlaceholderText("Search logs"),
    ).toBeInTheDocument();
    // still under the shared chrome (the service header shows the name)
    expect(screen.getByRole("heading", { name: "app" })).toBeInTheDocument();
    // and not showing the overview panel (its "Phase" field is absent)
    expect(screen.queryByText("Phase")).not.toBeInTheDocument();
  });
});
