import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseVersionControl } from "@/features/databases/components/database-version-control";
import type { DatabaseDetailView } from "@/features/databases/types";
import {
  POSTGRES_VERSIONS,
  postgresUpgradeTargets,
} from "@/features/databases/lib/versions";

const updateVersion = vi.fn();
const clearError = vi.fn();
let mockError: string | null = null;

vi.mock("@/features/databases/hooks/use-update-database-version", () => ({
  useUpdateDatabaseVersion: () => ({
    updateVersion,
    clearError,
    busy: false,
    error: mockError,
  }),
}));

const DATABASE: DatabaseDetailView = {
  id: "orders-db",
  name: "orders-db",
  status: "available",
  plan: "basic-1gb",
  version: "16",
  diskSizeGB: 5,
  diskAutoscalingEnabled: false,
  poolerEnabled: false,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
  databaseName: "orders_db",
  databaseUser: "orders_db_user",
  highAvailabilityEnabled: false,
  readReplicas: [],
  externalHost: null,
  backupsEnabled: true,
  region: null,
};

beforeEach(() => {
  updateVersion.mockReset();
  updateVersion.mockResolvedValue(true);
  clearError.mockReset();
  mockError = null;
});

describe("DatabaseVersionControl", () => {
  it("offers only newer supported versions and warns about downtime", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <DatabaseVersionControl database={DATABASE} onChanged={onChanged} />,
    );

    expect(screen.getByText("PostgreSQL 16")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Upgrade" }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(
      screen.getByText(
        /database will be unavailable while the offline upgrade/i,
      ),
    ).toBeInTheDocument();
    const confirm = screen.getByRole("button", {
      name: "Upgrade to PostgreSQL 18",
    });
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    expect(updateVersion).toHaveBeenCalledWith("orders-db", "18");
    expect(onChanged).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders a backup guard refusal inline", async () => {
    mockError =
      "a completed physical backup is required before upgrading this durable Postgres instance";
    const user = userEvent.setup();
    render(<DatabaseVersionControl database={DATABASE} onChanged={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Upgrade" }));
    expect(screen.getByText("Upgrade blocked")).toBeInTheDocument();
    expect(screen.getByText(mockError)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("offers a PostgreSQL 13 database only the targets CNPG can upgrade to (w4/308)", async () => {
    const user = userEvent.setup();
    render(
      <DatabaseVersionControl
        database={{ ...DATABASE, version: "13" }}
        onChanged={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Upgrade" }));
    await user.click(screen.getByRole("combobox"));
    const options = screen
      .getAllByRole("option")
      .map((option) => option.textContent);
    expect(options).toEqual([
      "PostgreSQL 18",
      "PostgreSQL 17",
      "PostgreSQL 16",
    ]);
    await user.click(screen.getByRole("option", { name: "PostgreSQL 16" }));
    await user.click(
      screen.getByRole("button", { name: "Upgrade to PostgreSQL 16" }),
    );
    expect(updateVersion).toHaveBeenCalledWith("orders-db", "16");
  });
});

describe("postgresUpgradeTargets", () => {
  it("keeps upgrade targets at 16 or newer while creation keeps 13-18", () => {
    expect(postgresUpgradeTargets("13")).toEqual(["18", "17", "16"]);
    expect(postgresUpgradeTargets("15")).toEqual(["18", "17", "16"]);
    expect(postgresUpgradeTargets("16")).toEqual(["18", "17"]);
    expect(postgresUpgradeTargets("18")).toEqual([]);
    expect([...POSTGRES_VERSIONS]).toEqual([
      "18",
      "17",
      "16",
      "15",
      "14",
      "13",
    ]);
  });
});
