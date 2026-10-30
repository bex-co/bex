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
import type { WorkspaceCreationAttempt } from "@/features/workspaces/hooks/use-workspace-creation-billing";
import { serverRendered } from "@/test/hydration";

const prepare = vi.fn();
const finalize = vi.fn();
const cancel = vi.fn();
const creationState = {
  policy: {
    mode: "off",
    paymentRequired: false,
    providerAvailable: false,
  },
  attempt: null as WorkspaceCreationAttempt | null,
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

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args) },
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

function pageRouter() {
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
  return router;
}

function renderPage() {
  const router = pageRouter();
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

  it("stays on the page and says so when cancelling the attempt fails", async () => {
    cancel.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.click(await screen.findByRole("button", { name: "Cancel" }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "Couldn't cancel workspace creation. Try again.",
      ),
    );
    expect(router.state.location.pathname).toBe("/new/workspace");
  });

  it("leaves the page once the attempt is cancelled", async () => {
    cancel.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.click(await screen.findByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(toastError).not.toHaveBeenCalled();
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

// w4/m219 (a846): a slug typed into the server-rendered form before handlers
// attach is wiped by hydration.
it("keeps the slug uneditable until hydration, then focuses it (w4/m219)", async () => {
  const router = pageRouter();
  await router.load();
  const { hydrate, getByLabelText } = serverRendered(
    <RouterProvider router={router} />,
  );
  const slug = () => getByLabelText<HTMLInputElement>("Workspace slug");
  expect(slug().disabled).toBe(true);
  await hydrate();
  expect(slug().disabled).toBe(false);
  expect(slug()).toHaveFocus();
  await userEvent.type(slug(), "Acme-Prod");
  expect(slug().value).toBe("acme-prod");
});

it("hands focus to the slug only when nothing else holds it (w4/m219)", async () => {
  const router = pageRouter();
  await router.load();
  const { hydrate, getByLabelText } = serverRendered(
    <RouterProvider router={router} />,
  );
  const email = getByLabelText<HTMLInputElement>(/Billing email/i);
  email.focus();
  await hydrate();
  expect(email).toHaveFocus();
});

it("keeps a resumed attempt's slug locked after hydration (w4/m219)", async () => {
  creationState.attempt = {
    id: "wca-resume",
    name: "acme",
    plan: "hobby",
    billingEmail: "user@example.com",
    paymentRequired: false,
    state: "prepared",
    clientSecret: "",
    publishableKey: "",
  };
  const router = pageRouter();
  await router.load();
  const { hydrate, getByLabelText } = serverRendered(
    <RouterProvider router={router} />,
  );
  const slug = () => getByLabelText<HTMLInputElement>("Workspace slug");
  await hydrate();
  expect(slug().disabled).toBe(true);
  expect(slug().value).toBe("acme");
});
