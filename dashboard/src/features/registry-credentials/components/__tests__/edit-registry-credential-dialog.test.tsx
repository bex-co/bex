import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditRegistryCredentialDialog } from "@/features/registry-credentials/components/edit-registry-credential-dialog";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

// Radix Dialog uses the Pointer Capture API jsdom doesn't implement.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

const detail = {
  credential: null as RegistryCredentialView | null,
  loading: false,
  error: false,
};
vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credential",
  () => ({
    useRegistryCredential: () => detail,
  }),
);

const update = vi.fn();
vi.mock(
  "@/features/registry-credentials/hooks/use-update-registry-credential",
  () => ({
    useUpdateRegistryCredential: () => ({ update, busy: false }),
  }),
);

const entry: RegistryCredentialView = {
  id: "rgc-1",
  name: "GHCR prod",
  host: "ghcr.io",
  username: "alice",
  expiresAt: null,
  status: "active",
  createdAt: null,
};

beforeEach(() => {
  detail.credential = {
    ...entry,
    name: "GHCR prod (detail)",
    username: "alice-detail",
  };
  detail.loading = false;
  detail.error = false;
  update.mockReset();
  update.mockResolvedValue(true);
});

async function openDialog() {
  const user = userEvent.setup();
  render(<EditRegistryCredentialDialog entry={entry} />);
  await user.click(screen.getByRole("button", { name: "Edit" }));
  return user;
}

describe("EditRegistryCredentialDialog", () => {
  it("prefills the editable fields from the detail read and shows host read-only", async () => {
    await openDialog();
    // Seeded from the registryCredential detail read, not the list row.
    expect(screen.getByDisplayValue("alice-detail")).toBeInTheDocument();
    expect(screen.getByDisplayValue("GHCR prod (detail)")).toBeInTheDocument();
    const host = screen.getByDisplayValue("ghcr.io");
    expect(host).toBeDisabled();
  });

  it("refuses an over-long or blank name inline (w21/036)", async () => {
    const user = await openDialog();
    const name = screen.getByDisplayValue("GHCR prod (detail)");
    name.removeAttribute("maxlength");
    const save = screen.getByRole("button", { name: "Save changes" });
    await user.clear(name);
    expect(save).toBeDisabled();
    await user.click(name);
    await user.paste("a".repeat(101));
    expect(save).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("up to 100 characters");
  });

  // w21/053: the username and token the server refuses are marked in place and
  // hold Save before any request (the host is immutable).
  it("marks a username over 253 bytes or with a control character and holds Save", async () => {
    const user = await openDialog();
    const username = screen.getByDisplayValue("alice-detail");
    const save = screen.getByRole("button", { name: "Save changes" });
    await user.clear(username);
    await user.click(username);
    await user.paste("u".repeat(300));
    expect(username).toHaveAttribute("aria-invalid", "true");
    expect(username).toHaveAccessibleDescription("At most 253 bytes");
    expect(save).toBeDisabled();

    await user.clear(username);
    await user.click(username);
    await user.paste("qa\tuser");
    expect(username).toHaveAccessibleDescription(
      "Must not contain control characters",
    );
    expect(save).toBeDisabled();
    expect(update).not.toHaveBeenCalled();

    await user.clear(username);
    await user.type(username, "alice");
    expect(username).not.toHaveAttribute("aria-invalid", "true");
    expect(save).toBeEnabled();
  });

  it("stops the name at 100 characters (w21/035)", async () => {
    await openDialog();
    expect(screen.getByDisplayValue("GHCR prod (detail)")).toHaveAttribute(
      "maxlength",
      "100",
    );
  });

  it("keeps the stored token when the token field is left blank", async () => {
    const user = await openDialog();
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(update).toHaveBeenCalledWith({
      id: "rgc-1",
      name: "GHCR prod (detail)",
      username: "alice-detail",
      authToken: undefined,
    });
  });

  it("rotates the token when a new value is entered", async () => {
    const user = await openDialog();
    await user.type(
      screen.getByPlaceholderText("Leave blank to keep the current token"),
      "ghp_rotated",
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(update).toHaveBeenCalledWith(
      expect.objectContaining({ id: "rgc-1", authToken: "ghp_rotated" }),
    );
  });
});
