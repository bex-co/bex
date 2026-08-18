import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueRowActions } from "@/features/keyvalue/components/key-value-row-actions";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

const remove = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove, deleting: null }),
}));

// KeyValueRowActions renders a "Move to project" submenu via useMoveToProject,
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

const KV: KeyValueView = {
  id: "sessions-cache",
  name: "sessions-cache",
  status: "available",
  plan: "starter",
  version: "8",
  createdAt: null,
  externalHost: null,
  public: false,
  suspended: false,
};

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
});

describe("KeyValueRowActions", () => {
  it("keeps a denied delete focusable with a reason and suppresses selection", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={vi.fn()} />);

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

  it("gates delete behind Render's exact sudo confirmation", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Key Value Instance",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    const input = within(dialog).getByRole("textbox");
    await user.type(input, "wrong-name");
    expect(confirm).toBeDisabled();

    await user.clear(input);
    await user.type(input, "sudo delete key value sessions-cache");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("sessions-cache", "sessions-cache");
    expect(onDeleted).toHaveBeenCalledWith("sessions-cache");
  });

  it("retries a protected delete only with the server-issued phrase", async () => {
    remove
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo delete key value sessions-cache",
      })
      .mockResolvedValueOnce({ status: "success" });
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const localDialog = await screen.findByRole("dialog");
    await user.type(
      within(localDialog).getByRole("textbox"),
      "sudo delete key value sessions-cache",
    );
    await user.click(
      within(localDialog).getByRole("button", {
        name: "Delete Key Value Instance",
      }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo delete key value sessions-cache",
    );
    await user.click(
      within(protectedDialog).getByRole("button", {
        name: "Delete Key Value Instance",
      }),
    );
    expect(remove).toHaveBeenLastCalledWith(
      "sessions-cache",
      "sessions-cache",
      "sudo delete key value sessions-cache",
    );
    expect(onDeleted).toHaveBeenCalledWith("sessions-cache");
  });
});
