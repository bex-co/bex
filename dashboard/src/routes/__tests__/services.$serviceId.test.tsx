import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceDetailLayout } from "@/features/services/components/service-detail-layout";
import { ServiceLogsPage } from "../services.$serviceId.logs";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";

// The layout reads useServer for the header and drives lifecycle.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));
// The shared topbar's service switcher reads sibling services and project /
// environment context. This routing test has no ApolloProvider, so keep those
// navigation hooks aligned with the mocked service state above.
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: serverState.service ? [serverState.service] : [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));
vi.mock("@/features/deploys/hooks/use-latest-deploy", () => ({
  useLatestDeploy: () => ({
    deploy: null,
    loading: false,
    error: undefined,
  }),
}));
const run = vi.fn();
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run }),
}));

// (The service sidebar — w1/m45's resource-scoped nav that replaced the tab
// strip — reads the same useServer hook as the shell, mocked above, so the
// two agree on whether the service exists by construction.)

// The header's instance-type chip + Manual Deploy button both go through Apollo;
// stub them at the hook boundary so this routing test needs no ApolloProvider.
const STARTER: InstanceTypeView = {
  id: "starter",
  name: "Starter",
  cpu: "500m",
  memory: "512Mi",
  monthlyUsd: "4.90",
};
vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    instanceTypes: [STARTER],
    loading: false,
    error: undefined,
    byID: (id: string | null | undefined) =>
      id === "starter" ? STARTER : undefined,
  }),
}));
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ deploying: false, trigger: vi.fn() }),
}));
// The header's row-actions menu renders a "Move to project" submenu over Apollo.
vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

// The Logs tab's data layer hits Apollo + SSE; this routing test only cares
// that the viewer mounts under the shared chrome, so stub both to empty.
vi.mock("@/features/logs/hooks/use-log-history", () => ({
  useLogHistory: () => ({
    lines: [],
    loading: false,
    error: undefined,
    storeUnavailable: false,
  }),
}));
vi.mock("@/features/logs/hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
}));
// The filter bar's dropdowns discover values over Apollo — stub so this routing
// test needs no ApolloProvider.
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelValues: () => [],
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "r1",
    plan: "starter",
    ...overrides,
  } as ServiceView;
}

// Rebuilds the real layout + the Logs child as a router rooted at `initialPath`,
// mirroring the generated nesting (every tab lives under the
// `/services/$serviceId` layout). The Events landing tab isn't mounted here —
// it's a file route that reads its own params over Apollo; the nav assertions
// below cover that it's the tab the shell points at.
function renderAt(initialPath: string) {
  const rootRoute = createRootRoute();
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>services home</div>,
  });
  const layoutRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => <ServiceDetailLayout serviceId="app" />,
  });
  const logsRoute = createRoute({
    getParentRoute: () => layoutRoute,
    path: "logs",
    component: () => <ServiceLogsPage serviceId="app" />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      homeRoute,
      layoutRoute.addChildren([logsRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  serverState.service = svc();
  serverState.loading = false;
  serverState.error = undefined;
  serverState.refetch = vi.fn(async () => []);
  run.mockReset();
});

describe("service-detail layout routing", () => {
  it("renders the Logs viewer tab at /services/$serviceId/logs under the shared chrome", async () => {
    renderAt("/services/app/logs");

    // the viewer's filter bar (search box) marks the logs tab as rendered
    const search = await screen.findByPlaceholderText("Search logs");
    expect(search).toBeInTheDocument();
    // …under the shared chrome: the header shows the name, type, and live URL
    const heading = screen.getByRole("heading", { name: "app" });
    expect(heading).toBeInTheDocument();
    expect(screen.getByText("Web Service")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "https://app.onbex.co" }),
    ).toHaveAttribute("href", "https://app.onbex.co");

    // The service header and tab content share one scroll container, so the
    // header scrolls away instead of staying frozen above it. (The service nav
    // moved into the resource-scoped sidebar, w1/m45 — outside this container.)
    const scrollContainer = heading.closest(".overflow-auto");
    expect(scrollContainer).toContainElement(heading);
    expect(scrollContainer).toContainElement(search);
    const eventsLink = await screen.findByRole("link", { name: "Events" });
    expect(scrollContainer).not.toContainElement(eventsLink);
  });

  it("leaves document-title ownership with the parent route head", async () => {
    serverState.service = svc({ id: "app", name: "friendly" });
    document.title = "route-owned title";
    renderAt("/services/app/logs");

    expect(
      await screen.findByRole("heading", { name: "friendly" }),
    ).toBeInTheDocument();
    expect(document.title).toBe("route-owned title");
  });

  it("keeps Events directly reachable in the Render tab set, with no Overview tab", async () => {
    renderAt("/services/app/logs");

    // Events remains a direct sibling even though the service root now lands
    // on Deploys (pinned by service-root-redirect.test.ts).
    const tabs = await screen.findAllByRole("link");
    const names = tabs.map((el) => el.textContent);
    expect(names).not.toContain("Overview");
    expect(await screen.findByRole("link", { name: "Events" })).toHaveAttribute(
      "href",
      "/services/app/events",
    );
    expect(screen.getByRole("link", { name: "Logs" })).toHaveAttribute(
      "href",
      "/services/app/logs",
    );
  });

  it("redirects an unknown service id home from any tab (w9/m55)", async () => {
    serverState.service = null;
    serverState.loading = false;
    const router = renderAt("/services/app/logs");

    // the shell redirects above every tab — no service chrome, no outlet, and
    // never a child tab borrowing another service's data en route
    expect(await screen.findByText("services home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    expect(
      screen.queryByRole("link", { name: "Events" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText("Search logs"),
    ).not.toBeInTheDocument();
  });

  it("shows a retryable error instead of not-found when the service query fails", async () => {
    serverState.service = null;
    serverState.loading = false;
    serverState.error = new Error("Cannot query field ipAllowList");
    renderAt("/services/app/logs");

    expect(
      await screen.findByText("Couldn't load service"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The service may still exist.", { exact: false }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Service not found")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Events" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(serverState.refetch).toHaveBeenCalledOnce();
  });
});
