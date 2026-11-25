import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { CronPlanPage } from "@/features/services/components/cron-plan-page";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";

/**
 * w3/105: a cron job's `/plan` is Render's CronJobUpdatePlan page: the plan
 * cards open and ready to change, Cancel back to Settings, and a change that
 * opens the new build (bex's deploy). docs/render-artifacts/cron-dashboard.md.
 */
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

function plan(id: string, planId: string, usdPerMinute: string) {
  return {
    id,
    planId,
    name: id[0].toUpperCase() + id.slice(1),
    cpu: "1",
    memory: "2Gi",
    monthlyUsd: "",
    usdPerMinute,
    available: true,
    unavailableReason: "",
  } satisfies InstanceTypeView;
}
const STARTER = plan("starter", "0.5c-512mb", "0.00016");
const STANDARD = plan("standard", "1c-2g", "0.00058");
vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    instanceTypes: [STARTER, STANDARD],
    loading: false,
    error: undefined,
    byID: (id: string | null | undefined) =>
      [STARTER, STANDARD].find((it) => it.id === id),
  }),
}));

const updatePlan = vi.fn<() => Promise<{ deployId: string | null } | null>>();
vi.mock("@/features/services/hooks/use-update-plan", () => ({
  useUpdatePlan: () => ({ updatePlan, busy: false }),
}));

function cronJob(): ServiceView {
  return {
    id: "srv-1",
    name: "nightly",
    type: "cron_job",
    suspended: false,
    phase: "Running",
    revision: "r1",
    plan: "starter",
  } as ServiceView;
}

function renderPlanPage() {
  const rootRoute = createRootRoute();
  const planRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/plan",
    component: () => <CronPlanPage serviceId="srv-1" />,
  });
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/settings",
    component: () => <div>settings page</div>,
  });
  const deployRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => <div>deploy {deployRoute.useParams().deployId}</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([planRoute, settingsRoute, deployRoute]),
    history: createMemoryHistory({ initialEntries: ["/services/srv-1/plan"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

async function changeToStandard() {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("radio", { name: /Standard/ }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  const dialog = await screen.findByRole("alertdialog");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
}

beforeEach(() => {
  serverState.service = cronJob();
  serverState.loading = false;
  updatePlan.mockReset();
  updatePlan.mockResolvedValue({ deployId: "dep-9" });
});

describe("a cron job's plan page (w3/105)", () => {
  it("opens the plan cards ready to change, the current plan picked", async () => {
    renderPlanPage();

    expect(await screen.findByRole("radiogroup")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Starter/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  });

  it("returns to Settings on Cancel", async () => {
    const router = renderPlanPage();

    await userEvent
      .setup()
      .click(await screen.findByRole("button", { name: "Cancel" }));

    expect(await screen.findByText("settings page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/services/srv-1/settings");
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("opens the deploy a change started, as Render opens the new build", async () => {
    const router = renderPlanPage();

    await changeToStandard();

    expect(await screen.findByText("deploy dep-9")).toBeInTheDocument();
    expect(updatePlan).toHaveBeenCalledWith(
      "srv-1",
      "standard",
      expect.stringContaining("Standard"),
    );
    expect(router.state.location.pathname).toBe(
      "/services/srv-1/deploys/dep-9",
    );
  });

  it("returns to Settings after a change that started no deploy", async () => {
    updatePlan.mockResolvedValue({ deployId: null });
    renderPlanPage();

    await changeToStandard();

    expect(await screen.findByText("settings page")).toBeInTheDocument();
  });

  it("keeps the cards open when the change is refused", async () => {
    updatePlan.mockResolvedValue(null);
    const router = renderPlanPage();

    await changeToStandard();

    expect(updatePlan).toHaveBeenCalled();
    expect(screen.getByRole("radiogroup")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/services/srv-1/plan");
  });

  it("draws the plan page's pending frame while the service loads", async () => {
    serverState.service = null;
    serverState.loading = true;
    renderPlanPage();

    await waitFor(() =>
      expect(
        document.querySelector('[data-route-skeleton="cron-plan"]'),
      ).not.toBeNull(),
    );
    expect(screen.queryByRole("radiogroup")).toBeNull();
  });
});
