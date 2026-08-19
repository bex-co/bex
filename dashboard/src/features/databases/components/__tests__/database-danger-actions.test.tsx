import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatabaseDangerActions } from "@/features/databases/components/database-danger-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
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

function lifecycleStub(
  run = vi.fn().mockResolvedValue({ status: "success" }),
): UseDatabaseLifecycleResult {
  return { pending: null, run };
}

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
});

describe("DatabaseDangerActions — detail-page bottom action row", () => {
  it("keeps can_operate lifecycle actions but disables can_create delete for a contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    const removeButton = screen.getByRole("button", {
      name: "Delete Database",
    });
    expect(removeButton).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Suspend Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();

    await user.hover(removeButton.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can’t make this change.",
    );
    await user.click(removeButton);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  it("disables lifecycle actions with the operate reason for a viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canCreate: false,
        canOperate: false,
      }),
    );
    const lifecycle = lifecycleStub();
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycle}
      />,
    );

    const suspend = screen.getByRole("button", { name: "Suspend Database" });
    expect(suspend).toBeDisabled();
    await user.hover(suspend.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can only view this service.",
    );
    await user.click(suspend);
    expect(lifecycle.run).not.toHaveBeenCalled();
  });

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

  it("gates delete behind the sudo type-to-confirm phrase", async () => {
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

    expect(
      within(dialog).getByText("sudo delete postgres shop-db"),
    ).toBeInTheDocument();
    const input = within(dialog).getByLabelText("Sudo Command");
    // The bare database name is deliberately insufficient.
    await user.type(input, "shop-db");
    expect(confirm).toBeDisabled();
    await user.clear(input);
    await user.type(input, "sudo delete postgres shop-db");
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

  it("prompts with the server-issued phrase when a protected suspend is blocked", async () => {
    const run = vi
      .fn()
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend database shop-db",
      })
      .mockResolvedValueOnce({ status: "success" });
    const user = userEvent.setup();
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub(run)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Suspend Database" }));
    const firstDialog = await screen.findByRole("dialog");
    await user.click(
      within(firstDialog).getByRole("button", { name: "Suspend" }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText("sudo suspend database shop-db"),
    ).toBeInTheDocument();
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo suspend database shop-db",
    );
    await user.click(
      within(protectedDialog).getByRole("button", { name: "Suspend" }),
    );
    expect(run).toHaveBeenLastCalledWith(
      "suspend",
      DB,
      "sudo suspend database shop-db",
    );
  });

  it("disables Delete for a contributor without can_create", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    render(
      <DatabaseDangerActions
        database={DB}
        onDeleted={vi.fn()}
        lifecycle={lifecycleStub()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Delete Database" }),
    ).toBeDisabled();
    // Restart/suspend are can_operate — a contributor keeps them.
    expect(
      screen.getByRole("button", { name: "Restart Database" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Suspend Database" }),
    ).toBeEnabled();
  });

  it("keeps Delete enabled for an admin", () => {
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
  });
});
