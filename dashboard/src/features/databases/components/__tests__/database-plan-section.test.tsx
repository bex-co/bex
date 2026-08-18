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
      { id: "free", name: "Free", cpu: "0.1", memory: "256Mi", storageGB: 1 },
      {
        id: "starter",
        name: "Starter",
        cpu: "0.5",
        memory: "1Gi",
        storageGB: 10,
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

  it("does not disable before capabilities are definitive", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canOperate: false, loading: true, loaded: false }),
    );
    render(<DatabasePlanSection database={DATABASE} onChanged={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Starter/ })).toBeEnabled();
  });
});
