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
  refetch: () => Promise<unknown>;
} = { keyValue: null, loading: false, refetch: vi.fn() };
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
// panel's data hook above.
vi.mock("@/features/metrics/hooks/use-datastore-metrics", () => ({
  useDatastoreMetrics: () => ({
    series: [],
    loading: false,
    unavailable: false,
    error: undefined,
  }),
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

function renderPage() {
  const rootRoute = createRootRoute();
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/keyvalue/$keyValueId",
    component: KeyValueDetailPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([detailRoute]),
    history: createMemoryHistory({
      initialEntries: ["/keyvalue/sessions-cache"],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  keyValueState.keyValue = null;
  keyValueState.loading = false;
  lifecycleRun.mockReset();
  reveal.mockReset();
});

describe("KeyValueDetailPage", () => {
  it("renders an available store's metadata and a Suspend action", async () => {
    keyValueState.keyValue = kv({ status: "available", suspended: false });
    renderPage();

    expect(await screen.findByText("sessions-cache")).toBeInTheDocument();
    expect(screen.getByText("Available")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    ).toBeInTheDocument();
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

  it("shows the not-found state when the store doesn't exist", async () => {
    keyValueState.keyValue = null;
    keyValueState.loading = false;
    renderPage();

    expect(
      await screen.findByText("Key Value store not found"),
    ).toBeInTheDocument();
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

    await screen.findByText("sessions-cache");
    expect(screen.queryByText("Region")).not.toBeInTheDocument();
  });
});
