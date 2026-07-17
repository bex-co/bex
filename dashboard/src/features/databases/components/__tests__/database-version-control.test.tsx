import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseVersionControl } from "@/features/databases/components/database-version-control";
import type { DatabaseDetailView } from "@/features/databases/types";

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
  });

  it("renders a backup guard refusal inline", async () => {
    mockError =
      "a completed physical backup is required before upgrading this durable Postgres instance";
    const user = userEvent.setup();
    render(<DatabaseVersionControl database={DATABASE} onChanged={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Upgrade" }));
    expect(screen.getByText("Upgrade blocked")).toBeInTheDocument();
    expect(screen.getByText(mockError)).toBeInTheDocument();
  });
});
