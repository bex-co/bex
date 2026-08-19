import type { ReactNode } from "react";
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
import { NewWorkspacePage } from "../new.workspace";
import type { BillingReadiness } from "@/features/usage/hooks/use-billing-onboarding";

const create = vi.fn();
vi.mock("@/features/workspaces/hooks/use-create-workspace", () => ({
  useCreateWorkspace: () => ({ create, busy: false, error: null }),
}));

const setCurrentWorkspaceId = vi.fn();
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    setCurrentWorkspaceId,
    currentWorkspaceId: "tea-current",
  }),
}));

const billingState: {
  readiness: BillingReadiness | null;
  loading: boolean;
  error: Error | undefined;
} = {
  readiness: null,
  loading: false,
  error: undefined,
};

vi.mock("@/features/usage/hooks/use-billing-onboarding", () => ({
  useBillingOnboarding: () => ({
    ...billingState,
    checkoutBusy: false,
    portalBusy: false,
    openCheckout: vi.fn(),
    openPortal: vi.fn(),
    refetch: vi.fn(),
  }),
}));

vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canManageBilling: true }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/new/workspace",
    component: NewWorkspacePage,
  });
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>Workspace home</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute, homeRoute]),
    history: createMemoryHistory({ initialEntries: ["/new/workspace"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

function gatedReadiness(ready: boolean): BillingReadiness {
  return {
    workspaceId: "tea-current",
    mode: "test",
    customerReady: true,
    subscriptionReady: true,
    paymentMethodReady: ready,
    paymentMethodBrand: "",
    paymentMethodLast4: "",
    paymentMethodRequired: true,
    lifecycle: {
      status: "healthy",
      reason: "",
      graceDeadline: "",
      enforcementOwned: false,
      recoveryPending: false,
      allowedActions: ["update_payment_method"],
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
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  billingState.readiness = null;
  billingState.loading = false;
  billingState.error = undefined;
  create.mockResolvedValue({
    id: "tea-returned-id",
    name: "acme",
    plan: "hobby",
    role: "admin",
    createdAt: null,
  });
});

describe("NewWorkspacePage", () => {
  it("uses a page heading and a workspace slug field", async () => {
    renderPage();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Create a workspace",
      }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Workspace slug")).toBeInTheDocument();
    expect(
      screen.getByText(/Used in URLs and resource names/),
    ).toBeInTheDocument();
  });

  it("does not submit an invalid slug", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "Acme HQ");
    expect(screen.getByRole("button", { name: "Create Workspace" })).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("selects the returned workspace before landing on its home context", async () => {
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-returned-id");
    expect(router.history.location.pathname).toBe("/");
  });

  it("does not switch or navigate when create fails", async () => {
    create.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/new/workspace");
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Workspace slug")).toHaveValue("acme");
  });

  it("does not show a payment panel on Hobby and allows create without a card", async () => {
    billingState.readiness = gatedReadiness(false);
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    expect(screen.queryByRole("heading", { name: "Payment Method" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create Workspace" })).toBeEnabled();
  });

  it("disables Create on Pro when the paid-intent gate is on and billing is not ready", async () => {
    billingState.readiness = gatedReadiness(false);
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(screen.getByRole("heading", { name: "Payment Method" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create Workspace" })).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("allows Pro create when the paid-intent gate is off", async () => {
    billingState.readiness = {
      ...gatedReadiness(false),
      paymentMethodRequired: false,
    };
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(screen.getByRole("heading", { name: "Payment Method" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create Workspace" })).toBeEnabled();
  });
});
