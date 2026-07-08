import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { InstanceTypePicker } from "@/features/services/components/instance-type-picker";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";

const instanceTypesState: {
  instanceTypes: InstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
} = { instanceTypes: [], loading: false, error: undefined };

vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => instanceTypesState,
}));

const updatePlan = vi.fn();
vi.mock("@/features/services/hooks/use-update-plan", () => ({
  useUpdatePlan: () => ({ updatePlan, busy: false }),
}));

const FREE: InstanceTypeView = { id: "free", name: "Free", cpu: "100m", memory: "512Mi" };
const STANDARD: InstanceTypeView = {
  id: "standard",
  name: "Standard",
  cpu: "1",
  memory: "2Gi",
};
const PRO: InstanceTypeView = { id: "pro", name: "Pro", cpu: "2", memory: "4Gi" };

function renderPicker(currentPlan: string | null) {
  const rootRoute = createRootRoute();
  const pickerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <InstanceTypePicker serviceId="app" currentPlan={currentPlan} />
    ),
  });
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/settings",
    component: () => <div>settings page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([pickerRoute, settingsRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, STANDARD, PRO];
  instanceTypesState.loading = false;
  instanceTypesState.error = undefined;
  updatePlan.mockReset();
  updatePlan.mockResolvedValue(true);
});

describe("InstanceTypePicker", () => {
  it("pre-selects the current plan and keeps Save disabled until the selection changes", async () => {
    renderPicker("standard");

    const standardCard = await screen.findByRole("radio", { name: /Standard/ });
    expect(standardCard).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeDisabled();
  });

  it("enables Save once a different card is picked, and separates Free from the paid ladder", async () => {
    const user = userEvent.setup();
    renderPicker("standard");

    expect(await screen.findAllByText("Free")).toHaveLength(2); // group label + card title
    expect(screen.getByText("Paid")).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: /Pro\b/ }));
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeEnabled();
  });

  it("confirms and fires updateServicePlan with the picked Render-spelled id, then navigates to Settings on success", async () => {
    const user = userEvent.setup();
    renderPicker("standard");

    await user.click(await screen.findByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText("Change instance type to Pro?"),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Save Changes" }));

    expect(updatePlan).toHaveBeenCalledWith("app", "pro", "Pro");
    expect(await screen.findByText("settings page")).toBeInTheDocument();
  });

  it("does not navigate when the mutation fails", async () => {
    updatePlan.mockResolvedValue(false);
    const user = userEvent.setup();
    renderPicker("standard");

    await user.click(await screen.findByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save Changes" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save Changes" }));

    expect(updatePlan).toHaveBeenCalled();
    expect(screen.queryByText("settings page")).not.toBeInTheDocument();
  });

  it("shows an error card instead of the picker when the catalog query fails", async () => {
    instanceTypesState.error = new Error("network down");
    renderPicker("standard");

    expect(
      await screen.findByText("Couldn't load instance types"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup")).not.toBeInTheDocument();
  });
});
