import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DatabaseNameSection } from "@/features/databases/components/database-name-section";
import type { DatabaseDetailView } from "@/features/databases/types";

const rename = vi.fn();
vi.mock("@/features/databases/hooks/use-rename-database", () => ({
  useRenameDatabase: () => ({ rename, busy: false }),
}));

const database: DatabaseDetailView = {
  id: "dpg-stable",
  name: "old-name",
  status: "available",
  plan: "free",
  version: "16",
  diskSizeGB: 1,
  createdAt: "2026-07-14T00:00:00Z",
  public: false,
  suspended: "not_suspended",
  databaseName: "dpg_stable",
  databaseUser: "dpg_stable_user",
  highAvailabilityEnabled: false,
  readReplicas: [],
  externalHost: null,
  region: null,
};

beforeEach(() => {
  rename.mockReset();
  rename.mockResolvedValue(true);
});

describe("DatabaseNameSection", () => {
  it("keeps the stable id while submitting a new display name", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(<DatabaseNameSection database={database} onChanged={onChanged} />);

    const input = screen.getByLabelText("Name");
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(screen.getByRole("button", { name: "Save name" }));

    expect(rename).toHaveBeenCalledWith("dpg-stable", "new-name");
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("blocks an invalid name before it reaches the API", async () => {
    const user = userEvent.setup();
    render(<DatabaseNameSection database={database} onChanged={vi.fn()} />);

    const input = screen.getByLabelText("Name");
    await user.clear(input);
    await user.type(input, "Bad Name");

    expect(screen.getByText(/lowercase letters/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
    expect(rename).not.toHaveBeenCalled();
  });
});
