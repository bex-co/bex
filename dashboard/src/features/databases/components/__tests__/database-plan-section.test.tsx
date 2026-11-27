import { beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabasePlanSection } from "@/features/databases/components/database-plan-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { CREATE_REASON, mockCapabilities } from "@/test/mocks/capabilities";
import { uncodedGraphQLError } from "@/test/mocks/apollo";
import { shape } from "@/test/card-shape";
import { DatastorePlanPreview } from "@/common/components/datastore-plan-preview";
import type { DatabaseDetailView } from "@/features/databases/types";

const updatePlan = vi.fn();
const mutate = vi.hoisted(() => vi.fn());

// Only the storage edit writes through Apollo here; the plan change goes
// through the mocked useUpdateDatabasePlan.
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
}));

vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [
      {
        id: "free",
        name: "Free",
        cpu: "0.1",
        memory: "256Mi",
        storageGB: 1,
        maxStorageGB: 1,
        supportsDiskAutoscaling: false,
        supportsConnectionPooling: false,
        maxReadReplicas: 0,
        supportsHighAvailability: false,
        monthlyUsd: "0.00",
        storageMonthlyUsdPerGB: "0.00",
      },
      {
        id: "starter",
        name: "Starter",
        cpu: "0.5",
        memory: "1Gi",
        storageGB: 10,
        maxStorageGB: 16384,
        supportsDiskAutoscaling: true,
        supportsConnectionPooling: true,
        maxReadReplicas: 0,
        supportsHighAvailability: false,
        monthlyUsd: "7.00",
        storageMonthlyUsdPerGB: "0.21",
      },
      {
        id: "pro",
        name: "Pro",
        cpu: "1",
        memory: "4Gi",
        storageGB: 20,
        maxStorageGB: 16384,
        supportsDiskAutoscaling: true,
        supportsConnectionPooling: true,
        maxReadReplicas: 5,
        supportsHighAvailability: true,
        monthlyUsd: "55.00",
        storageMonthlyUsdPerGB: "0.21",
      },
      {
        id: "accelerated",
        name: "Accelerated",
        cpu: "2",
        memory: "8Gi",
        storageGB: 20,
        maxStorageGB: 16384,
        supportsDiskAutoscaling: true,
        supportsConnectionPooling: true,
        maxReadReplicas: 5,
        supportsHighAvailability: true,
        monthlyUsd: "110.00",
        storageMonthlyUsdPerGB: "0.21",
      },
    ],
    loading: false,
    error: undefined,
  }),
}));

vi.mock("@/features/databases/hooks/use-update-database-plan", () => ({
  useUpdateDatabasePlan: () => ({ updatePlan, busy: false }),
}));

const DATABASE: DatabaseDetailView = {
  id: "dpg-shop",
  name: "shop",
  status: "available",
  plan: "free",
  version: "18",
  diskSizeGB: 1,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
  databaseName: "shop",
  databaseUser: "shop_user",
  highAvailabilityEnabled: false,
  diskAutoscalingEnabled: false,
  poolerEnabled: false,
  readReplicas: [],
  externalHost: null,
  backupsEnabled: false,
};

/** A plan review's lines as [label, figures, price], read off the dialog. */
function reviewLines(dialog: HTMLElement) {
  return [...dialog.querySelectorAll('[data-slot="review-line"]')].map((row) =>
    [...row.children].map((cell) => cell.textContent),
  );
}

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  updatePlan.mockReset().mockResolvedValue({ ok: true });
  mutate.mockReset().mockResolvedValue({ data: {} });
});

describe("DatabasePlanSection", () => {
  it("lets a developer change plan (can_create)", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<DatabasePlanSection database={DATABASE} onChanged={onChanged} />);

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Update compute" }),
    );

    expect(updatePlan).toHaveBeenCalledWith("dpg-shop", "starter", "Starter");
    expect(onChanged).toHaveBeenCalledOnce();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  // Render gives a Contributor no configuration change, a plan included
  // (w9/m619): the plan is can_create, so a Contributor, who keeps
  // can_operate, reads the role's reason on held controls.
  it("disables plan controls for a contributor with the create reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />);

    const starter = screen.getByRole("radio", { name: /Starter/ });
    expect(starter).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByText(CREATE_REASON)).toBeInTheDocument();
    await user.click(starter);
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("blocks a move to a plan without HA while HA is on (w8/m43)", async () => {
    const user = userEvent.setup();
    render(
      <DatabasePlanSection
        database={{ ...DATABASE, plan: "pro", highAvailabilityEnabled: true }}
        onChanged={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Starter does not support high availability (it requires at least 1 CPU). Disable high availability first.",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    // Cancel still resets the selection.
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  // w2/m171: bex-api checks the spec, so a standby still starting already
  // blocks the move — not only one the operator reports ready.
  it("blocks the move while requested HA is still starting", async () => {
    const user = userEvent.setup();
    render(
      <DatabasePlanSection
        database={{
          ...DATABASE,
          plan: "pro",
          highAvailabilityEnabled: false,
          highAvailabilityRequested: true,
        }}
        onChanged={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Disable high availability first.",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("lets a database without HA move to any plan", async () => {
    const user = userEvent.setup();
    render(
      <DatabasePlanSection
        database={{ ...DATABASE, plan: "pro" }}
        onChanged={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it.each([
    {
      changes: { diskSizeGB: 10 },
      message:
        "Free allows up to 1 GB of storage. This database has 10 GB, which cannot be reduced.",
    },
    {
      changes: { readReplicas: [{ name: "reader", connectionInfo: null }] },
      message: "Read replica limit for Free: 0. This database has 1.",
    },
    {
      changes: { diskAutoscalingEnabled: true },
      message:
        "Free does not support disk autoscaling. Turn off disk autoscaling first.",
    },
    {
      changes: { poolerEnabled: true },
      message:
        "Free does not support connection pooling. Disable the pooler first.",
    },
  ])(
    "explains an incompatible downgrade: $message",
    async ({ changes, message }) => {
      const user = userEvent.setup();
      render(
        <DatabasePlanSection
          database={{ ...DATABASE, plan: "pro", ...changes }}
          onChanged={vi.fn()}
        />,
      );
      await user.click(screen.getByRole("radio", { name: /Free/ }));
      expect(screen.getByRole("alert")).toHaveTextContent(message);
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
      expect(updatePlan).not.toHaveBeenCalled();
      await user.click(screen.getByRole("button", { name: "Cancel" }));
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    },
  );

  it("allows supported replica and pooler settings on a compatible target plan", async () => {
    const user = userEvent.setup();
    render(
      <DatabasePlanSection
        database={{
          ...DATABASE,
          plan: "starter",
          diskSizeGB: 10,
          readReplicas: Array.from({ length: 5 }, (_, index) => ({
            name: `reader-${index}`,
            connectionInfo: null,
          })),
          poolerEnabled: true,
          diskAutoscalingEnabled: true,
        }}
        onChanged={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("blocks a target plan when the database exceeds its replica limit", async () => {
    const user = userEvent.setup();
    render(
      <DatabasePlanSection
        database={{
          ...DATABASE,
          plan: "starter",
          diskSizeGB: 10,
          readReplicas: Array.from({ length: 6 }, (_, index) => ({
            name: `reader-${index}`,
            connectionInfo: null,
          })),
        }}
        onChanged={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Read replica limit for Pro: 5. This database has 6.",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("keeps the review open with the server's refusal, and the plan selected", async () => {
    updatePlan.mockResolvedValue({
      ok: false,
      error: "Payment method required.",
    });
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<DatabasePlanSection database={DATABASE} onChanged={onChanged} />);
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Update compute" }),
    );
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Payment method required.",
    );
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("radio", { name: /Starter/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("fail-closes plan controls until capabilities are definitive", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canCreate: false, loading: true, loaded: false }),
    );
    render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeDisabled();
  });
});

// w3/m674: Render's review before a compute change applies. Each billed line
// with its monthly price, the total, Render's downtime warning and its button.
describe("DatabasePlanSection compute review", () => {
  const HA_WITH_REPLICA: DatabaseDetailView = {
    ...DATABASE,
    plan: "pro",
    diskSizeGB: 25,
    highAvailabilityEnabled: true,
    highAvailabilityRequested: true,
    readReplicas: [{ name: "reader", connectionInfo: null }],
  };

  async function review(database: DatabaseDetailView, plan: RegExp) {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <>
        <DatabasePlanSection database={database} onChanged={onChanged} />
        <section id="high-availability">
          <button type="button" role="switch" aria-checked="true">
            High Availability
          </button>
        </section>
      </>,
    );
    await user.click(screen.getByRole("radio", { name: plan }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    return { user, dialog, onChanged };
  }

  it("prices the plan, the standby, the replicas and their storage, then totals them", async () => {
    const { user, dialog } = await review(HA_WITH_REPLICA, /Accelerated/);
    expect(dialog).toHaveAccessibleName("Apply changes?");
    expect(dialog).toHaveTextContent(
      "Review your compute and storage selections.",
    );
    expect(reviewLines(dialog)).toEqual([
      ["Compute plan", "2 CPU8 GB RAM", "$110.00/month"],
      ["High Availability", "Enabled", "$110.00/month"],
      ["Read replicas", "1 replica", "$110.00/month"],
      // 25 GB on the primary, the standby and the replica at $0.21.
      ["Storage", "25 GB (x3 instances)", "$15.75/month"],
    ]);
    expect(dialog).toHaveTextContent("Total$345.75/month");
    expect(dialog).toHaveTextContent(
      "Your database and its replicas will be unavailable for a few seconds while updating.",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Update compute" }),
    );
    expect(updatePlan).toHaveBeenCalledWith(
      "dpg-shop",
      "accelerated",
      "Accelerated",
    );
  });

  it("lists one instance, and the storage a larger plan floor raises, without HA", async () => {
    const { dialog } = await review(
      { ...DATABASE, plan: "starter", diskSizeGB: 10 },
      /Pro/,
    );
    expect(reviewLines(dialog)).toEqual([
      ["Compute plan", "1 CPU4 GB RAM", "$55.00/month"],
      // Pro's 20 GB floor is above the database's 10 GB.
      ["Storage", "20 GB", "$4.20/month"],
    ]);
    expect(dialog).toHaveTextContent("Total$59.20/month");
    expect(dialog).toHaveTextContent(
      "Your database will be unavailable for a few minutes while updating.",
    );
  });

  it("lists no storage on Free, whose storage is included", async () => {
    const { dialog } = await review(
      { ...DATABASE, plan: "starter", diskSizeGB: 1 },
      /Free/,
    );
    expect(reviewLines(dialog)).toEqual([
      ["Compute plan", "0.1 CPU256 MB RAM", "$0.00/month"],
    ]);
    expect(dialog).toHaveTextContent("Total$0.00/month");
  });

  it("returns to the picker with the choice kept from the plan's Edit", async () => {
    const { user, dialog } = await review(HA_WITH_REPLICA, /Accelerated/);
    await user.click(
      within(dialog).getByRole("button", { name: "Edit compute plan" }),
    );
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Accelerated/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("drops the change and goes to High Availability from its Edit", async () => {
    const scroll = vi.mocked(Element.prototype.scrollIntoView);
    scroll.mockClear();
    const { user, dialog } = await review(HA_WITH_REPLICA, /Accelerated/);
    await user.click(
      within(dialog).getByRole("button", { name: "Edit high availability" }),
    );
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(scroll.mock.contexts).toContain(
      document.getElementById("high-availability"),
    );
    // As on Render, the plan goes back to the database's own.
    expect(screen.getByRole("radio", { name: /Pro/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("applies nothing on Cancel", async () => {
    const { user, dialog, onChanged } = await review(
      HA_WITH_REPLICA,
      /Accelerated/,
    );
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(updatePlan).not.toHaveBeenCalled();
    expect(onChanged).not.toHaveBeenCalled();
  });
});

// w2/m307: Render's Plan page Storage section — any multiple of 5 GB above the
// current size, up to the plan's maximum; never smaller; fixed on Free.
describe("DatabasePlanSection storage", () => {
  const STARTER: DatabaseDetailView = {
    ...DATABASE,
    plan: "starter",
    diskSizeGB: 10,
  };

  async function openStorage(database: DatabaseDetailView = STARTER) {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<DatabasePlanSection database={database} onChanged={onChanged} />);
    await user.click(screen.getByRole("button", { name: "Edit storage" }));
    const dialog = await screen.findByRole("dialog");
    const field = within(dialog).getByRole("spinbutton", {
      name: "Storage (GB)",
    });
    const save = within(dialog).getByRole("button", { name: "Save" });
    async function enter(value: string) {
      await user.clear(field);
      await user.type(field, value);
    }
    return { user, dialog, field, save, enter, onChanged };
  }

  it("shows the allocated size and grows it", async () => {
    render(<DatabasePlanSection database={STARTER} onChanged={vi.fn()} />);
    expect(screen.getByText("Storage")).toBeInTheDocument();
    expect(screen.getByText("10 GB")).toBeInTheDocument();
    cleanup();

    const { user, dialog, field, save, enter, onChanged } = await openStorage();
    expect(dialog).toHaveTextContent(
      "Enter any multiple of 5 GB greater than the current 10 GB. Storage can't be reduced once it grows.",
    );
    // Prefilled with the smallest size it accepts.
    expect(field).toHaveValue(15);
    await enter("25");
    expect(save).toBeEnabled();
    await user.click(save);

    // Save opens Render's storage review; nothing applies before it is
    // confirmed (w3/m674).
    const review = await screen.findByRole("alertdialog");
    expect(mutate).not.toHaveBeenCalled();
    expect(review).toHaveTextContent("Review your storage selection.");
    expect(reviewLines(review)).toEqual([
      ["Compute plan", "0.5 CPU1 GB RAM", "$7.00/month"],
      ["Storage", "25 GB", "$5.25/month"],
    ]);
    expect(review).toHaveTextContent("Total$12.25/month");
    // Render warns of no downtime for a storage change.
    expect(review).not.toHaveTextContent("unavailable");
    await user.click(
      within(review).getByRole("button", { name: "Update storage" }),
    );

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-shop", diskSizeGB: 25 },
    });
    expect(onChanged).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("prices the new size on every instance the database bills", async () => {
    const { user, save, enter } = await openStorage({
      ...STARTER,
      plan: "pro",
      diskSizeGB: 20,
      highAvailabilityEnabled: true,
      highAvailabilityRequested: true,
      readReplicas: [{ name: "reader", connectionInfo: null }],
    });
    await enter("25");
    await user.click(save);
    const review = await screen.findByRole("alertdialog");
    expect(reviewLines(review)).toEqual([
      ["Compute plan", "1 CPU4 GB RAM", "$55.00/month"],
      ["High Availability", "Enabled", "$55.00/month"],
      ["Read replicas", "1 replica", "$55.00/month"],
      ["Storage", "25 GB (x3 instances)", "$15.75/month"],
    ]);
    expect(review).toHaveTextContent("Total$180.75/month");
  });

  it.each(["Edit storage", "Cancel"])(
    "returns to the field with the size kept from the review's %s",
    async (button) => {
      const { user, save, enter } = await openStorage();
      await enter("25");
      await user.click(save);
      const review = await screen.findByRole("alertdialog");
      await user.click(within(review).getByRole("button", { name: button }));
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      const dialog = await screen.findByRole("dialog");
      expect(
        within(dialog).getByRole("spinbutton", { name: "Storage (GB)" }),
      ).toHaveValue(25);
      expect(mutate).not.toHaveBeenCalled();
    },
  );

  // Render's almost-full warning links `/plan#storage` (w3/m656).
  it("is Render's #storage anchor", () => {
    render(<DatabasePlanSection database={STARTER} onChanged={vi.fn()} />);
    const anchor = document.getElementById("storage");
    expect(anchor).toHaveTextContent(/^Storage10 GB/);
    expect(
      within(anchor!).getByRole("button", { name: "Edit storage" }),
    ).toBeInTheDocument();
  });

  it("rejects a size that is not a multiple of 5 GB", async () => {
    const { dialog, save, enter } = await openStorage();
    await enter("12");
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Enter a whole multiple of 5 GB.",
    );
    expect(save).toBeDisabled();
    expect(mutate).not.toHaveBeenCalled();
  });

  it.each(["10", "5"])(
    "rejects %s GB, which is not above the current 10 GB",
    async (value) => {
      const { dialog, save, enter } = await openStorage();
      await enter(value);
      expect(within(dialog).getByRole("alert")).toHaveTextContent(
        "Enter more than the current 10 GB. Storage can't be reduced.",
      );
      expect(save).toBeDisabled();
    },
  );

  it("rejects a size above the plan's maximum", async () => {
    const { dialog, save, enter } = await openStorage();
    await enter("16390");
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Starter allows up to 16384 GB of storage.",
    );
    expect(save).toBeDisabled();
  });

  it("keeps a free database's storage fixed, and says why", () => {
    render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Edit storage" })).toBeDisabled();
    expect(
      screen.getByText(
        "Free databases have fixed storage. Upgrade to a paid plan to add storage.",
      ),
    ).toBeInTheDocument();
  });

  // Until the catalog names the database's plan, whether its disk is fixed
  // (Free) is unknown, so Edit waits rather than offering a refused write.
  it("disables Edit while the database's plan is not in the catalog", () => {
    render(
      <DatabasePlanSection
        database={{ ...STARTER, plan: "not-in-catalog" }}
        onChanged={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Edit storage" })).toBeDisabled();
  });

  it("disables Edit for a contributor, who cannot change settings (w9/m619)", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    render(<DatabasePlanSection database={STARTER} onChanged={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Edit storage" })).toBeDisabled();
  });

  // The pending plan card has the same Storage row: its label, row and
  // Edit slot match, and the size is a slot until the database is read.
  it("is previewed by the pending plan card's Storage row", () => {
    const storageRow = (card: Element) =>
      card.querySelector('[data-slot="card-content"]')!.lastElementChild!
        .previousElementSibling!;
    const pending = storageRow(
      render(<DatastorePlanPreview kind="postgres" />).container
        .firstElementChild!,
    );
    const ready = storageRow(
      render(<DatabasePlanSection database={STARTER} onChanged={vi.fn()} />)
        .container.firstElementChild!,
    );
    expect(pending.className).toBe(ready.className);
    const [pendingText, pendingEdit] = [...pending.children];
    const [readyText, readyEdit] = [...ready.children];
    expect(pendingText.className).toBe(readyText.className);
    expect(shape(pendingText.firstElementChild!)).toBe(
      shape(readyText.firstElementChild!),
    );
    expect(pendingText.children[1].className).toContain("h-5");
    expect(pendingEdit.className).toBe(readyEdit.className);
    expect(pendingEdit.textContent).toBe(readyEdit.textContent);
    expect((pendingEdit as HTMLButtonElement).disabled).toBe(true);
    expect(pending.textContent).not.toContain("GB");
  });

  // Render's Plan page puts Storage Autoscaling under Storage (w3/m641): the
  // ready card ends in its switch row, and the pending card ends in the same
  // row, its label real and its size line and switch slots.
  it("ends in the Storage Autoscaling row, previewed by the pending card", () => {
    const lastRow = (card: Element) =>
      card.querySelector('[data-slot="card-content"]')!.lastElementChild!;
    const pending = lastRow(
      render(<DatastorePlanPreview kind="postgres" />).container
        .firstElementChild!,
    );
    const ready = lastRow(
      render(<DatabasePlanSection database={STARTER} onChanged={vi.fn()} />)
        .container.firstElementChild!,
    );
    expect(
      within(ready as HTMLElement).getByRole("switch"),
    ).toHaveAccessibleName("Disk autoscaling");
    expect(pending.className).toBe(ready.className);
    const [pendingText, pendingSwitch] = [...pending.children];
    const [readyText, readySwitch] = [...ready.children];
    expect(pendingText.className).toBe(readyText.className);
    expect(pendingText.firstElementChild!.textContent).toBe(
      readyText.firstElementChild!.textContent,
    );
    expect(pendingText.children[1].className).toContain("h-5");
    // The switch's 20 × 36 box.
    expect(pendingSwitch.className).toContain("h-5 w-9");
    expect(
      readySwitch.querySelector('[role="switch"]') ?? readySwitch,
    ).toHaveClass("h-5", "w-9");
    expect(pending.textContent).not.toContain("GB");
  });

  it("shows the server's refusal in the review and keeps it open", async () => {
    mutate.mockRejectedValueOnce(
      uncodedGraphQLError(
        "bad request: Postgres storage is grow-only: requested 15 GB is below the allocated 20 GB",
      ),
    );
    const { user, save, onChanged } = await openStorage();
    await user.click(save);
    const review = await screen.findByRole("alertdialog");
    await user.click(
      within(review).getByRole("button", { name: "Update storage" }),
    );
    expect(within(review).getByRole("alert")).toHaveTextContent(
      "Postgres storage is grow-only: requested 15 GB is below the allocated 20 GB",
    );
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });
});
