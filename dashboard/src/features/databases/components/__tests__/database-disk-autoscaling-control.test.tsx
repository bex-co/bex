import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  DatabaseDiskAutoscalingControl,
  DISK_AUTOSCALING_CAP_GB,
} from "@/features/databases/components/database-disk-autoscaling-control";
import type { DatabaseDetailView } from "@/features/databases/types";

const updateDiskAutoscaling = vi.fn();
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
  plan: "free",
  version: "16",
  diskSizeGB: 10,
  diskAutoscalingEnabled: true,
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
});

describe("DatabaseDiskAutoscalingControl", () => {
  it("matches the shared Go operator/backend cap contract", () => {
    const catalog = readFileSync(
      resolve(process.cwd(), "../lego/types/tiers/tiers.yaml"),
      "utf8",
    );
    const match = catalog.match(/^\s*diskAutoscalingCapGB:\s*(\d+)\s*$/m);

    expect(match, "tiers.yaml must declare diskAutoscalingCapGB").not.toBeNull();
    expect(DISK_AUTOSCALING_CAP_GB).toBe(Number(match?.[1]));
  });

  it("shows the current size, cap, and enabled state beside the disk chart", () => {
    render(
      <DatabaseDiskAutoscalingControl
        database={database}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByText("10 GB current · 16384 GB max")).toBeVisible();
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
});
