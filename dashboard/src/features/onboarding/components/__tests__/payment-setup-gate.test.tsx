import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PaymentSetupGate } from "../payment-setup-gate";

const query = vi.hoisted(() => ({
  data: undefined as
    | {
        workspaceBillingReadiness: {
          paymentMethodOnboardingRequired: boolean | null;
        } | null;
      }
    | undefined,
  calls: [] as Array<{ skip?: boolean; variables?: unknown }>,
}));

vi.mock("@apollo/client/react", () => ({
  useQuery: (
    _doc: unknown,
    options: { skip?: boolean; variables?: unknown },
  ) => {
    query.calls.push(options);
    return { data: options.skip ? undefined : query.data, loading: false };
  },
}));

const workspace = vi.hoisted(() => ({ id: "tea-a" as string | null }));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspace.id }),
}));

const permissiveCapabilities = {
  role: "ADMIN",
  canView: true,
  canViewLogs: true,
  canOperate: true,
  canCreate: true,
  canViewSensitive: true,
  canManageKeys: true,
  canManage: true,
  canManageBilling: true,
  loading: false,
  loaded: true,
};

function renderGate(initialPath = "/") {
  // The gate reads the browser location (not the memory router's) for `next`,
  // the same way the login bounce does.
  window.history.replaceState({}, "", initialPath);
  const rootRoute = createRootRoute();
  const appRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <PaymentSetupGate>
        <p>App content</p>
      </PaymentSetupGate>
    ),
  });
  const wallRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/setup/payment",
    validateSearch: (search: Record<string, unknown>) => ({
      next: typeof search.next === "string" ? search.next : undefined,
    }),
    component: () => <p>Payment wall</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([appRoute, wallRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  query.data = undefined;
  query.calls.length = 0;
  workspace.id = "tea-a";
  vi.mocked(useCapabilities).mockReturnValue(permissiveCapabilities);
});

describe("PaymentSetupGate", () => {
  it("sends a refused workspace's billing manager to the wall with the current href as next", async () => {
    query.data = {
      workspaceBillingReadiness: { paymentMethodOnboardingRequired: true },
    };
    const { router } = renderGate("/services/new?type=web");

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/setup/payment"),
    );
    expect(router.state.location.search).toEqual({
      next: "/services/new?type=web",
    });
    expect(await screen.findByText("Payment wall")).toBeInTheDocument();
    expect(screen.queryByText("App content")).not.toBeInTheDocument();
  });

  it("renders the app when the server says a create would pass", async () => {
    query.data = {
      workspaceBillingReadiness: { paymentMethodOnboardingRequired: false },
    };
    const { router } = renderGate();
    expect(await screen.findByText("App content")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("fails open while readiness is unknown", async () => {
    query.data = undefined;
    renderGate();
    expect(await screen.findByText("App content")).toBeInTheDocument();
  });

  it("never gates a member who cannot bind a card, and does not even ask", async () => {
    vi.mocked(useCapabilities).mockReturnValue({
      ...permissiveCapabilities,
      canManageBilling: false,
    });
    query.data = {
      workspaceBillingReadiness: { paymentMethodOnboardingRequired: true },
    };
    const { router } = renderGate();
    expect(await screen.findByText("App content")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    expect(query.calls.every((call) => call.skip === true)).toBe(true);
  });

  it("waits for a workspace before asking", async () => {
    workspace.id = null;
    query.data = {
      workspaceBillingReadiness: { paymentMethodOnboardingRequired: true },
    };
    renderGate();
    expect(await screen.findByText("App content")).toBeInTheDocument();
    expect(query.calls.every((call) => call.skip === true)).toBe(true);
  });
});
