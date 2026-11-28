import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  useSearch,
} from "@tanstack/react-router";
import type { HistoryState } from "@tanstack/history";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import type { WorkspaceView } from "@/features/workspaces/types";
import type { PlanChangeBlocker } from "@/features/workspaces/hooks/use-plan-change-blockers";

// Render's plan pages (w3/m708, docs/render-artifacts/workspace-plan-change.md)
// under a real router: what each section draws, what holds Select plan and
// Confirm, and where each action goes.

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

const workspace: WorkspaceView = {
  id: "tea-1",
  name: "Acme",
  slug: "acme",
  plan: "hobby",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
};
const workspaceState = vi.hoisted(() => ({
  current: null as WorkspaceView | null,
  list: [] as WorkspaceView[],
  loading: false,
}));
const refetch = vi.fn();
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    currentWorkspace: workspaceState.current,
    currentWorkspaceId: workspaceState.current?.id ?? null,
    workspaces: workspaceState.list,
    loading: workspaceState.loading,
    refetch,
  }),
}));

const capabilityState = vi.hoisted(() => ({
  canManage: true,
  canManageBilling: true,
  loading: false,
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ ...capabilityState, reasonKey: () => null }),
}));

const readiness = vi.hoisted(() => ({
  value: {
    paymentMethodRequired: true,
    paymentMethodReady: true,
  } as Record<string, unknown> | null,
}));
const billingOptions = vi.hoisted(() => ({ returnPath: "" }));
vi.mock("@/features/usage/hooks/use-billing-onboarding", () => ({
  useBillingOnboarding: (options: { returnPath: string }) => {
    billingOptions.returnPath = options.returnPath;
    return {
      readiness: readiness.value && {
        workspaceId: "tea-1",
        mode: "test",
        customerReady: true,
        subscriptionReady: true,
        paymentMethodBrand: "visa",
        paymentMethodLast4: "4242",
        paymentMethodOnboardingRequired: false,
        paymentMethodRejection: "",
        lifecycle: {
          status: "healthy",
          reason: "",
          graceDeadline: "",
          enforcementOwned: false,
          recoveryPending: false,
          allowedActions: [],
          updatedAt: "",
          overdueInvoice: null,
        },
        tax: {},
        ...readiness.value,
      },
      loading: false,
      error: undefined,
      checkoutBusy: false,
      portalBusy: false,
      openCheckout: vi.fn(),
      openPortal: vi.fn(),
      refetch: vi.fn(),
    };
  },
}));

const blockerState = vi.hoisted(() => ({
  value: [] as PlanChangeBlocker[],
  asked: [] as (string | null)[],
}));
vi.mock("@/features/workspaces/hooks/use-plan-change-blockers", () => ({
  usePlanChangeBlockers: (_id: string | undefined, plan: string | null) => {
    blockerState.asked.push(plan);
    return { blockers: plan ? blockerState.value : [], loading: false };
  },
}));

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { UpdatePlanPage } from "@/features/workspaces/components/update-plan-page";
import { UpdatePlanConfirmPage } from "@/features/workspaces/components/update-plan-confirm-page";
import {
  planFromSearch,
  validateUpdatePlanSearch,
  type UpdatePlanSearch,
} from "@/features/workspaces/lib/update-plan";

/** The plan pages' validated search, as their routes hand it over. */
const useRouteSearch = () => useSearch({ strict: false }) as UpdatePlanSearch;

async function renderAt(href: string, state?: HistoryState) {
  const rootRoute = createRootRoute();
  const planRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/billing/update-plan",
    validateSearch: validateUpdatePlanSearch,
    component: function PlanPage() {
      return <UpdatePlanPage search={useRouteSearch()} />;
    },
  });
  const confirmRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/billing/update-plan-confirm",
    validateSearch: validateUpdatePlanSearch,
    component: function ConfirmPage() {
      const search = useRouteSearch();
      const plan = planFromSearch(search.plan);
      return plan ? (
        <UpdatePlanConfirmPage plan={plan} search={search} />
      ) : null;
    },
  });
  const landings = ["/", "/webhooks", "/billing"].map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => <p>landed on {path}</p>,
    }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren([planRoute, confirmRoute, ...landings]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  render(<RouterProvider router={router} />);
  await router.navigate({ href, state });
  return router;
}

const selectPlan = () => screen.getByRole("button", { name: /^Select plan$/ });
const confirm = () => screen.getByRole("button", { name: /^Confirm$/ });

beforeEach(() => {
  workspaceState.current = workspace;
  workspaceState.list = [workspace];
  workspaceState.loading = false;
  capabilityState.canManage = true;
  capabilityState.loading = false;
  readiness.value = { paymentMethodRequired: true, paymentMethodReady: true };
  blockerState.value = [];
  blockerState.asked = [];
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
  refetch.mockReset();
});

describe("/billing/update-plan", () => {
  it("draws Render's sections with ?plan= choosing the card", async () => {
    await renderAt("/billing/update-plan?plan=professional");

    expect(
      await screen.findByRole("heading", { name: "Change plan for Acme" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Select a plan" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "pricing page" })).toHaveAttribute(
      "href",
      "https://bex.co/pricing",
    );
    const pro = screen.getByRole("radio", { name: /Pro/ });
    expect(pro).toBeChecked();
    expect(pro).toHaveTextContent("New plan");
    expect(screen.getByRole("radio", { name: /Hobby/ })).toHaveTextContent(
      "Current plan",
    );
    expect(screen.getByRole("link", { name: "Get in touch" })).toHaveAttribute(
      "href",
      "mailto:hello@bex.co",
    );
    expect(
      screen.getByRole("heading", { name: "Payment Method" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "This workspace plan requires a card for paid services and compute.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "How billing works" }),
    ).toHaveAttribute("href", "https://bex.co/docs/platform/billing-setup");
    expect(screen.getByText("Workspace billing")).toBeInTheDocument();
    expect(screen.getByText("Visa ending in 4242")).toBeInTheDocument();
    // Stripe's hosted pages come back to this same URL.
    expect(billingOptions.returnPath).toBe(
      "/billing/update-plan?plan=professional",
    );
    expect(selectPlan()).toBeEnabled();
  });

  it("starts on the current plan, with Select plan held and no card notice for Hobby", async () => {
    await renderAt("/billing/update-plan");

    expect(await screen.findByRole("radio", { name: /Hobby/ })).toBeChecked();
    expect(
      screen.queryByText(
        "This workspace plan requires a card for paid services and compute.",
      ),
    ).not.toBeInTheDocument();
    expect(selectPlan()).toBeDisabled();
    // Nothing to ask bex-api about the plan the workspace is on.
    expect(blockerState.asked.every((plan) => plan === null)).toBe(true);
  });

  it("holds a paid plan while the platform needs a card and none is on file", async () => {
    readiness.value = {
      paymentMethodRequired: true,
      paymentMethodReady: false,
    };
    await renderAt("/billing/update-plan?plan=pro");

    await screen.findByRole("heading", { name: "Change plan for Acme" });
    expect(selectPlan()).toBeDisabled();
    expect(
      screen.getAllByText(
        "There are some errors in your selection. Please review and fix them to continue.",
      ).length,
    ).toBeGreaterThan(0);
  });

  it("holds Select plan while bex-api names a blocker, and frees it once none is left", async () => {
    workspaceState.current = { ...workspace, plan: "pro" };
    blockerState.value = [
      {
        kind: "member_limit",
        items: ["carol@example.com:developer"],
        limit: 1,
      },
    ];
    const user = userEvent.setup();
    await renderAt("/billing/update-plan?plan=hobby");

    expect(await screen.findByText("Too many users in team.")).toBeVisible();
    expect(blockerState.asked).toContain("hobby");
    expect(selectPlan()).toBeDisabled();

    blockerState.value = [];
    await user.click(screen.getByRole("radio", { name: /Scale/ }));
    await user.click(screen.getByRole("radio", { name: /Hobby/ }));
    expect(screen.queryByText("Too many users in team.")).toBeNull();
    expect(selectPlan()).toBeEnabled();
  });

  it("applies an upgrade here, as Render's does, and returns to next", async () => {
    const user = userEvent.setup();
    const router = await renderAt(
      "/billing/update-plan?plan=professional&next=/webhooks",
    );

    await user.click(
      await screen.findByRole("button", { name: /^Select plan$/ }),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/webhooks"),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", plan: "pro" },
    });
    expect(refetch).toHaveBeenCalled();
  });

  it("keeps the page with Render's card message when bex-api wants a payment method", async () => {
    mutate.mockRejectedValue(codedGraphQLError("PAYMENT_REQUIRED"));
    const user = userEvent.setup();
    const router = await renderAt("/billing/update-plan?plan=professional");

    await user.click(
      await screen.findByRole("button", { name: /^Select plan$/ }),
    );
    expect(
      (
        await screen.findAllByText(
          "An error occurred. Add a valid card to upgrade to a paid plan.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(router.state.location.pathname).toBe("/billing/update-plan");
  });

  it("takes a downgrade to the confirmation, carrying next", async () => {
    workspaceState.current = { ...workspace, plan: "pro" };
    const user = userEvent.setup();
    const router = await renderAt(
      "/billing/update-plan?plan=hobby&next=/webhooks",
    );

    await user.click(
      await screen.findByRole("button", { name: /^Select plan$/ }),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(
        "/billing/update-plan-confirm",
      ),
    );
    expect(router.state.location.search).toEqual({
      plan: "hobby",
      next: "/webhooks",
    });
    expect(mutate).not.toHaveBeenCalled();
  });

  it.each([
    ["next", "/billing/update-plan?next=/webhooks", undefined, "/webhooks"],
    [
      "what the link handed over",
      "/billing/update-plan",
      { afterPlanUpdate: "/billing" },
      "/billing",
    ],
    ["home", "/billing/update-plan", undefined, "/"],
  ])("cancels back to %s", async (_, href, state, landing) => {
    const user = userEvent.setup();
    const router = await renderAt(href, state as HistoryState | undefined);

    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(router.state.location.pathname).toBe(landing));
  });

  it("tells a member who cannot change the plan why, and lets them choose nothing", async () => {
    capabilityState.canManage = false;
    await renderAt("/billing/update-plan?plan=pro");

    expect(
      await screen.findByText(
        "Your role can’t change this workspace’s plan. Ask a workspace admin.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Scale/ })).toBeDisabled();
    expect(selectPlan()).toBeDisabled();
    expect(blockerState.asked.every((plan) => plan === null)).toBe(true);
  });

  it("shows a secondary's plan as its organization's", async () => {
    const secondary: WorkspaceView = {
      ...workspace,
      plan: "scale",
      organization: {
        id: "org-1",
        name: "Org",
        primaryWorkspaceId: "tea-primary",
        requireTwoFactor: false,
        requireGoogleLogin: false,
      },
      primary: false,
    };
    workspaceState.current = secondary;
    workspaceState.list = [secondary];
    await renderAt("/billing/update-plan?plan=pro");

    expect(
      await screen.findByTestId("organization-plan-note"),
    ).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Hobby/ })).toBeDisabled();
    expect(selectPlan()).toBeDisabled();
  });
});

describe("/billing/update-plan-confirm", () => {
  beforeEach(() => {
    workspaceState.current = {
      ...workspace,
      name: "Tian  Pan's Workspace",
      plan: "pro",
    };
    workspaceState.list = [workspaceState.current];
  });

  it("asks for Render's typed phrase before a downgrade, and returns to next", async () => {
    const user = userEvent.setup();
    const router = await renderAt(
      "/billing/update-plan-confirm?plan=hobby&next=/webhooks",
    );

    expect(
      await screen.findByRole("heading", {
        name: "Change Plan for Tian Pan's Workspace",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "New plan summary" }),
    ).toBeInTheDocument();
    const downgrade = screen
      .getByRole("heading", { name: "Confirm downgrade" })
      .closest("section")!;
    expect(downgrade).toHaveTextContent(
      "To initiate the changes to your plan, type sudo downgrade workspace Tian Pan's Workspace in the text box and click the confirm button.",
    );
    const input = within(downgrade).getByLabelText("Sudo Command");
    expect(input).toHaveFocus();
    expect(confirm()).toBeDisabled();

    await user.type(input, "sudo downgrade workspace Tian Pan");
    expect(confirm()).toBeDisabled();
    await user.clear(input);
    await user.type(
      input,
      "  sudo downgrade   workspace Tian Pan's Workspace ",
    );
    expect(confirm()).toBeEnabled();

    await user.click(confirm());
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/webhooks"),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", plan: "hobby" },
    });
  });

  it("confirms a plan above the current one without the phrase, as Render's does", async () => {
    await renderAt("/billing/update-plan-confirm?plan=organization");

    await screen.findByRole("heading", { name: "New plan summary" });
    expect(
      screen.queryByRole("heading", { name: "Confirm downgrade" }),
    ).not.toBeInTheDocument();
    expect(confirm()).toBeEnabled();
  });

  it("sets the two plans side by side with what changes", async () => {
    await renderAt("/billing/update-plan-confirm?plan=hobby");

    await screen.findByText("Current Plan");
    expect(screen.getByText("New Plan")).toBeInTheDocument();
    expect(
      screen.getByText("14-day window for logs and metrics"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("7-day window for logs and metrics"),
    ).toBeInTheDocument();
    expect(screen.getByText("Just for me")).toBeInTheDocument();
  });

  it("keeps the page with Render's error when the change fails", async () => {
    mutate.mockRejectedValue(new Error("Failed to fetch"));
    const user = userEvent.setup();
    const router = await renderAt(
      "/billing/update-plan-confirm?plan=organization",
    );

    await user.click(await screen.findByRole("button", { name: /^Confirm$/ }));
    const alert = await screen.findByText("An error occurred.");
    expect(alert.closest('[role="alert"]')).toHaveTextContent(
      "Error updating plan.",
    );
    expect(router.state.location.pathname).toBe("/billing/update-plan-confirm");
  });

  it("draws a raced org refusal's preconditions as Render's blockers", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("ORGANIZATION_DOWNGRADE_BLOCKED", {
        preconditions: [{ kind: "secondary_workspaces", items: ["beta"] }],
      }),
    );
    const user = userEvent.setup();
    await renderAt("/billing/update-plan-confirm?plan=organization");

    await user.click(await screen.findByRole("button", { name: /^Confirm$/ }));
    expect(await screen.findByText("Too many workspaces.")).toBeInTheDocument();
  });

  it("relays the server's own reason", async () => {
    mutate.mockRejectedValue(
      uncodedGraphQLError(
        "workspace has 30 services, exceeds hobby plan's limit of 25",
      ),
    );
    const user = userEvent.setup();
    await renderAt("/billing/update-plan-confirm?plan=organization");

    await user.click(await screen.findByRole("button", { name: /^Confirm$/ }));
    expect(
      await screen.findByText(
        "Workspace has 30 services, exceeds hobby plan's limit of 25",
      ),
    ).toBeInTheDocument();
  });

  it("lists the organization's removal before an org primary goes below Scale", async () => {
    workspaceState.current = {
      ...workspace,
      plan: "scale",
      organization: {
        id: "org-1",
        name: "Org",
        primaryWorkspaceId: "tea-1",
        requireTwoFactor: false,
        requireGoogleLogin: false,
      },
      primary: true,
    };
    await renderAt("/billing/update-plan-confirm?plan=professional");

    const downgrade = (
      await screen.findByRole("heading", { name: "Confirm downgrade" })
    ).closest("section")!;
    expect(downgrade).toHaveTextContent(
      "To initiate the changes to your plan, including:",
    );
    expect(within(downgrade).getByRole("listitem")).toHaveTextContent(
      "The organization will be removed.",
    );
    expect(downgrade).toHaveTextContent(
      "Type sudo downgrade workspace Acme in the text box and click the confirm button.",
    );
  });

  it("goes Back to the plan page, keeping where it returns", async () => {
    const user = userEvent.setup();
    const router = await renderAt(
      "/billing/update-plan-confirm?plan=hobby&next=/webhooks",
    );

    await user.click(await screen.findByRole("button", { name: /^Back$/ }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/billing/update-plan"),
    );
    expect(router.state.location.search).toEqual({ next: "/webhooks" });
  });
});

// AGENTS.md: a pending frame previews the ready page's geometry. The
// confirmation's comparison is static plan facts, so its pending card is the
// ready card with its words reserved.
describe("the plan pages' pending frames", () => {
  it("previews the comparison card with the ready card's anatomy and copy", async () => {
    const { shape } = await import("@/test/card-shape");
    const { PlanComparisonCard } =
      await import("@/features/workspaces/components/plan-comparison-card");
    const { container: ready } = render(
      <PlanComparisonCard current="pro" next="hobby" />,
    );
    const { container: pending } = render(
      <PlanComparisonCard current="pro" next="hobby" pending />,
    );
    const words = (el: Element) => el.textContent;
    expect(words(pending.firstElementChild!)).toBe(
      words(ready.firstElementChild!),
    );
    // The same boxes once each line's reserved span is set aside.
    const anatomy = (el: Element): string => {
      if (
        el.matches("span.relative") &&
        el.firstElementChild?.matches("span.invisible")
      ) {
        return "";
      }
      const children = [...el.children].map(anatomy).filter(Boolean);
      return `${el.tagName}.${el.getAttribute("class") ?? ""}[${children.join(",")}]`;
    };
    expect(anatomy(pending.firstElementChild!)).toBe(
      anatomy(ready.firstElementChild!),
    );
    expect(shape(ready.firstElementChild!)).not.toBe(
      shape(pending.firstElementChild!),
    );
  });

  it("draws the confirmation's sections while the workspace loads", async () => {
    workspaceState.current = null;
    workspaceState.loading = true;
    await renderAt("/billing/update-plan-confirm?plan=hobby");
    await waitFor(() =>
      expect(
        document.querySelector('[data-skeleton-region="plan-summary"]'),
      ).not.toBeNull(),
    );
    expect(
      document.querySelector('[data-skeleton-region="confirm-downgrade"]'),
    ).not.toBeNull();
  });

  it("draws the plan page's sections while the workspace loads", async () => {
    workspaceState.current = null;
    workspaceState.loading = true;
    await renderAt("/billing/update-plan?plan=pro");
    await waitFor(() =>
      expect(
        document.querySelector('[data-skeleton-region="select-plan"]'),
      ).not.toBeNull(),
    );
    expect(
      document.querySelector('[data-skeleton-region="payment-method"]'),
    ).not.toBeNull();
    expect(
      screen.getByRole("button", { name: /^Select plan$/ }),
    ).toBeDisabled();
  });
});
