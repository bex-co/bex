import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DatabaseDiskAutoscalingControl } from "@/features/databases/components/database-disk-autoscaling-control";
import type { DatabaseDetailView } from "@/features/databases/types";

const updateDiskAutoscaling = vi.fn();
let instanceTypes = [
  { id: "free", maxStorageGB: 1, supportsDiskAutoscaling: false },
  { id: "basic-1gb", maxStorageGB: 250, supportsDiskAutoscaling: true },
];
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({ instanceTypes }),
}));
vi.mock(
  "@/features/databases/hooks/use-update-database-disk-autoscaling",
  () => ({
    useUpdateDatabaseDiskAutoscaling: () => ({
      updateDiskAutoscaling,
      busy: false,
    }),
  }),
);

const database: DatabaseDetailView = {
  id: "dpg-autoscale",
  name: "autoscale",
  status: "available",
  plan: "basic-1gb",
  version: "16",
  diskSizeGB: 10,
  diskAutoscalingEnabled: true,
  poolerEnabled: false,
  createdAt: "2026-07-15T00:00:00Z",
  public: false,
  suspended: "not_suspended",
  databaseName: "dpg_autoscale",
  databaseUser: "dpg_autoscale_user",
  highAvailabilityEnabled: false,
  readReplicas: [],
  externalHost: null,
  backupsEnabled: false,
  region: null,
};

beforeEach(() => {
  updateDiskAutoscaling.mockReset();
  updateDiskAutoscaling.mockResolvedValue(true);
  instanceTypes = [
    { id: "free", maxStorageGB: 1, supportsDiskAutoscaling: false },
    { id: "basic-1gb", maxStorageGB: 250, supportsDiskAutoscaling: true },
  ];
});

describe("DatabaseDiskAutoscalingControl", () => {
  it("shows the current size, cap, and enabled state beside the disk chart", () => {
    render(
      <DatabaseDiskAutoscalingControl
        database={database}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByText("10 GB current · 250 GB max")).toBeVisible();
    expect(
      screen.getByRole("switch", { name: "Disk autoscaling" }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it("round-trips a toggle through the shared mutation and refetches", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <DatabaseDiskAutoscalingControl
        database={database}
        onChanged={onChanged}
      />,
    );

    await user.click(screen.getByRole("switch", { name: "Disk autoscaling" }));

    expect(updateDiskAutoscaling).toHaveBeenCalledWith("dpg-autoscale", false);
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("prevents enabling autoscaling on a plan that does not support it", async () => {
    const user = userEvent.setup();
    render(
      <DatabaseDiskAutoscalingControl
        database={{
          ...database,
          plan: "free",
          diskSizeGB: 1,
          diskAutoscalingEnabled: false,
        }}
        onChanged={vi.fn()}
      />,
    );
    const toggle = screen.getByRole("switch", { name: "Disk autoscaling" });
    expect(toggle).toBeDisabled();
    expect(screen.getByText("1 GB current · 1 GB max")).toBeVisible();
    expect(toggle).toHaveAccessibleDescription(
      /does not support disk autoscaling/,
    );
    // The visible note IS the description — one copy, so a screen reader
    // does not hear it twice (w4/184).
    expect(
      screen.getAllByText(/does not support disk autoscaling/),
    ).toHaveLength(1);
    await user.click(toggle);
    expect(updateDiskAutoscaling).not.toHaveBeenCalled();
  });

  it("allows disabling a pre-existing unsupported setting", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <DatabaseDiskAutoscalingControl
        database={{ ...database, plan: "free" }}
        onChanged={onChanged}
      />,
    );
    const toggle = screen.getByRole("switch", { name: "Disk autoscaling" });
    expect(toggle).toBeEnabled();
    await user.click(toggle);
    expect(updateDiskAutoscaling).toHaveBeenCalledWith(database.id, false);
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("waits for plan metadata before enabling and does not invent a maximum", () => {
    instanceTypes = [];
    render(
      <DatabaseDiskAutoscalingControl
        database={{ ...database, diskAutoscalingEnabled: false }}
        onChanged={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("switch", { name: "Disk autoscaling" }),
    ).toBeDisabled();
    expect(screen.getByText("10 GB current")).toBeVisible();
    expect(screen.queryByText(/GB max/)).not.toBeInTheDocument();
  });

  it("keeps the current setting and skips refresh when the server refuses", async () => {
    updateDiskAutoscaling.mockResolvedValue(false);
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <DatabaseDiskAutoscalingControl
        database={database}
        onChanged={onChanged}
      />,
    );
    await user.click(screen.getByRole("switch", { name: "Disk autoscaling" }));
    expect(
      screen.getByRole("switch", { name: "Disk autoscaling" }),
    ).toHaveAttribute("aria-checked", "true");
    expect(onChanged).not.toHaveBeenCalled();
  });
});
