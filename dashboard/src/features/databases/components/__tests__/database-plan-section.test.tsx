import { beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabasePlanSection } from "@/features/databases/components/database-plan-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
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

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  updatePlan.mockReset().mockResolvedValue(true);
  mutate.mockReset().mockResolvedValue({ data: {} });
});

describe("DatabasePlanSection", () => {
  it("lets a contributor change plan through can_operate", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<DatabasePlanSection database={DATABASE} onChanged={onChanged} />);

    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(updatePlan).toHaveBeenCalledWith("dpg-shop", "starter", "Starter");
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("disables plan controls for a viewer with the operate reason", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canOperate: false }),
    );
    const user = userEvent.setup();
    render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />);

    const starter = screen.getByRole("radio", { name: /Starter/ });
    expect(starter).toBeDisabled();
    expect(
      screen.getByText(/Your role can only view this service/),
    ).toBeInTheDocument();
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

  it("retains the selected plan when the server refuses the update", async () => {
    updatePlan.mockResolvedValue(false);
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<DatabasePlanSection database={DATABASE} onChanged={onChanged} />);
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(screen.getByRole("radio", { name: /Starter/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("fail-closes plan controls until capabilities are definitive", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canOperate: false, loading: true, loaded: false }),
    );
    render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeDisabled();
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

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-shop", diskSizeGB: 25 },
    });
    expect(onChanged).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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

  it("disables Edit for a member who cannot operate the database", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canOperate: false }),
    );
    render(<DatabasePlanSection database={STARTER} onChanged={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Edit storage" })).toBeDisabled();
  });

  // The pending plan card ends in the same Storage row: its label, row and
  // Edit slot match, and the size is a slot until the database is read.
  it("is previewed by the pending plan card's Storage row", () => {
    const storageRow = (card: Element) =>
      card.querySelector('[data-slot="card-content"]')!.lastElementChild!;
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

  it("shows the server's refusal in the dialog and keeps it open", async () => {
    mutate.mockRejectedValueOnce(
      uncodedGraphQLError(
        "bad request: Postgres storage is grow-only: requested 15 GB is below the allocated 20 GB",
      ),
    );
    const { user, dialog, save, onChanged } = await openStorage();
    await user.click(save);
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Postgres storage is grow-only: requested 15 GB is below the allocated 20 GB",
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });
});
