import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateRegistryCredentialDialog } from "@/features/registry-credentials/components/create-registry-credential-dialog";

const create = vi.fn();
vi.mock(
  "@/features/registry-credentials/hooks/use-create-registry-credential",
  () => ({
    useCreateRegistryCredential: () => ({ create, busy: false }),
  }),
);

beforeEach(() => {
  create.mockReset();
});

describe("CreateRegistryCredentialDialog", () => {
  it("submits host/username/authToken/name and closes on success", async () => {
    create.mockResolvedValue(true);
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<CreateRegistryCredentialDialog onCreated={onCreated} />);

    await user.click(screen.getByRole("button", { name: "Add credential" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Registry host"), "ghcr.io");
    await user.type(within(dialog).getByLabelText("Username"), "alice");
    await user.type(
      within(dialog).getByLabelText("Password or access token"),
      "hunter2",
    );
    await user.click(within(dialog).getByRole("button", { name: "Add" }));

    expect(create).toHaveBeenCalledWith({
      host: "ghcr.io",
      username: "alice",
      authToken: "hunter2",
      name: "",
    });
    expect(onCreated).toHaveBeenCalled();
    // The dialog closes on success — no reveal step (the secret is
    // caller-supplied, there's nothing to show back).
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps submit disabled until host/username/authToken are all filled", async () => {
    const user = userEvent.setup();
    render(<CreateRegistryCredentialDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Add credential" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Add" });
    expect(submit).toBeDisabled();

    await user.type(within(dialog).getByLabelText("Registry host"), "ghcr.io");
    expect(submit).toBeDisabled();
    await user.type(within(dialog).getByLabelText("Username"), "alice");
    expect(submit).toBeDisabled();
    await user.type(
      within(dialog).getByLabelText("Password or access token"),
      "hunter2",
    );
    expect(submit).not.toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("a create failure keeps the dialog open with the entered values intact", async () => {
    create.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<CreateRegistryCredentialDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Add credential" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Registry host"), "ghcr.io");
    await user.type(within(dialog).getByLabelText("Username"), "alice");
    await user.type(
      within(dialog).getByLabelText("Password or access token"),
      "hunter2",
    );
    await user.click(within(dialog).getByRole("button", { name: "Add" }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Registry host")).toHaveValue(
      "ghcr.io",
    );
  });

  it("resets the form when cancelled and reopened", async () => {
    const user = userEvent.setup();
    render(<CreateRegistryCredentialDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Add credential" }));
    let dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Registry host"), "ghcr.io");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

    await user.click(screen.getByRole("button", { name: "Add credential" }));
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Registry host")).toHaveValue("");
  });
});
