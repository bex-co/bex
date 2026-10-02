import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { HAPanel } from "@/features/databases/components/ha-panel";
import type { DatabaseDetailView } from "@/features/databases/types";

vi.mock("@apollo/client/react", () => ({
  useMutation: () => [vi.fn(), { loading: false }],
}));

vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [
      {
        id: "free",
        name: "Free",
        cpu: "100m",
        memory: "256Mi",
        storageGB: 1,
        supportsHighAvailability: false,
      },
      {
        id: "pro",
        name: "Pro",
        cpu: "1",
        memory: "4Gi",
        storageGB: 20,
        supportsHighAvailability: true,
      },
    ],
    loading: false,
    error: undefined,
  }),
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

describe("HAPanel plan requirement (w8/m43)", () => {
  it("names the 1-CPU requirement on a plan without HA", () => {
    render(<HAPanel database={DATABASE} refetch={vi.fn()} />);
    expect(
      screen.getByText(
        "High availability requires a plan with at least 1 CPU. This database's plan does not offer it.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps the neutral line on a plan that offers HA", () => {
    render(
      <HAPanel database={{ ...DATABASE, plan: "pro" }} refetch={vi.fn()} />,
    );
    expect(
      screen.getByText("High availability is not enabled for this database."),
    ).toBeInTheDocument();
  });
});
