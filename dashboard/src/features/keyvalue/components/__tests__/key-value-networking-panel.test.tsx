import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";

const PHRASE = "sudo clear key value cache";
const state = vi.hoisted(() => ({
  externalAccessRestarts: true,
  save: vi.fn(async (..._args: unknown[]): Promise<unknown> => true),
}));

vi.mock("@/features/keyvalue/hooks/use-key-value-networking", () => ({
  useKeyValueNetworking: () => ({
    allowList: [],
    isPublic: false,
    externalAccessRestarts: state.externalAccessRestarts,
    savingAllowList: false,
    saveAllowList: state.save,
  }),
}));

const RESTART_HINT = /the first save restarts it/;
const UNCHANGED = /Internal access is unchanged/;
const RULE = [{ cidrBlock: "203.0.113.7/32", description: "" }];

async function addRuleAndSave(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    screen.getByRole("textbox", { name: "New CIDR block" }),
    "203.0.113.7/32",
  );
  await user.click(screen.getByRole("button", { name: "Add" }));
  await user.click(screen.getByRole("button", { name: "Save allowlist" }));
}

// w4/m233: a first enable restarted the store unannounced, and on a
// persistence-Off store emptied it, under a hint saying internal access was
// unchanged.
describe("Key Value networking: external access that restarts the store", () => {
  beforeEach(() => {
    state.externalAccessRestarts = true;
    state.save.mockReset();
    state.save.mockResolvedValue(true);
  });

  it("says the first save restarts the store, then confirms the data loss bex-api names", async () => {
    state.save
      .mockResolvedValueOnce({ confirm: PHRASE, name: "cache" })
      .mockResolvedValueOnce(true);
    const user = userEvent.setup();
    render(<KeyValueNetworkingPanel id="red-cache" />);
    expect(screen.getByText(RESTART_HINT)).toBeInTheDocument();
    expect(screen.queryByText(UNCHANGED)).toBeNull();

    await addRuleAndSave(user);
    expect(state.save).toHaveBeenNthCalledWith(1, RULE);
    const dialog = screen.getByRole("dialog", {
      name: "Delete all data and enable external access?",
    });
    const confirm = screen.getByRole("button", {
      name: "Delete data and save rules",
    });
    expect(confirm).toBeDisabled();
    await user.type(screen.getByRole("textbox", { name: "Sudo Command" }), PHRASE);
    await user.click(confirm);
    expect(state.save).toHaveBeenNthCalledWith(2, RULE, PHRASE);
    expect(dialog).not.toBeInTheDocument();
  });

  it("closing the dialog saves nothing and a later refusal starts with an empty phrase", async () => {
    state.save.mockResolvedValue({ confirm: PHRASE, name: "cache" });
    const user = userEvent.setup();
    render(<KeyValueNetworkingPanel id="red-cache" />);

    await addRuleAndSave(user);
    await user.type(screen.getByRole("textbox", { name: "Sudo Command" }), "sudo");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(state.save).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Save allowlist" }));
    expect(screen.getByRole("textbox", { name: "Sudo Command" })).toHaveValue("");
  });

  it("keeps the no-restart copy and saves directly when the listener already runs", async () => {
    state.externalAccessRestarts = false;
    const user = userEvent.setup();
    render(<KeyValueNetworkingPanel id="red-cache" />);
    expect(screen.getByText(UNCHANGED)).toBeInTheDocument();
    expect(screen.queryByText(RESTART_HINT)).toBeNull();

    await addRuleAndSave(user);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(state.save).toHaveBeenCalledExactlyOnceWith(RULE);
  });
});
