import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { KeyValueDangerActions } from "@/features/keyvalue/components/key-value-danger-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { mockCapabilities } from "@/test/mocks/capabilities";
import {
  ADMIN_ONLY_REASON,
  ROLE_REASON,
  adminOnlyAs,
} from "@/test/mocks/admin-only";
import { lifecycleRows, lifecycleState } from "@/test/mocks/resource-actions";
import type { KeyValueView } from "@/features/keyvalue/types";

// Suspend and Resume read the store's own projection, and a fresh read of it
// right before dispatch (w4/m578).
const projection = vi.hoisted(() => ({ state: null as unknown }));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDatastoreActions: () => projection.state,
}));
const apolloQuery = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

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
  adminOnlyAs("admin");
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
  run.mockReset();
  run.mockResolvedValue({ status: "success" });
  projection.state = lifecycleState(KEY_VALUE.id, "allowed");
  apolloQuery.mockReset();
  apolloQuery.mockResolvedValue({
    data: { keyValueActions: lifecycleRows("allowed") },
  });
});

describe("KeyValueDangerActions — Render-parity bottom action row", () => {
  it("keeps can_operate lifecycle actions but disables can_create delete for a contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // Delete opens with can_create, so its decision is the role's refusal.
    adminOnlyAs("viewer");
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
    expect(await screen.findByRole("tooltip")).toHaveTextContent(ROLE_REASON);
    await user.click(removeButton);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  // w4/724: Render lets only an Admin delete a protected environment's Key
  // Value. A Developer used to type the phrase and read "Forbidden"; now the
  // button says why before the dialog opens.
  it("disables Delete for a Developer in a protected environment with Render's reason", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={vi.fn()}
      />,
    );
    expect(useAdminOnlyActions).toHaveBeenCalledWith("keyValue", KEY_VALUE.id);

    const removeButton = screen.getByRole("button", {
      name: "Delete Key Value Instance",
    });
    expect(removeButton).toBeDisabled();
    await user.hover(removeButton.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      ADMIN_ONLY_REASON,
    );
    await user.click(removeButton);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(remove).not.toHaveBeenCalled();
  });

  it("disables lifecycle actions with the role's reason for a viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "VIEWER",
        canCreate: false,
        canOperate: false,
      }),
    );
    projection.state = lifecycleState(KEY_VALUE.id, "viewer");
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
      "Your role can’t perform this action.",
    );
    await user.click(suspend);
    expect(run).not.toHaveBeenCalled();
  });

  // w4/m578: in a protected environment only an Admin may suspend or resume,
  // so a Developer reads Render's reason before the click and sends nothing.
  it.each([
    ["Suspend Key Value Instance", KEY_VALUE],
    ["Resume Key Value Instance", { ...KEY_VALUE, suspended: true }],
  ])(
    "disables %s with Render's admin-only reason in a protected environment",
    async (name, keyValue) => {
      projection.state = lifecycleState(KEY_VALUE.id, "adminOnly");
      const user = userEvent.setup();
      render(
        <KeyValueDangerActions
          keyValue={keyValue}
          onDeleted={vi.fn()}
          onChanged={vi.fn()}
        />,
      );

      const control = screen.getByRole("button", { name });
      expect(control).toBeDisabled();
      await user.hover(control.parentElement!);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "This action is limited to admins.",
      );
      await user.click(control);
      expect(screen.queryByRole("alertdialog")).toBeNull();
      expect(run).not.toHaveBeenCalled();
      expect(apolloQuery).not.toHaveBeenCalled();
    },
  );

  it("disables Suspend while its decision is still being checked", async () => {
    projection.state = { status: "checking", refresh: async () => undefined };
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
      "Checking whether you can perform this action…",
    );
  });

  it("sends no resume when the fresh check before dispatch refuses", async () => {
    apolloQuery.mockResolvedValue({
      data: { keyValueActions: lifecycleRows("adminOnly") },
    });
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={{ ...KEY_VALUE, suspended: true }}
        onDeleted={vi.fn()}
        onChanged={onChanged}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Resume Key Value Instance" }),
    );
    await waitFor(() => expect(apolloQuery).toHaveBeenCalledTimes(1));
    expect(apolloQuery.mock.calls[0][0]).toMatchObject({
      variables: { id: KEY_VALUE.id, ownerId: "tea-test" },
    });
    expect(run).not.toHaveBeenCalled();
    expect(onChanged).not.toHaveBeenCalled();
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

    const dialog = await screen.findByRole("alertdialog");
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
    // w4/247: an Off store loses its keys on suspend, so the warning ties
    // retention to the persistence mode instead of promising preservation.
    const detail = within(dialog).getByText(/suspend sessions-cache/);
    expect(detail).toHaveTextContent("Off loses all data");
    expect(detail).toHaveTextContent(
      "Snapshot only keeps data up to the last snapshot",
    );
    expect(detail).not.toHaveTextContent(/data is preserved/i);
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
    await waitFor(() => expect(onChanged).toHaveBeenCalledOnce());
    expect(run).toHaveBeenCalledOnce();
    expect(run).toHaveBeenCalledWith(
      "suspend",
      "red-sessions",
      "sessions-cache",
    );
  });

  it("Cancel on the suspend warning runs no lifecycle mutation (w4/247)", async () => {
    const user = userEvent.setup();
    render(
      <KeyValueDangerActions
        keyValue={KEY_VALUE}
        onDeleted={vi.fn()}
        onChanged={vi.fn()}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Suspend Key Value Instance" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(run).not.toHaveBeenCalled();
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

    await waitFor(() => expect(onChanged).toHaveBeenCalledOnce());
    expect(run).toHaveBeenCalledWith(
      "resume",
      "red-sessions",
      "sessions-cache",
    );
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
    await waitFor(() => expect(onChanged).toHaveBeenCalledOnce());
    expect(run).toHaveBeenLastCalledWith(
      "suspend",
      "red-sessions",
      "sessions-cache",
      "sudo suspend key value sessions-cache",
    );
  });

  it("disables Delete for a contributor without can_create", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    adminOnlyAs("viewer");
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
