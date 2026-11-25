import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ComputePlanSection } from "@/features/services/components/compute-plan-section";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";

const instanceTypesState: {
  instanceTypes: InstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
} = { instanceTypes: [], loading: false, error: undefined };

vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    ...instanceTypesState,
    byID: (id: string | null | undefined) =>
      instanceTypesState.instanceTypes.find((it) => it.id === id),
  }),
}));

const updatePlan = vi.fn();
vi.mock("@/features/services/hooks/use-update-plan", () => ({
  useUpdatePlan: () => ({ updatePlan, busy: false }),
}));

const FREE: InstanceTypeView = {
  id: "free",
  planId: "free",
  name: "Free",
  cpu: "100m",
  memory: "512Mi",
  monthlyUsd: "0.00",
  available: true,
  unavailableReason: "",
};
const STANDARD: InstanceTypeView = {
  id: "standard",
  planId: "1c-2g",
  name: "Standard",
  cpu: "1",
  memory: "2Gi",
  monthlyUsd: "17.50",
  available: true,
  unavailableReason: "",
};
const PRO: InstanceTypeView = {
  id: "pro",
  planId: "2c-4g",
  name: "Pro",
  cpu: "2",
  memory: "4Gi",
  monthlyUsd: "59.50",
  available: true,
  unavailableReason: "",
};

// The picker is the Compute section's edit mode (w6/m483): render the
// section and open it with Edit, as a user does.
async function renderPicker(
  currentPlan: string | null,
  serviceType: string | null = "web_service",
  neverLive = false,
) {
  const rootRoute = createRootRoute();
  const pickerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <ComputePlanSection
        serviceId="app"
        currentPlan={currentPlan}
        serviceType={serviceType}
        live={!neverLive}
      />
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([pickerRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  const view = render(<RouterProvider router={router} />);
  fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
  return view;
}

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, STANDARD, PRO];
  instanceTypesState.loading = false;
  instanceTypesState.error = undefined;
  updatePlan.mockReset();
  updatePlan.mockResolvedValue({ deployId: null });
});

describe("InstanceTypePicker", () => {
  it("pre-selects the current plan and keeps Save disabled until the selection changes", async () => {
    await renderPicker("standard");

    const standardCard = await screen.findByRole("radio", { name: /Standard/ });
    expect(standardCard).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  // w6/m483: Render's plan ID first, the legacy name beside it; Free has no
  // legacy name, so its card keeps the name.
  it("names each paid card by its Render plan ID beside the legacy name", async () => {
    await renderPicker("standard");

    const standard = await screen.findByRole("radio", { name: /Standard/ });
    expect(standard.querySelector(".font-mono")).toHaveTextContent(/^1c-2g$/);
    expect(standard).toHaveAccessibleName(/^1c-2g Standard Current 2 GB/);
    expect(
      screen.getByRole("radio", { name: /^2c-4g Pro 4 GB/ }),
    ).toBeInTheDocument();
    const free = screen.getByRole("radio", { name: /^Free 512 MB/ });
    expect(free.querySelector(".font-mono")).toBeNull();
  });

  it("enables Save once a different card is picked, and separates Free from the paid ladder", async () => {
    const user = userEvent.setup();
    await renderPicker("standard");

    expect(await screen.findAllByText("Free")).toHaveLength(2); // group label + card title
    expect(screen.getByText("Paid")).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: /Pro\b/ }));
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  // w4/m207: one Tab stop across the Free and Paid sections; the arrows move
  // the draft, Save follows it, and restoring the plan disables Save again.
  it("moves the draft across Free and Paid with the arrows, without saving", async () => {
    const user = userEvent.setup();
    await renderPicker("free");
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
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
    await user.keyboard("{ArrowLeft}");
    expect(free).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("offers no Free tier for a background worker (w6/025 paid-only)", async () => {
    await renderPicker("standard", "background_worker");

    expect(await screen.findByText("Paid")).toBeInTheDocument();
    expect(screen.queryByText("Free")).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2); // Standard + Pro only
  });

  it("offers no Free tier for a private service either (w1/111 paid-only)", async () => {
    await renderPicker("standard", "private_service");

    expect(await screen.findByText("Paid")).toBeInTheDocument();
    expect(screen.queryByText("Free")).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2); // Standard + Pro only
  });

  it("still offers Free to the type Render sells on it (w1/111 control)", async () => {
    // A web service keeps Free; cron jobs became paid-only with w8/048.
    await renderPicker("standard", "web_service");

    expect(await screen.findAllByText("Free")).toHaveLength(2); // group label + card title
    expect(screen.getByText("Paid")).toBeInTheDocument();
  });

  it("offers no Free tier for a cron job (w8/048 paid-only)", async () => {
    await renderPicker("standard", "cron_job");

    expect(await screen.findByText("Paid")).toBeInTheDocument();
    expect(screen.queryByText("Free")).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2);
  });

  // w4/351: a cron job created on Free before w8/048 matched no offered card,
  // so the page showed an empty selection and never said it runs on Free.
  it("says a legacy Free cron job is on Free, and pre-selects nothing", async () => {
    await renderPicker("free", "cron_job");

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
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it.each([
    ["background_worker", "free", "This background worker is on Free"],
    ["private_service", "free", "This private service is on Free"],
    ["cron_job", null, "This cron job has no compute plan"],
  ])(
    "words the notice for a %s on %s",
    async (serviceType, currentPlan, opening) => {
      await renderPicker(currentPlan, serviceType);
      expect(await screen.findByRole("note")).toHaveTextContent(opening);
    },
  );

  it("shows the notice while the catalog is still loading", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    await renderPicker("free", "cron_job");
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
      await renderPicker(currentPlan, serviceType);
      expect(await screen.findByRole("radio", { name: card })).toHaveAttribute(
        "aria-checked",
        "true",
      );
      expect(screen.queryByRole("note")).toBeNull();
    },
  );

  it("shows each card's catalog price and repeats it in the confirmation (w4/400)", async () => {
    const user = userEvent.setup();
    await renderPicker("standard");

    expect(await screen.findByText("$0.00/month")).toBeInTheDocument();
    expect(screen.getByText("$17.50/month")).toBeInTheDocument();
    expect(screen.getByText("$59.50/month")).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(/This compute plan costs \$59\.50\/month\./),
    ).toBeInTheDocument();
  });

  // w4/546: a cron job is billed per second only while a run executes, so
  // its cards and the confirmation price run time, never the always-on month.
  it("prices a cron job's instance types per minute of run time", async () => {
    instanceTypesState.instanceTypes = [
      { ...STANDARD, usdPerMinute: "0.0004" },
      { ...PRO, usdPerMinute: "0.0014" },
    ];
    const user = userEvent.setup();
    await renderPicker("standard", "cron_job");

    expect(
      await screen.findByText("$0.0004/min of run time"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/\/month/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(
        /costs \$0\.0014 per minute of run time, billed per second while a run executes\. The minimum charge is \$0\.70 a month per cron job\./,
      ),
    ).toBeInTheDocument();
  });

  // w9/m448: the Plan page states the cron job minimum above a cron job's
  // instance types, as the create form does, and never for another type.
  it("states the cron job minimum on a cron job's Plan page only", async () => {
    const view = await renderPicker("standard", "cron_job");
    expect(
      await screen.findByText(
        "A cron job is billed per second while a run executes. The minimum charge is $0.70 a month per cron job.",
      ),
    ).toBeInTheDocument();
    view.unmount();
    await renderPicker("standard", "web_service");
    await screen.findByRole("radiogroup");
    expect(screen.queryByText(/minimum charge/)).not.toBeInTheDocument();
  });

  // w4/548: the resize confirmation promises a rolling replacement only when
  // a release is live; a never-live service has nothing serving, and a cron
  // job serves no requests at all.
  it.each([
    [
      "a live service",
      "web_service",
      false,
      "resizes and rolls with no downtime",
    ],
    [
      "a never-live service",
      "web_service",
      true,
      "Nothing is serving yet — it goes live when a deploy succeeds.",
    ],
    [
      "a cron job",
      "cron_job",
      false,
      "Runs that start after the change use the new compute plan.",
    ],
    // A cron job serves nothing either way, so its own copy wins.
    [
      "a never-run cron job",
      "cron_job",
      true,
      "Runs that start after the change use the new compute plan.",
    ],
  ])(
    "confirms a resize truthfully for %s",
    async (_, type, neverLive, want) => {
      const user = userEvent.setup();
      await renderPicker("standard", type, neverLive);
      await user.click(await screen.findByRole("radio", { name: /Pro\b/ }));
      await user.click(screen.getByRole("button", { name: "Save" }));
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent(want);
      if (neverLive) {
        expect(dialog).not.toHaveTextContent("in-flight requests");
      }
    },
  );

  it("renders no price when the catalog omits one (w4/400)", async () => {
    instanceTypesState.instanceTypes = [{ ...PRO, monthlyUsd: "" }];
    await renderPicker("standard");
    const card = await screen.findByRole("radio", { name: /Pro\b/ });
    expect(card).not.toHaveTextContent("$");
    expect(card).not.toHaveTextContent("undefined");
  });

  it("confirms and fires updateServicePlan with the picked Render-spelled id, then closes the cards on success", async () => {
    const user = userEvent.setup();
    await renderPicker("standard");

    await user.click(await screen.findByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText("Change compute plan to 2c-4g (Pro)?"),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(updatePlan).toHaveBeenCalledWith("app", "pro", "2c-4g (Pro)");
    expect(
      await screen.findByRole("button", { name: "Edit" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup")).toBeNull();
  });

  it("keeps the cards open when the mutation fails", async () => {
    updatePlan.mockResolvedValue(null);
    const user = userEvent.setup();
    await renderPicker("standard");

    await user.click(await screen.findByRole("radio", { name: /Pro\b/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(updatePlan).toHaveBeenCalled();
    expect(screen.getByRole("radiogroup")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  });

  // w2/m250: a paused tier stays listed, disabled with bex-api's reason, and
  // can never be submitted — not by click, nor by arrowing past it.
  const PAUSED_REASON =
    '"Pro Ultra" is temporarily unavailable; choose Pro Max or a smaller compute plan.';
  const ULTRA: InstanceTypeView = {
    id: "pro_ultra",
    name: "Pro Ultra",
    cpu: "8",
    memory: "32Gi",
    monthlyUsd: "450.00",
    available: false,
    unavailableReason: PAUSED_REASON,
  };

  it("shows a paused tier disabled with its reason and never submits it", async () => {
    instanceTypesState.instanceTypes = [FREE, STANDARD, PRO, ULTRA];
    const user = userEvent.setup();
    await renderPicker("pro");

    const ultra = await screen.findByRole("radio", { name: /Pro Ultra/ });
    expect(ultra).toBeDisabled();
    expect(ultra).toHaveAttribute("title", PAUSED_REASON);
    expect(ultra).toHaveTextContent("Unavailable");
    expect(ultra).toHaveTextContent(PAUSED_REASON);
    expect(ultra).not.toHaveTextContent("$450.00/month");

    await user.click(ultra);
    expect(ultra).toHaveAttribute("aria-checked", "false");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    // Arrowing on from the last available card wraps past the paused one.
    // The service is on Pro, so its card carries the Current badge.
    const pro = screen.getByRole("radio", { name: /^2c-4g Pro Current 4 GB/ });
    pro.focus();
    await user.keyboard("{ArrowRight}");
    expect(ultra).toHaveAttribute("aria-checked", "false");
    expect(screen.getByRole("radio", { name: /^Free/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("still shows a service already on a paused tier as its current plan", async () => {
    instanceTypesState.instanceTypes = [FREE, STANDARD, PRO, ULTRA];
    await renderPicker("pro_ultra");

    const ultra = await screen.findByRole("radio", { name: /Pro Ultra/ });
    expect(ultra).toHaveAttribute("aria-checked", "true");
    expect(ultra).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
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
        ].map((id) => ({ ...STANDARD, id, planId: id, name: id })),
      ];
      instanceTypesState.instanceTypes = [];
      instanceTypesState.loading = true;
      const pending = await renderPicker("standard", serviceType);
      await screen.findByRole("button", { name: "Cancel" });
      const pendingShape = groupShape(cardContent(pending.container));
      expect(screen.queryAllByRole("radio")).toHaveLength(0);
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
      pending.unmount();

      instanceTypesState.instanceTypes = catalog;
      instanceTypesState.loading = false;
      const ready = await renderPicker("standard", serviceType);
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
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    },
  );

  it("never reserves a Free group while the service type is undecided", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    const { container } = await renderPicker(null, null);
    await screen.findByRole("button", { name: "Cancel" });

    expect(groupShape(cardContent(container))).toHaveLength(1);
  });

  it("shows an error card instead of the picker when the catalog query fails", async () => {
    instanceTypesState.error = new Error("network down");
    await renderPicker("standard");

    expect(
      await screen.findByText("Couldn't load compute plans"),
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
    await renderPicker("free");
    expect(await screen.findByText(FREE_NOTE)).toBeInTheDocument();
    expect(screen.getByText(PAID_NOTE)).toBeInTheDocument();
  });

  it("draws the same notes while the catalog loads", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    const { container } = await renderPicker("free");
    await screen.findByText(PAID_NOTE);
    const skeleton = container.querySelector(
      "[data-instance-type-groups-skeleton]",
    );
    expect(skeleton).not.toBeNull();
    expect(skeleton).toHaveTextContent(FREE_NOTE);
    expect(skeleton).toHaveTextContent(PAID_NOTE);
  });
});
