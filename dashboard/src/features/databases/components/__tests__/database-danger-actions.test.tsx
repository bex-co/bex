import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseDangerActions } from "@/features/databases/components/database-danger-actions";
import type { UseDatabaseLifecycleResult } from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";

const remove = vi.fn();
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove, deleting: null }),
}));

const DB: DatabaseView = {
  id: "shop-db",
  name: "shop-db",
  status: "available",
  plan: "free",
  version: "18",
  diskSizeGB: 1,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
};

function lifecycleStub(): UseDatabaseLifecycleResult {
  return { pending: null, run: vi.fn().mockResolvedValue(undefined) };
}

beforeEach(() => {
  remove.mockReset();
  remove.mockResolvedValue(true);
});

describe("DatabaseDangerActions — detail-page bottom action row", () => {
  it("renders Delete / Restart / Suspend Database buttons", () => {
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Delete Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Suspend Database" }),
    ).toBeEnabled();
  });

  it("gates delete behind a typed-name confirmation", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={onDeleted}
        lifecycle={lifecycleStub()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Delete Database" }));
    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete database",
    });
    expect(confirm).toBeDisabled();

    await user.type(within(dialog).getByRole("textbox"), "shop-db");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("shop-db", "shop-db");
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });

  it("confirms suspend and restart through a dialog before running the verb", async () => {
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Suspend Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Suspend" }));
    expect(lifecycle.run).toHaveBeenCalledWith("suspend", DB);

    await user.click(screen.getByRole("button", { name: "Restart Database" }));
    const restartDialog = await screen.findByRole("dialog");
    await user.click(
      within(restartDialog).getByRole("button", { name: "Restart" }),
    );
    expect(lifecycle.run).toHaveBeenCalledWith("restart", DB);
  });

  it("swaps Suspend for Resume (immediate, no confirm) while suspended", async () => {
    const lifecycle = lifecycleStub();
    const suspended = { ...DB, suspended: "suspended" };
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={suspended}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Suspend Database" }),
    ).toBeNull();
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Resume Database" }));
    expect(lifecycle.run).toHaveBeenCalledWith("resume", suspended);
  });
});
