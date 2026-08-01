import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiKeysPanel } from "@/features/api-keys/components/api-keys-panel";
import type { ApiKeyView } from "@/features/api-keys/types";

const apiKeysState: {
  keys: ApiKeyView[];
  loading: boolean;
  error: Error | undefined;
} = { keys: [], loading: false, error: undefined };
const refetch = vi.fn();
vi.mock("@/features/api-keys/hooks/use-api-keys", () => ({
  useApiKeys: () => ({ ...apiKeysState, refetch }),
}));

const revoke = vi.fn();
vi.mock("@/features/api-keys/hooks/use-revoke-api-key", () => ({
  useRevokeApiKey: () => ({ revoke, revoking: null }),
}));

vi.mock("@/features/api-keys/components/create-api-key-dialog", () => ({
  CreateApiKeyDialog: () => null,
}));

beforeEach(() => {
  apiKeysState.keys = [];
  apiKeysState.loading = false;
  apiKeysState.error = undefined;
  refetch.mockReset();
  revoke.mockReset();
});

describe("ApiKeysPanel", () => {
  it("links API key setup to the CLI guide", () => {
    render(<ApiKeysPanel />);

    expect(
      screen.getByRole("link", { name: "Set up the CLI." }),
    ).toHaveAttribute("href", "https://bex.co/docs/cli");
  });

  it("lists the workspace's keys (w4/m8/t002)", () => {
    apiKeysState.keys = [
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
      },
      {
        id: "key-2",
        name: "ci-bot",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
      },
    ];
    render(<ApiKeysPanel />);

    expect(screen.getByText("deploy-agent")).toBeInTheDocument();
    expect(screen.getByText("ci-bot")).toBeInTheDocument();
  });

  it("shows an empty state with no keys", () => {
    render(<ApiKeysPanel />);
    expect(screen.getByText("No API keys")).toBeInTheDocument();
  });

  it("shows a forbidden state and not the table when the query 403s", () => {
    apiKeysState.error = new Error("forbidden");
    render(<ApiKeysPanel />);
    expect(screen.getByText("Not authorized")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows a generic error state for any other failure", () => {
    apiKeysState.error = new Error("boom");
    render(<ApiKeysPanel />);
    expect(screen.getByText("Couldn't load API keys")).toBeInTheDocument();
  });

  it("a failed revoke does not refetch — the key stays listed (t006)", async () => {
    apiKeysState.keys = [
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
      },
    ];
    revoke.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<ApiKeysPanel />);

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

    expect(revoke).toHaveBeenCalledWith("key-1", "deploy-agent");
    expect(refetch).not.toHaveBeenCalled();
    expect(screen.getByText("deploy-agent")).toBeInTheDocument();
  });

  it("a successful revoke refetches the list", async () => {
    apiKeysState.keys = [
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
      },
    ];
    revoke.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<ApiKeysPanel />);

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

    expect(refetch).toHaveBeenCalled();
  });
});
