import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueDangerActions } from "@/features/keyvalue/components/key-value-danger-actions";
import type { KeyValueView } from "@/features/keyvalue/types";

const remove = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove, deleting: null }),
}));

const run = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-key-value-lifecycle", () => ({
  useKeyValueLifecycle: () => ({ pending: null, run }),
}));

const KEY_VALUE: KeyValueView = {
  id: "red-sessions",
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
  run.mockReset();
  run.mockResolvedValue(true);
});

describe("KeyValueDangerActions — Render-parity bottom action row", () => {
  it("renders the discoverable Delete and Suspend Key Value Instance actions", () => {
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Delete Key Value Instance" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    ).toBeEnabled();
  });

  it("gates delete behind Render's sudo phrase before deleting and leaving", async () => {
    const onDeleted = vi.fn();
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={onDeleted}
        onChanged={vi.fn()}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Delete Key Value Instance" }),
    );

    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByRole("heading", {
        name: "Delete Key Value Instance",
      }),
    ).toBeInTheDocument();
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Key Value Instance",
    });
    // The exact phrase is body copy; the input itself is labeled "Sudo Command".
    expect(
      within(dialog).getByText("sudo delete key value sessions-cache"),
    ).toBeInTheDocument();
    const input = within(dialog).getByLabelText("Sudo Command");
    expect(confirm).toBeDisabled();

    await user.type(input, "sudo delete key value sessions-cache");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("red-sessions", "sessions-cache");
    expect(onDeleted).toHaveBeenCalledWith("red-sessions");
  });

  it("requires Render's sudo phrase before suspending", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={onChanged}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    );

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByRole("heading", {
        name: "Suspend Key Value Instance",
      }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(/suspend sessions-cache/),
    ).toBeInTheDocument();
    const confirm = within(dialog).getByRole("button", {
      name: "Suspend Key Value Instance",
    });
    expect(
      within(dialog).getByText("sudo suspend key value sessions-cache"),
    ).toBeInTheDocument();
    const input = within(dialog).getByLabelText("Sudo Command");
    expect(confirm).toBeDisabled();

    await user.type(input, "sudo suspend key value sessions-cache");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(run).toHaveBeenCalledWith(
      "suspend",
      "red-sessions",
      "sessions-cache",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("swaps Suspend for an immediate Resume action while suspended", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={{ ...KEY_VALUE, suspended: true }}
        onDeleted={vi.fn()}
        onChanged={onChanged}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Suspend Key Value Instance" }),
    ).not.toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Resume Key Value Instance" }),
    );

    expect(run).toHaveBeenCalledWith(
      "resume",
      "red-sessions",
      "sessions-cache",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });
});
