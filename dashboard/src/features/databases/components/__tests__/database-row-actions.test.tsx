import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import type { DatabaseView } from "@/features/databases/types";

const remove = vi.fn();
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove, deleting: null }),
}));

// DatabaseRowActions renders a "Move to project" submenu via useMoveToProject,
// which needs an ApolloProvider + workspace context neither this render nor
// the component under test cares about — stub it to the empty-projects shape
// (the submenu renders nothing when there are no projects).
vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
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
};

beforeEach(() => {
  remove.mockReset();
  remove.mockResolvedValue(true);
});

describe("DatabaseRowActions", () => {
  it("gates delete behind a typed-name confirmation", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<DatabaseRowActions database={DB} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete database",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    const input = within(dialog).getByRole("textbox");
    await user.type(input, "wrong-name");
    expect(confirm).toBeDisabled();

    await user.clear(input);
    await user.type(input, "shop-db");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("shop-db", "shop-db");
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });
});
