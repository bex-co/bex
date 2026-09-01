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

const prepare = vi.fn();
const finalize = vi.fn();
const cancel = vi.fn();
const creationState = {
  policy: {
    mode: "off",
    paymentRequired: false,
    providerAvailable: false,
  },
  attempt: null,
};
vi.mock("@/features/workspaces/hooks/use-workspace-creation-billing", () => ({
  useWorkspaceCreationBilling: () => ({
    ...creationState,
    policyLoading: false,
    prepare,
    finalize,
    cancel,
    busy: false,
    error: null,
  }),
}));

const setCurrentWorkspaceId = vi.fn();
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    setCurrentWorkspaceId,
    currentWorkspaceId: "tea-current",
  }),
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
    context: {
      client: {} as never,
      session: {
        identity: { traits: { email: "user@example.com" } },
      } as never,
    },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  vi.clearAllMocks();
  creationState.policy = {
    mode: "off",
    paymentRequired: false,
    providerAvailable: false,
  };
  creationState.attempt = null;
  prepare.mockResolvedValue({
    id: "wca-attempt",
    workspaceId: "tea-returned-id",
    name: "acme",
    plan: "hobby",
    billingEmail: "user@example.com",
    paymentRequired: false,
    state: "prepared",
    expiresAt: "2026-09-01T00:00:00Z",
    clientSecret: "",
    publishableKey: "",
  });
  finalize.mockResolvedValue({
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
    expect(screen.getByLabelText("Billing Email")).toHaveValue(
      "user@example.com",
    );
    expect(screen.getByLabelText("Billing Email")).toHaveAttribute("readonly");
    expect(
      screen.getByText(/Used in URLs and resource names/),
    ).toBeInTheDocument();
  });

  it("does not submit an invalid slug", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "Acme HQ");
    expect(
      screen.getByRole("button", { name: "Create Workspace" }),
    ).toBeDisabled();
    expect(prepare).not.toHaveBeenCalled();
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
    finalize.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() => expect(finalize).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/new/workspace");
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Workspace slug")).toHaveValue("acme");
  });

  it("always shows Payment Method and allows optional Hobby creation", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    expect(
      screen.getByRole("heading", { name: "Payment Method" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create Workspace" }),
    ).toBeEnabled();
  });

  it("disables Create on Pro when the paid-intent gate is on and billing is not ready", async () => {
    creationState.policy = {
      mode: "all",
      paymentRequired: true,
      providerAvailable: true,
    };
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(
      screen.getByRole("heading", { name: "Payment Method" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create Workspace" }),
    ).toBeDisabled();
    expect(finalize).not.toHaveBeenCalled();
  });

  it("allows Pro create when the paid-intent gate is off", async () => {
    creationState.policy = {
      mode: "off",
      paymentRequired: false,
      providerAvailable: false,
    };
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Workspace slug"), "acme");
    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(
      screen.getByRole("heading", { name: "Payment Method" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create Workspace" }),
    ).toBeEnabled();
    expect(screen.getByLabelText("Billing Email")).not.toHaveAttribute(
      "readonly",
    );
  });
});
