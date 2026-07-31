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
  render(
    <EditRegistryCredentialDialog entry={entry} onUpdated={() => {}} />,
  );
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
