import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DatabaseNameRow } from "@/features/databases/components/database-name-row";
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

describe("DatabaseNameRow", () => {
  it("renders read-only until the pencil, then renames without touching the id", async () => {
    const user = userEvent.setup();
    const onRenamed = vi.fn();
    render(<DatabaseNameRow database={database} onRenamed={onRenamed} />);

    // Edit-in-place: the value lives in a visibly disabled input; no Save yet.
    const input = screen.getByLabelText("Name");
    expect(input).toHaveValue("old-name");
    expect(input).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Edit database name" }),
    );
    expect(input).toBeEnabled();
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(rename).toHaveBeenCalledWith("dpg-stable", "new-name");
    expect(onRenamed).toHaveBeenCalledOnce();
  });

  it("blocks an invalid name before it reaches the API", async () => {
    const user = userEvent.setup();
    render(<DatabaseNameRow database={database} onRenamed={vi.fn()} />);

    await user.click(
      screen.getByRole("button", { name: "Edit database name" }),
    );
    const input = screen.getByLabelText("Name");
    await user.clear(input);
    await user.type(input, "Bad Name");

    expect(screen.getByText(/lowercase letters/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(rename).not.toHaveBeenCalled();
  });
});
