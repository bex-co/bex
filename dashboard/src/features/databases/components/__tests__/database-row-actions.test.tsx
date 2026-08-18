import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import type { DatabaseView } from "@/features/databases/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

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
  suspended: "not_suspended",
};

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
});

describe("DatabaseRowActions", () => {
  it("keeps a denied delete focusable with a reason and suppresses selection", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(<DatabaseRowActions database={DB} onDeleted={vi.fn()} />);

    await user.tab();
    expect(
      screen.getByRole("button", { name: "Open actions menu" }),
    ).toHaveFocus();
    await user.keyboard("{Enter}{ArrowDown}");
    const removeItem = await screen.findByRole("menuitem", { name: "Delete" });
    expect(removeItem).toHaveAttribute("aria-disabled", "true");
    expect(removeItem).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can’t make this change.",
    );

    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  it("uses can_operate for lifecycle menu items and suppresses viewer selection", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canCreate: false,
        canOperate: false,
      }),
    );
    const run = vi.fn().mockResolvedValue({ status: "success" });
    const user = userEvent.setup();
    render(
      <DatabaseRowActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={{ pending: null, run }}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    const suspend = await screen.findByRole("menuitem", { name: "Suspend" });
    const restart = screen.getByRole("menuitem", { name: "Restart" });
    expect(suspend).toHaveAttribute("aria-disabled", "true");
    expect(restart).toHaveAttribute("aria-disabled", "true");

    await user.click(suspend);
    expect(run).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("gates delete behind the sudo type-to-confirm phrase", async () => {
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
    await user.type(input, "sudo delete postgres shop-db");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("shop-db", "shop-db");
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });

  it("retries a protected delete only with the server-issued phrase", async () => {
    remove
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo delete database shop-db",
      })
      .mockResolvedValueOnce({ status: "success" });
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<DatabaseRowActions database={DB} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const localDialog = await screen.findByRole("dialog");
    await user.type(
      within(localDialog).getByRole("textbox"),
      "sudo delete postgres shop-db",
    );
    await user.click(
      within(localDialog).getByRole("button", { name: "Delete database" }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText("sudo delete database shop-db"),
    ).toBeInTheDocument();
    const retry = within(protectedDialog).getByRole("button", {
      name: "Delete database",
    });
    expect(retry).toBeDisabled();
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo delete database shop-db",
    );
    await user.click(retry);

    expect(remove).toHaveBeenLastCalledWith(
      "shop-db",
      "shop-db",
      "sudo delete database shop-db",
    );
    expect(onDeleted).toHaveBeenCalledWith("shop-db");
  });
});
