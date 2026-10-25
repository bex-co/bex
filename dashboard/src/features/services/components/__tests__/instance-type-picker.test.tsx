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

const FREE: InstanceTypeView = {
  id: "free",
  name: "Free",
  cpu: "100m",
  memory: "512Mi",
  monthlyUsd: "0.00",
};
const STANDARD: InstanceTypeView = {
  id: "standard",
  name: "Standard",
  cpu: "1",
  memory: "2Gi",
  monthlyUsd: "17.50",
};
const PRO: InstanceTypeView = {
  id: "pro",
  name: "Pro",
  cpu: "2",
  memory: "4Gi",
  monthlyUsd: "59.50",
};

function renderPicker(
  currentPlan: string | null,
  serviceType: string | null = "web_service",
) {
  const rootRoute = createRootRoute();
  const pickerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <InstanceTypePicker
        serviceId="app"
        currentPlan={currentPlan}
        serviceType={serviceType}
      />
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

  // w4/m207: one Tab stop across the Free and Paid sections; the arrows move
  // the draft, Save follows it, and restoring the plan disables Save again.
  it("moves the draft across Free and Paid with the arrows, without saving", async () => {
    const user = userEvent.setup();
    renderPicker("free");
    const free = await screen.findByRole("radio", { name: /^Free/ });
    expect(free).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("radio", { name: /Standard/ })).toHaveAttribute(
      "tabindex",
      "-1",
    );
    free.focus();
    await user.keyboard("{ArrowRight}");
    const standard = screen.getByRole("radio", { name: /Standard/ });
    expect(document.activeElement).toBe(standard);
    expect(standard).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeEnabled();
    await user.keyboard("{ArrowLeft}");
    expect(free).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeDisabled();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("offers no Free tier for a background worker (w6/025 paid-only)", async () => {
    renderPicker("standard", "background_worker");

    expect(await screen.findByText("Paid")).toBeInTheDocument();
    expect(screen.queryByText("Free")).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2); // Standard + Pro only
  });

  it("offers no Free tier for a private service either (w1/111 paid-only)", async () => {
    renderPicker("standard", "private_service");

    expect(await screen.findByText("Paid")).toBeInTheDocument();
    expect(screen.queryByText("Free")).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2); // Standard + Pro only
  });

  it("still offers Free to the type Render sells on it (w1/111 control)", async () => {
    // A web service keeps Free; cron jobs became paid-only with w8/048.
    renderPicker("standard", "web_service");

    expect(await screen.findAllByText("Free")).toHaveLength(2); // group label + card title
    expect(screen.getByText("Paid")).toBeInTheDocument();
  });

  it("offers no Free tier for a cron job (w8/048 paid-only)", async () => {
    renderPicker("standard", "cron_job");

    expect(await screen.findByText("Paid")).toBeInTheDocument();
    expect(screen.queryByText("Free")).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2);
  });

  // w4/351: a cron job created on Free before w8/048 matched no offered card,
  // so the page showed an empty selection and never said it runs on Free.
  it("says a legacy Free cron job is on Free, and pre-selects nothing", async () => {
    renderPicker("free", "cron_job");

    const note = await screen.findByRole("note");
    expect(note).toHaveTextContent("This cron job is on Free");
    // Read before the cards: the radio group is described by it.
    expect(screen.getByRole("radiogroup")).toHaveAccessibleDescription(
      note.textContent!,
    );
    expect(screen.queryByText("Free", { exact: true })).not.toBeInTheDocument();
    for (const radio of screen.getAllByRole("radio")) {
      expect(radio).toHaveAttribute("aria-checked", "false");
    }
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeDisabled();
  });

  it.each([
    ["background_worker", "free", "This background worker is on Free"],
    ["private_service", "free", "This private service is on Free"],
    ["cron_job", null, "This cron job has no instance type"],
  ])(
    "words the notice for a %s on %s",
    async (serviceType, currentPlan, opening) => {
      renderPicker(currentPlan, serviceType);
      expect(await screen.findByRole("note")).toHaveTextContent(opening);
    },
  );

  it("shows the notice while the catalog is still loading", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    renderPicker("free", "cron_job");
    expect(await screen.findByRole("note")).toHaveTextContent(
      "This cron job is on Free",
    );
  });

  it.each([
    ["free", "web_service", /^Free/],
    ["standard", "cron_job", /Standard/],
  ])(
    "adds no notice where the current plan %s is offered to a %s",
    async (currentPlan, serviceType, card) => {
      renderPicker(currentPlan, serviceType);
      expect(await screen.findByRole("radio", { name: card })).toHaveAttribute(
        "aria-checked",
        "true",
      );
      expect(screen.queryByRole("note")).toBeNull();
    },
  );

  it("shows each card's catalog price and repeats it in the confirmation (w4/400)", async () => {
    const user = userEvent.setup();
    renderPicker("standard");

    expect(await screen.findByText("$0.00/month")).toBeInTheDocument();
    expect(screen.getByText("$17.50/month")).toBeInTheDocument();
    expect(screen.getByText("$59.50/month")).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save Changes" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(/This instance type costs \$59\.50\/month\./),
    ).toBeInTheDocument();
  });

  it("renders no price when the catalog omits one (w4/400)", async () => {
    instanceTypesState.instanceTypes = [
      { ...PRO, monthlyUsd: "" },
    ];
    renderPicker("standard");
    const card = await screen.findByRole("radio", { name: /Pro\b/ });
    expect(card).not.toHaveTextContent("$");
    expect(card).not.toHaveTextContent("undefined");
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

    await user.click(
      within(dialog).getByRole("button", { name: "Save Changes" }),
    );

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
    await user.click(
      within(dialog).getByRole("button", { name: "Save Changes" }),
    );

    expect(updatePlan).toHaveBeenCalled();
    expect(screen.queryByText("settings page")).not.toBeInTheDocument();
  });

  // The catalog-loading state is a structural preview of the ready picker —
  // the same group stack, headings, grid and card frame/text rows.
  function groupShape(root: Element) {
    return [...root.querySelectorAll("[data-instance-type-group]")].map((g) => {
      const [label, grid] = [...g.children];
      return {
        group: g.className,
        label: label.className,
        grid: grid.className,
        cards: [...grid.children].map((card) => ({
          frame: ["rounded-lg", "border", "p-4"].every((c) =>
            card.classList.contains(c),
          ),
          rows: [...card.children].map((row) => row.className),
          lines: [...card.children[1].children].length,
        })),
      };
    });
  }

  function cardContent(container: HTMLElement) {
    return container.querySelector('[data-slot="card-content"]')!;
  }

  it.each([
    ["web_service", ["Free", "Paid"]],
    ["cron_job", ["Paid"]],
    ["background_worker", ["Paid"]],
    ["private_service", ["Paid"]],
  ])(
    "loads a %s catalog in its ready grouping with inert actions",
    async (serviceType, readyGroups) => {
      const catalog = [
        FREE,
        ...[
          "starter",
          "standard",
          "pro",
          "pro_plus",
          "pro_max",
          "pro_ultra",
        ].map((id) => ({ ...STANDARD, id, name: id })),
      ];
      instanceTypesState.instanceTypes = [];
      instanceTypesState.loading = true;
      const pending = renderPicker("standard", serviceType);
      await screen.findByText("Pick an Instance Type");
      const pendingShape = groupShape(cardContent(pending.container));
      expect(screen.queryAllByRole("radio")).toHaveLength(0);
      expect(
        screen.getByRole("button", { name: "Save Changes" }),
      ).toBeDisabled();
      pending.unmount();

      instanceTypesState.instanceTypes = catalog;
      instanceTypesState.loading = false;
      const ready = renderPicker("standard", serviceType);
      await screen.findByRole("radiogroup");
      const readyShape = groupShape(cardContent(ready.container));

      expect(readyShape).toHaveLength(readyGroups.length);
      for (const label of readyGroups) {
        expect(screen.getAllByText(label).length).toBeGreaterThan(0);
      }
      // Same groups, heading/grid classes, card frames and text rows.
      expect(pendingShape).toEqual(readyShape);
      expect(screen.getByRole("radio", { name: /^standard/ })).toHaveAttribute(
        "aria-checked",
        "true",
      );
      expect(
        screen.getByRole("button", { name: "Save Changes" }),
      ).toBeDisabled();
    },
  );

  it("never reserves a Free group while the service type is undecided", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    const { container } = renderPicker(null, null);
    await screen.findByText("Pick an Instance Type");

    expect(groupShape(cardContent(container))).toHaveLength(1);
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

// w8/m127: each group states its outbound-SMTP rule, and the loading skeleton
// draws the same lines so the cards do not move when the catalog arrives.
describe("InstanceTypePicker SMTP rule", () => {
  const FREE_NOTE =
    "Free services can't send email over SMTP ports 25, 465 or 587.";
  const PAID_NOTE =
    "Paid services can send email over port 587 or 2525 with STARTTLS. Ports 25 and 465 are unavailable on every plan.";

  it("notes the rule under the Free and Paid groups", async () => {
    instanceTypesState.instanceTypes = [FREE, STANDARD, PRO];
    instanceTypesState.loading = false;
    renderPicker("free");
    expect(await screen.findByText(FREE_NOTE)).toBeInTheDocument();
    expect(screen.getByText(PAID_NOTE)).toBeInTheDocument();
  });

  it("draws the same notes while the catalog loads", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    const { container } = renderPicker("free");
    await screen.findByText(PAID_NOTE);
    const skeleton = container.querySelector(
      "[data-instance-type-groups-skeleton]",
    );
    expect(skeleton).not.toBeNull();
    expect(skeleton).toHaveTextContent(FREE_NOTE);
    expect(skeleton).toHaveTextContent(PAID_NOTE);
  });
});
