import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SSHKeysPanel } from "@/features/ssh-keys/components/ssh-keys-panel";
import type { SSHKeyView } from "@/features/ssh-keys/types";

const create = vi.fn();
const remove = vi.fn();
const state: {
  keys: SSHKeyView[];
  loading: boolean;
  error: Error | undefined;
  busy: string | null;
} = { keys: [], loading: false, error: undefined, busy: null };

vi.mock("@/features/ssh-keys/hooks/use-ssh-keys", () => ({
  useSSHKeys: () => ({ ...state, create, remove }),
}));

// The panel reads the `returnTo` search param and navigates back after a save
// (w2/m66). A mutable object + spy stand in for the router.
const routerSearch: { returnTo?: string; addKey?: boolean } = {};
const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useSearch: () => routerSearch,
  useNavigate: () => navigate,
}));

async function addValidKey() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Add SSH key" }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Name"), "laptop");
  await user.type(
    within(dialog).getByLabelText("Public key"),
    "ssh-ed25519 AAAATEST public-comment",
  );
  await user.click(within(dialog).getByRole("button", { name: "Add key" }));
}

beforeEach(() => {
  state.keys = [];
  state.loading = false;
  state.error = undefined;
  state.busy = null;
  create.mockReset();
  remove.mockReset();
  navigate.mockReset();
  routerSearch.returnTo = undefined;
  routerSearch.addKey = undefined;
});

describe("SSHKeysPanel", () => {
  it("validates before submit and sends only public material", async () => {
    create.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<SSHKeysPanel />);

    await user.click(screen.getByRole("button", { name: "Add SSH key" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Add key" });
    await user.type(within(dialog).getByLabelText("Name"), "laptop");
    await user.type(
      within(dialog).getByLabelText("Public key"),
      "private key text",
    );
    expect(submit).toBeDisabled();
    expect(
      within(dialog).getByText(
        "Enter one supported OpenSSH public key (RSA must be at least 2048 bits).",
      ),
    ).toBeInTheDocument();

    await user.clear(within(dialog).getByLabelText("Public key"));
    await user.type(
      within(dialog).getByLabelText("Public key"),
      "ssh-ed25519 AAAATEST first{enter}ssh-ed25519 AAAASECOND second",
    );
    expect(submit).toBeDisabled();

    await user.clear(within(dialog).getByLabelText("Public key"));
    await user.type(
      within(dialog).getByLabelText("Public key"),
      "ssh-ed25519 AAAATEST public-comment",
    );
    await user.click(submit);
    expect(create).toHaveBeenCalledWith(
      "laptop",
      "ssh-ed25519 AAAATEST public-comment",
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("keeps the form open when server validation rejects the key", async () => {
    create.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<SSHKeysPanel />);

    await user.click(screen.getByRole("button", { name: "Add SSH key" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "duplicate");
    await user.type(
      within(dialog).getByLabelText("Public key"),
      "ssh-ed25519 AAAATEST public-comment",
    );
    await user.click(within(dialog).getByRole("button", { name: "Add key" }));

    expect(create).toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("renders fingerprints and confirms deletion", async () => {
    state.keys = [
      {
        id: "ssk-d5t5d4v8g3c73f5m9peg",
        name: "workstation",
        publicKey: "ssh-ed25519 AAAATEST",
        fingerprint: "SHA256:example",
        createdAt: "2026-07-14T12:00:00Z",
      },
    ];
    remove.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<SSHKeysPanel />);

    expect(screen.getByText("SHA256:example")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));
    expect(remove).toHaveBeenCalledWith(
      "ssk-d5t5d4v8g3c73f5m9peg",
      "workstation",
    );
  });

  it("opens the add dialog when deep-linked with ?addKey", async () => {
    routerSearch.addKey = true;
    render(<SSHKeysPanel />);
    // The form dialog is auto-opened so the CTA lands on the form, not the page.
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("round-trips back to a same-origin returnTo after saving a key", async () => {
    create.mockResolvedValue(true);
    routerSearch.returnTo = "/agents/ags-d9example0000000000";
    render(<SSHKeysPanel />);
    await addValidKey();
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        href: "/agents/ags-d9example0000000000",
      }),
    );
  });

  it("refuses an off-origin returnTo (open-redirect guard)", async () => {
    create.mockResolvedValue(true);
    routerSearch.returnTo = "https://evil.example.com/steal";
    render(<SSHKeysPanel />);
    await addValidKey();
    await waitFor(() => expect(navigate).toHaveBeenCalled());
    // safe-next.ts collapses any off-origin value to "/".
    expect(navigate).toHaveBeenCalledWith({ href: "/" });
  });
});
