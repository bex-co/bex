import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceDetailLayout } from "../service-detail-layout";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// w6/m44: `/services/<dead-id>` must go back to redirecting home, and a genuine
// backend failure must still keep the user on the inline retry state. The two
// branches differ only in the error bex-api reports — a dead id is answered with
// `server: null` PLUS `errors: [{message: "app not found"}]` — so a layout that
// tests `!error` alone renders a fully-chromed ghost service for a resource that
// does not exist (the regression of w9/m55 this milestone closes).

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
// environment context over Apollo; stub those at the hook boundary so this
// render needs no ApolloProvider (the pattern every other detail test uses).
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [],
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
  useLatestDeploy: () => ({ deploy: null, loading: false, error: undefined }),
}));
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null }),
}));
// The header (status chip, Connect, Manual Deploy) reads Apollo directly; stub
// it at the module boundary — the repo's pattern for a render that needs no
// ApolloProvider — and keep a marker so the test can assert the ghost service
// shell is never mounted for a dead id.
vi.mock("@/features/services/components/service-detail-header", () => ({
  ServiceDetailHeader: () => <p>service shell</p>,
  ServiceDetailHeaderSkeleton: () => <p>service shell skeleton</p>,
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args) },
}));

function renderDetail() {
  const rootRoute = createRootRoute();
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>home page</p>,
  });
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => <ServiceDetailLayout serviceId="srv-dead" />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, detail]),
    history: createMemoryHistory({ initialEntries: ["/services/srv-dead"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  serverState.service = null;
  serverState.loading = false;
  serverState.error = undefined;
  toastError.mockReset();
});

describe("service detail, dead id (w6/m44 — regression of w9/m55)", () => {
  it("redirects home when bex-api reports the id as not found", async () => {
    serverState.error = new Error("app not found");
    const router = renderDetail();

    expect(await screen.findByText("home page")).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(screen.queryByText("service shell")).not.toBeInTheDocument();
    expect(toastError).toHaveBeenCalledWith(
      "That resource doesn't exist or was deleted.",
      { id: "resource-not-found" },
    );
  });

  it("redirects home when the id resolves to nothing with no error at all", async () => {
    const router = renderDetail();

    expect(await screen.findByText("home page")).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });

  it("keeps a genuine failure on the inline error state, never redirecting", async () => {
    serverState.error = new Error("Failed to fetch");
    const router = renderDetail();

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/services/srv-dead"),
    );
    expect(screen.queryByText("home page")).not.toBeInTheDocument();
    expect(toastError).not.toHaveBeenCalled();
  });
});
