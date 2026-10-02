import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabasePlanSection } from "@/features/databases/components/database-plan-section";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { DatabaseDetailView } from "@/features/databases/types";

const updatePlan = vi.fn();

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
