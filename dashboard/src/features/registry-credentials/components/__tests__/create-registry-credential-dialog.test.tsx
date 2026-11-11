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

  // w21/035: the name is a label held to 100 characters, like the server.
  it("stops the optional name at 100 characters", async () => {
    const user = userEvent.setup();
    render(<CreateRegistryCredentialDialog onCreated={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Add credential" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/^Display name/)).toHaveAttribute(
      "maxlength",
      "100",
    );
  });

  it("refuses a 101-character name inline but allows leaving it blank (w21/036)", async () => {
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
    const add = within(dialog).getByRole("button", { name: "Add" });
    expect(add).toBeEnabled(); // optional name left blank

    const name = within(dialog).getByLabelText(/^Display name/);
    name.removeAttribute("maxlength"); // the field caps typing; exercise the check
    await user.click(name);
    await user.paste("a".repeat(101));
    expect(add).toBeDisabled();
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "up to 100 characters",
    );
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

  // w21/053: what the server refuses in the host, username and token is marked
  // in the field, in the person's language, and holds Add before any request.
  describe("field rules (w21/053)", () => {
    async function openFilled() {
      const user = userEvent.setup();
      render(<CreateRegistryCredentialDialog onCreated={vi.fn()} />);
      await user.click(screen.getByRole("button", { name: "Add credential" }));
      const dialog = await screen.findByRole("dialog");
      const fill = async (label: string, value: string) => {
        const input = within(dialog).getByLabelText(label);
        await user.clear(input);
        await user.click(input);
        await user.paste(value);
        return input;
      };
      await fill("Registry host", "ghcr.io");
      await fill("Username", "alice");
      await fill("Password or access token", "hunter2");
      return {
        dialog,
        fill,
        add: within(dialog).getByRole("button", { name: "Add" }),
      };
    }

    it.each(["ghcr.io/qa-owner/qa-image", "my registry.io"])(
      "marks the host %j and holds Add",
      async (value) => {
        const { fill, add } = await openFilled();
        const host = await fill("Registry host", value);
        expect(host).toHaveAttribute("aria-invalid", "true");
        expect(host).toHaveAccessibleDescription(/without a path/);
        expect(add).toBeDisabled();
        expect(create).not.toHaveBeenCalled();
      },
    );

    it("still accepts a pasted URL the server canonicalizes", async () => {
      create.mockResolvedValue(true);
      const user = userEvent.setup();
      const { fill, add } = await openFilled();
      const host = await fill("Registry host", "https://GHCR.IO/");
      expect(host).not.toHaveAttribute("aria-invalid", "true");
      await user.click(add);
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ host: "https://GHCR.IO/" }),
      );
    });

    it("marks a 300-byte host, a username with a tab and a 300-byte username", async () => {
      const { fill, add } = await openFilled();
      const host = await fill("Registry host", `${"a".repeat(300)}.io`);
      expect(host).toHaveAccessibleDescription("At most 253 bytes");
      expect(add).toBeDisabled();
      await fill("Registry host", "ghcr.io");

      const username = await fill("Username", "qa\tuser");
      expect(username).toHaveAttribute("aria-invalid", "true");
      expect(username).toHaveAccessibleDescription(
        "Must not contain control characters",
      );
      expect(add).toBeDisabled();

      await fill("Username", "u".repeat(300));
      expect(username).toHaveAccessibleDescription("At most 253 bytes");
      expect(add).toBeDisabled();
      await fill("Username", "alice");
      expect(add).toBeEnabled();
    });
  });
});
