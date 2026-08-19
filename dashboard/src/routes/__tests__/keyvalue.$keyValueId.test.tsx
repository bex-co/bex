import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { KeyValueDetailPage } from "../keyvalue.$keyValueId";
import type { KeyValueView } from "@/features/keyvalue/types";

const keyValueState: {
  keyValue: KeyValueView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = { keyValue: null, loading: false, error: undefined, refetch: vi.fn() };
vi.mock("@/features/keyvalue/hooks/use-key-value", () => ({
  useKeyValue: () => keyValueState,
}));

vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove: vi.fn(), deleting: null }),
}));

const lifecycleRun = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-key-value-lifecycle", () => ({
  useKeyValueLifecycle: () => ({ pending: null, run: lifecycleRun }),
}));

const reveal = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({
    info: null,
    loading: false,
    error: undefined,
    reveal,
    hide: vi.fn(),
  }),
}));

vi.mock("@/features/keyvalue/hooks/use-key-value-networking", () => ({
  useKeyValueNetworking: () => ({
    allowList: [],
    loading: false,
    savingAllowList: false,
    saveAllowList: vi.fn(),
  }),
}));

// DatastoreMetricsPanel (w3/m10) reads via Apollo's useQuery directly, which
// needs an ApolloProvider this route's test tree doesn't set up (no other
// panel here touches Apollo) — mocked at the hook boundary like every other
// panel's data hook above. Calls are recorded so a test can assert the panel
// receives the KeyValue's typed id (w5/m71: bex-api resolves
// datastoreMetrics.query.resource as the CR name, and CR names are the ids —
// passing the display name 404s into silent empty charts).
const datastoreMetricsCalls: { kind: string; resource: string }[] = [];
vi.mock("@/features/metrics/hooks/use-datastore-metrics", () => ({
  useDatastoreMetrics: (kind: string, resource: string) => {
    datastoreMetricsCalls.push({ kind, resource });
    return {
      series: [],
      loading: false,
      unavailable: false,
      error: undefined,
    };
  },
}));

// KeyValuePlanSection (m16) calls useKeyValueInstanceTypes (Apollo) and
// useUpdateKeyValuePlan (Apollo) — same pattern as DatastoreMetricsPanel above.
vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => ({
    instanceTypes: [],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-update-key-value-plan", () => ({
  useUpdateKeyValuePlan: () => ({ updatePlan: vi.fn(), busy: false }),
}));

// KeyValueMaxmemoryPolicySection (w7/007) reads + writes via Apollo — mocked at
// the hook boundary like the plan section above (no ApolloProvider here).
vi.mock("@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy", () => ({
  useSetKeyValueMaxmemoryPolicy: () => ({
    policy: "allkeys-lru",
    loading: false,
    saving: false,
    save: vi.fn(),
  }),
}));

// KeyValueNameSection's rename hook wraps Apollo's useMutation — mocked at the
// hook boundary like every other data hook above (no ApolloProvider here).
vi.mock("@/features/keyvalue/hooks/use-rename-key-value", () => ({
  useRenameKeyValue: () => ({ rename: vi.fn(), busy: false }),
}));

function kv(overrides: Partial<KeyValueView> = {}): KeyValueView {
  return {
    id: "red-sessions",
    name: "sessions-cache",
    status: "available",
    plan: "starter",
    version: "8",
    createdAt: null,
    externalHost: null,
    public: false,
    suspended: false,
    region: null,
    ...overrides,
  };
}

function renderPage(initialPath = "/keyvalue/sessions-cache") {
  const rootRoute = createRootRoute();
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>services home</div>,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/keyvalue/$keyValueId",
    component: KeyValueDetailPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([homeRoute, detailRoute]),
    history: createMemoryHistory({
      initialEntries: [initialPath],
    }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  keyValueState.keyValue = null;
  keyValueState.loading = false;
  keyValueState.error = undefined;
  lifecycleRun.mockReset();
  reveal.mockReset();
  datastoreMetricsCalls.length = 0;
});

describe("KeyValueDetailPage", () => {
  it("renders an available store's metadata and Render's bottom danger actions", async () => {
    keyValueState.keyValue = kv({ status: "available", suspended: false });
    renderPage();

    expect(
      await screen.findByRole("heading", { name: "sessions-cache" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Available")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Delete Key Value Instance" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Open actions menu" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reveal connection info" }),
    ).toBeInTheDocument();
  });

  it("renders a Resume action when the store is suspended", async () => {
    keyValueState.keyValue = kv({ suspended: true });
    renderPage();

    expect(
      await screen.findByRole("button", { name: "Resume Key Value Instance" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Suspend Key Value Instance" }),
    ).not.toBeInTheDocument();
  });

  it("renders a creating store without connection credentials revealed", async () => {
    keyValueState.keyValue = kv({ status: "creating", plan: "free" });
    renderPage();

    expect(await screen.findByText("Creating")).toBeInTheDocument();
    // The Reveal control is present, but nothing sensitive is ever pre-rendered.
    expect(
      screen.getByRole("button", { name: "Reveal connection info" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/redis:\/\//)).not.toBeInTheDocument();
  });

  it("redirects a dead store id home (w9/m55)", async () => {
    keyValueState.keyValue = null;
    keyValueState.loading = false;
    const router = renderPage();

    expect(await screen.findByText("services home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("stays put on the inline error state when the query fails (w9/m55)", async () => {
    keyValueState.keyValue = null;
    keyValueState.loading = false;
    keyValueState.error = new Error("bex-api unreachable");
    const router = renderPage();

    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/keyvalue/sessions-cache");
  });

  it("renders the Region row when region is configured (w9/m42/t004)", async () => {
    keyValueState.keyValue = kv({ region: "fsn1" });
    renderPage();

    expect(await screen.findByText("Region")).toBeInTheDocument();
    expect(screen.getByText("fsn1")).toBeInTheDocument();
  });

  it("omits the Region row when region is null (BEX_REGION unset)", async () => {
    keyValueState.keyValue = kv({ region: null });
    renderPage();

    await screen.findByRole("heading", { name: "sessions-cache" });
    expect(screen.queryByText("Region")).not.toBeInTheDocument();
  });

  it("renders the metrics panel on its own tab, feeding it the typed id (w5/m71)", async () => {
    // bex-api resolves datastoreMetrics.query.resource as the KeyValue CR
    // name, and CR names are the typed ids — a display name 404s into charts
    // that silently render "No data in range".
    keyValueState.keyValue = kv({ id: "red-sessions", name: "sessions-cache" });
    renderPage("/keyvalue/sessions-cache?tab=metrics");

    await screen.findByRole("heading", { name: "sessions-cache" });
    expect(datastoreMetricsCalls.length).toBeGreaterThan(0);
    for (const call of datastoreMetricsCalls) {
      expect(call).toEqual({ kind: "keyvalue", resource: "red-sessions" });
    }
  });

  it("does not render the metrics panel on the Overview tab", async () => {
    keyValueState.keyValue = kv({ id: "red-sessions", name: "sessions-cache" });
    renderPage();

    await screen.findByRole("heading", { name: "sessions-cache" });
    expect(datastoreMetricsCalls).toHaveLength(0);
  });
});
