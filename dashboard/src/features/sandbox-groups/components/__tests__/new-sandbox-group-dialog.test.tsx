import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { NewSandboxGroupDialog } from "@/features/sandbox-groups/components/new-sandbox-group-dialog";

// The real hook runs; only its two collaborators are stubbed, so these tests
// cover the dialog and the code-to-inline-error mapping together.
const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-owner" }),
}));

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

const GROUP = {
  __typename: "SandboxGroup",
  id: "sbg-created",
  name: "ci",
  region: "frankfurt",
  isDefault: true,
  concurrencyLimit: 1000,
};

const onOpenChange = vi.fn();

beforeEach(() => {
  mutate.mockReset();
  copy.mockReset();
  onOpenChange.mockReset();
});

/** The dialog open, like the overview's `?new=sandbox-group`. */
function renderOpen() {
  return render(<NewSandboxGroupDialog open onOpenChange={onOpenChange} />);
}

/** Owns `open` the way the overview's URL does, for reopen behaviour. */
function Harness() {
  const [open, setOpen] = useState(true);
  return (
    <>
      <button onClick={() => setOpen(true)}>reopen</button>
      <NewSandboxGroupDialog open={open} onOpenChange={setOpen} />
    </>
  );
}

describe("NewSandboxGroupDialog", () => {
  it("asks for a name only: no region field (bex-api places the group) and no environment picker", () => {
    renderOpen();
    const dialog = screen.getByRole("dialog", { name: "New Sandbox Group" });

    expect(within(dialog).getByLabelText("Name")).toHaveValue("");
    expect(within(dialog).queryByLabelText("Region")).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/oregon/i)).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/environment/i)).not.toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    ).toBeDisabled();
  });

  it("creates the group and confirms it in place: id, name, region, default, concurrency limit", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText("Name"), "  ci ");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    // bex-api refuses a padded name, so the dialog sends it trimmed.
    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "ci", ownerId: "tea-owner" },
    });
    expect(
      await within(dialog).findByText("Sandbox group created"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("sbg-created")).toBeInTheDocument();
    expect(within(dialog).getByText("ci", { selector: "span" })).toBeVisible();
    // The region bex-api reports, as service settings show theirs.
    expect(within(dialog).getByText("frankfurt")).toBeVisible();
    expect(within(dialog).getByText("Yes")).toBeVisible();
    expect(within(dialog).getByText("1000")).toBeVisible();
    // The form is gone, and nothing on the confirmation links anywhere.
    expect(within(dialog).queryByLabelText("Name")).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("link")).not.toBeInTheDocument();

    await user.click(
      within(dialog).getByRole("button", { name: "Copy sandbox group ID" }),
    );
    expect(copy).toHaveBeenCalledWith("sbg-created");

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("marks a group that is not the workspace default", async () => {
    mutate.mockResolvedValue({
      data: { createSandboxGroup: { ...GROUP, isDefault: false } },
    });
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText("Name"), "ci");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    expect(await within(dialog).findByText("No")).toBeVisible();
  });

  it("shows a duplicate name inline under the field and keeps the form", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("SANDBOX_GROUP_NAME_TAKEN", { field: "name" }),
    );
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");
    const name = within(dialog).getByLabelText("Name");

    await user.type(name, "ci");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    const error = await within(dialog).findByRole("alert");
    expect(error).toHaveTextContent(
      'A sandbox group named "ci" already exists in this workspace.',
    );
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription(
      expect.stringContaining("already exists"),
    );
    expect(name).toHaveValue("ci");
    expect(within(dialog).queryByText("Sandbox group created")).toBeNull();

    // Editing the name withdraws the stale refusal.
    await user.type(name, "2");
    expect(within(dialog).queryByRole("alert")).not.toBeInTheDocument();
    expect(name).toHaveAttribute("aria-invalid", "false");
  });

  it("shows the 10-group limit as an inline alert and keeps the form", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("sandbox_group_limit_reached", { limit: 10 }),
    );
    const user = userEvent.setup();
    renderOpen();
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText("Name"), "eleventh");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    );

    const alert = await within(dialog).findByRole("alert");
    expect(alert).toHaveTextContent("Sandbox group limit reached");
    expect(alert).toHaveTextContent(
      "This workspace already has the maximum of 10 sandbox groups.",
    );
    expect(within(dialog).getByLabelText("Name")).toHaveValue("eleventh");
    expect(
      within(dialog).getByRole("button", { name: "Create Sandbox Group" }),
    ).toBeEnabled();
  });

  it("submits on Enter", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const user = userEvent.setup();
    renderOpen();

    await user.type(screen.getByLabelText("Name"), "ci{Enter}");

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Sandbox group created")).toBeVisible();
  });

  it("does not submit a blank or whitespace-only name", async () => {
    const user = userEvent.setup();
    renderOpen();

    await user.type(screen.getByLabelText("Name"), "   {Enter}");

    expect(mutate).not.toHaveBeenCalled();
  });

  it("cancels through onOpenChange", async () => {
    const user = userEvent.setup();
    renderOpen();

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("holds the dialog open while a create is in flight", async () => {
    let finish: (value: unknown) => void = () => {};
    mutate.mockReturnValue(new Promise((resolve) => (finish = resolve)));
    const user = userEvent.setup();
    renderOpen();

    await user.type(screen.getByLabelText("Name"), "ci");
    await user.click(
      screen.getByRole("button", { name: "Create Sandbox Group" }),
    );
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(onOpenChange).not.toHaveBeenCalled();

    finish({ data: { createSandboxGroup: GROUP } });
    expect(await screen.findByText("Sandbox group created")).toBeVisible();
  });

  it("reopens on a clean form, never on the last confirmation", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const user = userEvent.setup();
    render(<Harness />);

    await user.type(screen.getByLabelText("Name"), "ci");
    await user.click(
      screen.getByRole("button", { name: "Create Sandbox Group" }),
    );
    await user.click(await screen.findByRole("button", { name: "Done" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "reopen" }));
    expect(await screen.findByLabelText("Name")).toHaveValue("");
    expect(screen.queryByText("Sandbox group created")).not.toBeInTheDocument();
  });
});
