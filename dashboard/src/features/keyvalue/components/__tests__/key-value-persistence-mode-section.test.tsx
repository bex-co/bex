import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValuePersistenceModeSection } from "@/features/keyvalue/components/key-value-persistence-mode-section";

const save = vi.fn();
let hookState = {
  mode: "journal-snapshot",
  loading: false,
  saving: false,
  save,
};

vi.mock("@/features/keyvalue/hooks/use-set-key-value-persistence-mode", () => ({
  useSetKeyValuePersistenceMode: () => hookState,
}));

beforeEach(() => {
  save.mockReset();
  save.mockResolvedValue(true);
  hookState = {
    mode: "journal-snapshot",
    loading: false,
    saving: false,
    save,
  };
});

describe("KeyValuePersistenceModeSection", () => {
  it("is read-only until the pencil, then fires the mutation with the new mode", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValuePersistenceModeSection id="red-abc" />);

    const select = screen.getByRole("combobox", { name: "Persistence mode" });
    expect(select).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Edit persistence mode" }),
    );
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();

    await user.click(
      screen.getByRole("combobox", { name: "Persistence mode" }),
    );
    await user.click(screen.getByRole("option", { name: "Snapshot only" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(save).toHaveBeenCalledWith("snapshot");
  });
});
