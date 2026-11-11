import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";

const save = vi.fn();
let hookState = {
  policy: "allkeys-lru",
  loading: false,
  saving: false,
  save,
};

vi.mock("@/features/keyvalue/hooks/use-set-key-value-maxmemory-policy", () => ({
  useSetKeyValueMaxmemoryPolicy: () => hookState,
}));
let persistenceMode = "journal-snapshot";
vi.mock("@/features/keyvalue/hooks/use-set-key-value-persistence-mode", () => ({
  useSetKeyValuePersistenceMode: () => ({ mode: persistenceMode }),
}));

beforeEach(() => {
  save.mockReset();
  save.mockResolvedValue(true);
  hookState = { policy: "allkeys-lru", loading: false, saving: false, save };
  persistenceMode = "journal-snapshot";
});

async function chooseAndSave(
  user: ReturnType<typeof userEvent.setup>,
  policy: string,
) {
  await user.click(
    screen.getByRole("button", { name: "Edit maxmemory policy" }),
  );
  await user.click(screen.getByRole("combobox", { name: "Maxmemory policy" }));
  await user.click(screen.getByRole("option", { name: policy }));
  await user.click(screen.getByRole("button", { name: "Save changes" }));
}

describe("KeyValueMaxmemoryPolicySection", () => {
  it("is read-only until the pencil, then fires the mutation with the new policy", async () => {
    // pointerEventsCheck off: Radix Select styles the trigger/options in ways
    // userEvent's pointer-events guard trips over in jsdom.
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" />);

    // Edit-in-place: the current policy sits in a disabled select, no Save yet.
    const select = screen.getByRole("combobox", { name: "Maxmemory policy" });
    expect(select).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    );
    // Editable now; Save disabled until the policy actually changes.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();

    await user.click(
      screen.getByRole("combobox", { name: "Maxmemory policy" }),
    );
    await user.click(screen.getByRole("option", { name: "volatile-lru" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    // A persisted store confirms the restart, with no phrase.
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "Change the maxmemory policy to volatile-lru?",
    );
    expect(within(dialog).queryByRole("textbox")).toBeNull();
    expect(save).not.toHaveBeenCalled();
    await user.click(
      within(dialog).getByRole("button", {
        name: "Restart and change policy",
      }),
    );

    expect(save).toHaveBeenCalledWith("volatile-lru");
  });

  it("cancel restores the current policy and leaves edit mode without saving", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" />);

    await user.click(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    );
    await user.click(
      screen.getByRole("combobox", { name: "Maxmemory policy" }),
    );
    await user.click(screen.getByRole("option", { name: "noeviction" }));
    expect(screen.getByRole("button", { name: "Save changes" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    // Back to read-only: the pencil returns and there is no Save button.
    expect(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();
    expect(save).not.toHaveBeenCalled();
  });

  // w4/m316: the restart replaces the only pod, and Persistence Off reloads
  // nothing, so every key goes.
  it("asks a Persistence Off store for the typed phrase, and Cancel saves nothing", async () => {
    persistenceMode = "off";
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" name="cache" />);
    await chooseAndSave(user, "noeviction");

    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "Persistence is Off, so the restart deletes every key in cache.",
    );
    const confirm = within(dialog).getByRole("button", {
      name: "Delete data and change policy",
    });
    expect(confirm).toBeDisabled();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(save).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Save changes" }));
    const reopened = screen.getByRole("alertdialog");
    await user.type(
      within(reopened).getByRole("textbox"),
      "sudo clear key value cache",
    );
    await user.click(
      within(reopened).getByRole("button", {
        name: "Delete data and change policy",
      }),
    );
    expect(save).toHaveBeenCalledWith("noeviction");
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("still asks for the phrase when the persistence mode is unknown, and says so", async () => {
    persistenceMode = "";
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-abc" name="cache" />);
    await chooseAndSave(user, "noeviction");
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent("persistence mode is unknown");
    expect(dialog).not.toHaveTextContent("Persistence is Off");
    expect(within(dialog).getByRole("textbox")).toBeInTheDocument();
  });

  it("saves a suspended store's policy without asking: nothing is running", async () => {
    persistenceMode = "off";
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(
      <KeyValueMaxmemoryPolicySection id="red-abc" name="cache" suspended />,
    );
    await chooseAndSave(user, "noeviction");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(save).toHaveBeenCalledWith("noeviction");
  });
});
