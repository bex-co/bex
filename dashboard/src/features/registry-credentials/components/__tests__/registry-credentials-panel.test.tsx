import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RegistryCredentialsPanel } from "@/features/registry-credentials/components/registry-credentials-panel";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

const state: {
  credentials: RegistryCredentialView[];
  loading: boolean;
  error: Error | undefined;
} = { credentials: [], loading: false, error: undefined };
const refetch = vi.fn();
vi.mock("@/features/registry-credentials/hooks/use-registry-credentials", () => ({
  useRegistryCredentials: () => ({ ...state, refetch }),
}));

const remove = vi.fn();
vi.mock("@/features/registry-credentials/hooks/use-delete-registry-credential", () => ({
  useDeleteRegistryCredential: () => ({ remove, deleting: null }),
}));

vi.mock("@/features/registry-credentials/components/create-registry-credential-dialog", () => ({
  CreateRegistryCredentialDialog: () => null,
}));

function cred(overrides: Partial<RegistryCredentialView> = {}): RegistryCredentialView {
  return {
    id: "rgc-1",
    name: "GHCR prod",
    host: "ghcr.io",
    username: "alice",
    expiresAt: null,
    status: "active",
    createdAt: null,
    ...overrides,
  };
}

beforeEach(() => {
  state.credentials = [];
  state.loading = false;
  state.error = undefined;
  refetch.mockReset();
  remove.mockReset();
});

describe("RegistryCredentialsPanel", () => {
  it("lists the workspace's credentials", () => {
    state.credentials = [
      cred({ id: "rgc-1", name: "GHCR prod" }),
      cred({ id: "rgc-2", name: "docker.io", host: "docker.io" }),
    ];
    render(<RegistryCredentialsPanel />);

    expect(screen.getByText("GHCR prod")).toBeInTheDocument();
    expect(screen.getByText("docker.io")).toBeInTheDocument();
  });

  it("shows an empty state with no credentials", () => {
    render(<RegistryCredentialsPanel />);
    expect(screen.getByText("No registry credentials")).toBeInTheDocument();
  });

  it("shows a forbidden state and not the table when the query 403s", () => {
    state.error = new Error("forbidden");
    render(<RegistryCredentialsPanel />);
    expect(screen.getByText("Not authorized")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows a generic error state for any other failure", () => {
    state.error = new Error("boom");
    render(<RegistryCredentialsPanel />);
    expect(
      screen.getByText("Couldn't load registry credentials"),
    ).toBeInTheDocument();
  });

  it("shows an Expired badge for an expired credential and Never expires for one with none", () => {
    state.credentials = [
      cred({ id: "rgc-1", status: "expired", expiresAt: "2020-01-01T00:00:00Z" }),
      cred({ id: "rgc-2", status: "active", expiresAt: null }),
    ];
    render(<RegistryCredentialsPanel />);
    expect(screen.getByText("Expired")).toBeInTheDocument();
    expect(screen.getByText("Never expires")).toBeInTheDocument();
  });

  it("a failed delete does not refetch — the credential stays listed", async () => {
    state.credentials = [cred()];
    remove.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<RegistryCredentialsPanel />);

    await user.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Delete" })[0],
    );

    expect(remove).toHaveBeenCalledWith("rgc-1", "GHCR prod");
    expect(refetch).not.toHaveBeenCalled();
    expect(screen.getByText("GHCR prod")).toBeInTheDocument();
  });

  it("a successful delete refetches the list", async () => {
    state.credentials = [cred()];
    remove.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<RegistryCredentialsPanel />);

    await user.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Delete" })[0],
    );

    expect(refetch).toHaveBeenCalled();
  });
});
