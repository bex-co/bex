import { describe, it, expect, vi, beforeEach } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { render, screen, waitFor, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceDetailLayout } from "../service-detail-layout";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// w3/m706: `/services/<dead-id>` keeps its URL and draws Render's "Service not
// found" (or "Service deleted" with the cached name) in place, with no toast,
// where w9/m55 redirected home with one. A refused read draws Access denied and
// a genuine backend failure "An error occurred". The branches differ only in
// the error bex-api reports — a dead id is answered with `server: null` PLUS a
// NOT_FOUND error — so a layout that tests `!error` alone would render a
// fully-chromed ghost service for a resource that does not exist (w6/m44).

const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));
const missingContext = vi.hoisted(() => ({
  name: null as string | null,
  projectId: null as string | null,
  type: null as string | null,
}));
vi.mock("@/common/hooks/use-missing-resource-context", () => ({
  useMissingServiceContext: () => missingContext,
}));
vi.mock("@/features/services/hooks/use-cached-service-type", () => ({
  useCachedServiceType: () => null,
  useCachedServiceSource: () => null,
  useCachedServiceName: () => null,
  useCachedRunsInstances: () => null,
  useCachedServiceShellAccess: () => null,
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

function renderDetail(
  routeState?: "forbidden" | "not-found" | "error" | "ready",
) {
  const rootRoute = createRootRoute();
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>home page</p>,
  });
  const project = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    component: () => <p>project page</p>,
  });
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => (
      <ServiceDetailLayout serviceId="srv-dead" routeState={routeState} />
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, project, detail]),
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
  missingContext.name = null;
  missingContext.projectId = null;
  missingContext.type = null;
  toastError.mockReset();
});

/** The drawn state, apart from the shell's own links and the service id the
 *  sidebar shows. */
function stateRegion(heading: string) {
  const region = screen
    .getByRole("heading", { name: heading })
    .closest<HTMLElement>("[data-resource-state]");
  if (!region) throw new Error(`no state region around ${heading}`);
  return within(region);
}

async function expectStaysPut(router: ReturnType<typeof renderDetail>) {
  await waitFor(() =>
    expect(router.state.location.pathname).toBe("/services/srv-dead"),
  );
  expect(screen.queryByText("home page")).not.toBeInTheDocument();
  expect(screen.queryByText("service shell")).not.toBeInTheDocument();
  expect(toastError).not.toHaveBeenCalled();
}

describe("service detail, dead id (w3/m706)", () => {
  it("draws Service not found in place when bex-api reports the id as not found", async () => {
    serverState.error = codedGraphQLError("NOT_FOUND", {}, "not found");
    const router = renderDetail();

    expect(
      await screen.findByRole("heading", { name: "Service not found" }),
    ).toBeInTheDocument();
    const state = stateRegion("Service not found");
    expect(state.getByText("srv-dead")).toBeInTheDocument();
    expect(
      state.getByText(
        /It may have been deleted, or the link may be incorrect\./,
      ),
    ).toBeInTheDocument();
    // No project known: Return home alone.
    const links = state.getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(["Return home"]);
    expect(links[0]).toHaveAttribute("href", "/");
    await expectStaysPut(router);
  });

  it("draws the same when the id resolves to nothing with no error at all", async () => {
    const router = renderDetail();

    expect(
      await screen.findByRole("heading", { name: "Service not found" }),
    ).toBeInTheDocument();
    await expectStaysPut(router);
  });

  // A resource the dashboard had read keeps its entity in Apollo's cache:
  // Render's deleted variant names it and offers its project (w3/m81: a
  // deleting service's by-id read is the same not-found).
  it("draws Service deleted with the cached name and Go to project first", async () => {
    serverState.error = codedGraphQLError("NOT_FOUND", {}, "not found");
    missingContext.name = "orders-api";
    missingContext.projectId = "prj-shop";
    const router = renderDetail();

    expect(
      await screen.findByRole("heading", { name: "Service deleted" }),
    ).toBeInTheDocument();
    const state = stateRegion("Service deleted");
    expect(state.getByText("orders-api")).toBeInTheDocument();
    expect(state.getByText(/was deleted\./)).toBeInTheDocument();
    const links = state.getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual([
      "Go to project",
      "Return home",
    ]);
    expect(links[0]).toHaveAttribute("href", "/project/prj-shop");
    await expectStaysPut(router);
  });

  it("names a cron job by its cached type", async () => {
    serverState.error = codedGraphQLError("NOT_FOUND", {}, "not found");
    missingContext.type = "cron_job";
    renderDetail();

    expect(
      await screen.findByRole("heading", { name: "Cron job not found" }),
    ).toBeInTheDocument();
  });

  it("draws Access denied for a refused read", async () => {
    serverState.error = codedGraphQLError("FORBIDDEN", {}, "forbidden");
    const router = renderDetail();

    expect(
      await screen.findByRole("heading", { name: "Access denied" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/You don’t have access to/)).toBeInTheDocument();
    await expectStaysPut(router);
  });

  // Apollo caches the refused read's null without its error; the loader that
  // saw the refusal decides.
  it("draws Access denied when only the route's loader saw the refusal", async () => {
    renderDetail("forbidden");

    expect(
      await screen.findByRole("heading", { name: "Access denied" }),
    ).toBeInTheDocument();
  });

  it("keeps a genuine failure on An error occurred, never not found or a redirect", async () => {
    serverState.error = new Error("Failed to fetch");
    const router = renderDetail();

    expect(
      await screen.findByRole("heading", { name: "An error occurred" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/could not be loaded\./)).toBeInTheDocument();
    expect(screen.queryByText("Service not found")).not.toBeInTheDocument();
    await expectStaysPut(router);
  });
});
