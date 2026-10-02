import { render, screen, within } from "@testing-library/react";
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
  it("keeps failed writes open and refuses a confirmation after the saved mode changes", async () => {
    hookState.mode = "off";
    save.mockResolvedValue(false);
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const { rerender } = render(
      <KeyValuePersistenceModeSection id="red-abc" name="cache" />,
    );
    await user.click(
      screen.getByRole("button", { name: "Edit persistence mode" }),
    );
    await user.click(
      screen.getByRole("combobox", { name: "Persistence mode" }),
    );
    await user.click(screen.getByRole("option", { name: "Snapshot only" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    const dialog = screen.getByRole("alertdialog");
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo clear key value cache",
    );
    await user.click(
      within(dialog).getByRole("button", {
        name: "Delete data and change mode",
      }),
    );
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(save).toHaveBeenCalledExactlyOnceWith("snapshot");
    hookState.mode = "journal-snapshot";
    rerender(<KeyValuePersistenceModeSection id="red-abc" name="cache" />);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(save).toHaveBeenCalledTimes(1);
  });

  it("is read-only until the pencil, then fires the mutation with the new mode", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(
      <KeyValuePersistenceModeSection id="red-abc" name="sessions-cache" />,
    );

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

async function selectMode(
  user: ReturnType<typeof userEvent.setup>,
  label: string,
) {
  await user.click(
    screen.getByRole("button", { name: "Edit persistence mode" }),
  );
  await user.click(screen.getByRole("combobox", { name: "Persistence mode" }));
  await user.click(screen.getByRole("option", { name: label }));
  await user.click(screen.getByRole("button", { name: "Save changes" }));
}

it.each([
  ["journal-snapshot", "Off", "off"],
  ["snapshot", "Off", "off"],
  ["off", "Journal + Snapshot", "journal-snapshot"],
  ["off", "Snapshot only", "snapshot"],
])("requires the displayed phrase for %s → %s", async (mode, label, target) => {
  hookState.mode = mode;
  const user = userEvent.setup({ pointerEventsCheck: 0 });
  render(<KeyValuePersistenceModeSection id="red-abc" name="sessions-cache" />);
  await selectMode(user, label);
  const dialog = screen.getByRole("alertdialog");
  const confirm = within(dialog).getByRole("button", {
    name: "Delete data and change mode",
  });
  expect(confirm).toBeDisabled();
  expect(save).not.toHaveBeenCalled();
  const input = within(dialog).getByRole("textbox");
  await user.type(input, "sudo clear key value another-store");
  expect(confirm).toBeDisabled();
  await user.clear(input);
  await user.type(input, "sudo clear key value sessions-cache");
  await user.click(confirm);
  expect(save).toHaveBeenCalledExactlyOnceWith(target);
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

it("cancels destructive changes without saving and reopens unarmed", async () => {
  const user = userEvent.setup({ pointerEventsCheck: 0 });
  render(<KeyValuePersistenceModeSection id="red-abc" name="sessions-cache" />);
  await selectMode(user, "Off");
  const dialog = screen.getByRole("alertdialog");
  await user.type(
    within(dialog).getByRole("textbox"),
    "sudo clear key value sessions-cache",
  );
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(save).not.toHaveBeenCalled();
  expect(screen.queryByRole("alertdialog")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  expect(
    screen.getByRole("button", { name: "Delete data and change mode" }),
  ).toBeDisabled();
});

it("keeps a rejected destructive save open for retry", async () => {
  save.mockResolvedValueOnce(false).mockResolvedValueOnce(true);
  const user = userEvent.setup({ pointerEventsCheck: 0 });
  render(<KeyValuePersistenceModeSection id="red-abc" name="sessions-cache" />);
  await selectMode(user, "Off");
  await user.type(
    within(screen.getByRole("alertdialog")).getByRole("textbox"),
    "sudo clear key value sessions-cache",
  );
  await user.click(
    screen.getByRole("button", { name: "Delete data and change mode" }),
  );
  expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Delete data and change mode" }),
  );
  expect(save).toHaveBeenCalledTimes(2);
  expect(screen.queryByRole("alertdialog")).toBeNull();
});
