import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueDangerActions } from "@/features/keyvalue/components/key-value-danger-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
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
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
  run.mockReset();
  run.mockResolvedValue({ status: "success" });
});

describe("KeyValueDangerActions — Render-parity bottom action row", () => {
  it("keeps can_operate lifecycle actions but disables can_create delete for a contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    const removeButton = screen.getByRole("button", {
      name: "Delete Key Value Instance",
    });
    expect(removeButton).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
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
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    const suspend = screen.getByRole("button", {
      name: "Suspend Key Value Instance",
    });
    expect(suspend).toBeDisabled();
    await user.hover(suspend.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can only view this service.",
    );
    await user.click(suspend);
    expect(run).not.toHaveBeenCalled();
  });

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

  it("prompts with the server-issued phrase when a protected suspend is blocked", async () => {
    run
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend key value sessions-cache",
      })
      .mockResolvedValueOnce({ status: "success" });
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
    const localDialog = await screen.findByRole("alertdialog");
    await user.type(
      within(localDialog).getByLabelText("Sudo Command"),
      "sudo suspend key value sessions-cache",
    );
    await user.click(
      within(localDialog).getByRole("button", {
        name: "Suspend Key Value Instance",
      }),
    );

    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText(
        "sudo suspend key value sessions-cache",
      ),
    ).toBeInTheDocument();
    await user.type(
      within(protectedDialog).getByRole("textbox"),
      "sudo suspend key value sessions-cache",
    );
    await user.click(
      within(protectedDialog).getByRole("button", {
        name: "Suspend Key Value Instance",
      }),
    );
    expect(run).toHaveBeenLastCalledWith(
      "suspend",
      "red-sessions",
      "sessions-cache",
      "sudo suspend key value sessions-cache",
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("disables Delete for a contributor without can_create", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Delete Key Value Instance" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    ).toBeEnabled();
  });

  it("keeps Delete enabled for an admin", () => {
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
  });
});
