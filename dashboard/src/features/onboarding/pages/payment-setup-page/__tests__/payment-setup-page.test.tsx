import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { BillingReadiness } from "@/features/usage/hooks/use-billing-onboarding";
import PaymentSetupPage from "..";

const billing = vi.hoisted(() => ({
  readiness: null as BillingReadiness | null,
  loading: false,
  error: undefined as Error | undefined,
  checkoutBusy: false,
  openCheckout: vi.fn(),
  openPortal: vi.fn(),
  refetch: vi.fn(),
  options: [] as unknown[],
}));

vi.mock("@/features/usage/hooks/use-billing-onboarding", () => ({
  useBillingOnboarding: (options: unknown) => {
    billing.options.push(options);
    return {
      readiness: billing.readiness,
      loading: billing.loading,
      error: billing.error,
      checkoutBusy: billing.checkoutBusy,
      portalBusy: false,
      openCheckout: billing.openCheckout,
      openPortal: billing.openPortal,
      refetch: billing.refetch,
    };
  },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    currentWorkspaceId: "tea-a",
    currentWorkspace: {
      id: "tea-a",
      name: "acme",
      plan: "hobby",
      role: "admin",
    },
  }),
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

function readiness(
  overrides: Partial<BillingReadiness> = {},
): BillingReadiness {
  return {
    workspaceId: "tea-a",
    mode: "test",
    customerReady: false,
    subscriptionReady: false,
    paymentMethodReady: false,
    paymentMethodBrand: "",
    paymentMethodLast4: "",
    paymentMethodRequired: true,
    paymentMethodOnboardingRequired: true,
    lifecycle: {
      status: "healthy",
      reason: "",
      graceDeadline: "",
      enforcementOwned: false,
      recoveryPending: false,
      allowedActions: [],
      updatedAt: "",
    },
    tax: {
      configured: false,
      enabled: false,
      reason: "",
      productTaxCode: "",
      taxBehavior: "",
      registrationCount: 0,
    },
    ...overrides,
  };
}

function renderWall(initialEntry = "/setup/payment") {
  const rootRoute = createRootRoute();
  const wallRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/setup/payment",
    validateSearch: (search: Record<string, unknown>) => ({
      next: typeof search.next === "string" ? search.next : undefined,
      billing:
        search.billing === "success" || search.billing === "cancelled"
          ? (search.billing as "success" | "cancelled")
          : undefined,
    }),
    component: PaymentSetupPage,
  });
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>Overview</p>,
  });
  const newServiceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/new",
    component: () => <p>New service</p>,
  });
  const logoutRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/auth/logout",
    component: () => <p>Logout</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      wallRoute,
      homeRoute,
      newServiceRoute,
      logoutRoute,
    ]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  billing.readiness = null;
  billing.loading = false;
  billing.error = undefined;
  billing.checkoutBusy = false;
  billing.openCheckout.mockReset();
  billing.refetch.mockReset();
  billing.options.length = 0;
  vi.mocked(useCapabilities).mockReturnValue(permissiveCapabilities);
});

describe("PaymentSetupPage", () => {
  it("walls a refused workspace: names it, opens Checkout, and offers the self-host and sign-out exits", async () => {
    billing.readiness = readiness();
    const user = userEvent.setup();
    const { router } = renderWall("/setup/payment?next=%2Fservices%2Fnew");

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Add a payment method",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Payment method required")).toBeInTheDocument();
    expect(screen.getByText("Workspace: acme")).toBeInTheDocument();
    expect(screen.getByText("Stripe Test Mode")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Add test payment method" }),
    );
    expect(billing.openCheckout).toHaveBeenCalledOnce();

    expect(
      screen.getByRole("link", { name: "Self-host bex instead" }),
    ).toHaveAttribute("href", "https://github.com/bex-co/bex");
    expect(screen.getByRole("link", { name: "Sign out" })).toHaveAttribute(
      "href",
      "/auth/logout",
    );
    // Still on the wall — nothing forwarded a refused workspace.
    expect(router.state.location.pathname).toBe("/setup/payment");
  });

  it("returns Stripe to the wall itself, deep link intact, and polls at the dialog's cadence", async () => {
    billing.readiness = readiness();
    renderWall("/setup/payment?next=%2Fservices%2Fnew%3Ftype%3Dweb");
    await screen.findByText("Payment method required");
    expect(billing.options[0]).toMatchObject({
      active: true,
      pollInterval: 2000,
      returnPath: "/setup/payment?next=%2Fservices%2Fnew%3Ftype%3Dweb",
    });
  });

  it("labels the live-mode button without the test qualifier", async () => {
    billing.readiness = readiness({ mode: "live" });
    renderWall();
    expect(
      await screen.findByRole("button", { name: "Add payment method" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Stripe Test Mode")).not.toBeInTheDocument();
  });

  it("explains a cancelled Checkout without leaving the wall", async () => {
    billing.readiness = readiness();
    renderWall("/setup/payment?billing=cancelled");
    expect(
      await screen.findByText(
        "Checkout was cancelled. No payment method was added.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add test payment method" }),
    ).toBeInTheDocument();
  });

  it("shows the confirming status after a successful Checkout until the webhook commits", async () => {
    billing.readiness = readiness();
    renderWall("/setup/payment?billing=success");
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Payment method received — confirming with Stripe…",
    );
  });

  it("continues to the deep link the moment the server says the gate is open", async () => {
    billing.readiness = readiness({ paymentMethodOnboardingRequired: false });
    const { router } = renderWall("/setup/payment?next=%2Fservices%2Fnew");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/services/new"),
    );
    expect(await screen.findByText("New service")).toBeInTheDocument();
    expect(billing.openCheckout).not.toHaveBeenCalled();
  });

  it("falls back to the overview when there is no deep link", async () => {
    billing.readiness = readiness({ paymentMethodOnboardingRequired: false });
    const { router } = renderWall();
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });

  it("forwards a member who cannot bind a card here instead of walling them", async () => {
    vi.mocked(useCapabilities).mockReturnValue({
      ...permissiveCapabilities,
      canManageBilling: false,
    });
    billing.readiness = null;
    const { router } = renderWall("/setup/payment?next=%2Fservices%2Fnew");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/services/new"),
    );
    // And never asked for a readiness they are not allowed to read.
    expect(billing.options[0]).toMatchObject({ active: false });
  });

  it("holds the wall's actions while readiness is unknown", async () => {
    billing.loading = true;
    renderWall();
    await screen.findByText("Payment method required");
    expect(
      screen.queryByRole("button", { name: /payment method/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Continuing…");
  });

  it("is not a dead end when readiness cannot be read", async () => {
    billing.error = new Error("billing unavailable");
    const user = userEvent.setup();
    const { router } = renderWall("/setup/payment?next=%2Fservices%2Fnew");

    expect(
      await screen.findByText(/Billing onboarding is unavailable/),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(billing.refetch).toHaveBeenCalledOnce();

    await user.click(
      screen.getByRole("link", { name: "Continue to the dashboard" }),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/services/new"),
    );
  });
});
