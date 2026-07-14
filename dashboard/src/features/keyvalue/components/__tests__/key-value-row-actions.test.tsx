import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueRowActions } from "@/features/keyvalue/components/key-value-row-actions";
import type { KeyValueView } from "@/features/keyvalue/types";

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
  remove.mockReset();
  remove.mockResolvedValue(true);
});

describe("KeyValueRowActions", () => {
  it("gates delete behind a typed-name confirmation", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(<KeyValueRowActions keyValue={KV} onDeleted={onDeleted} />);

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Key Value store",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    const input = within(dialog).getByRole("textbox");
    await user.type(input, "wrong-name");
    expect(confirm).toBeDisabled();

    await user.clear(input);
    await user.type(input, "sessions-cache");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("sessions-cache", "sessions-cache");
    expect(onDeleted).toHaveBeenCalledWith("sessions-cache");
  });
});
