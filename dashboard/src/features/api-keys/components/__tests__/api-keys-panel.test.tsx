import { describe, it, expect, vi, beforeEach } from "vitest";
import { tableHeaders } from "@/test/table";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { ApiKeysPanel } from "@/features/api-keys/components/api-keys-panel";
import { ApiKeysPanelSkeleton } from "@/features/api-keys/components/api-keys-panel-skeleton";
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

vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
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

  it("lists the workspace's keys with a Client ID column (w4/m8/t002, w4/m105)", () => {
    apiKeysState.keys = [
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
        permissions: { all: true, grants: [] },
      },
      {
        id: "key-2",
        name: "ci-bot",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
        permissions: { all: true, grants: [] },
      },
    ];
    render(<ApiKeysPanel />);

    expect(screen.getByText("deploy-agent")).toBeInTheDocument();
    expect(screen.getByText("ci-bot")).toBeInTheDocument();
    expect(screen.getByText("Client ID")).toBeInTheDocument();
    expect(screen.getByText("Permissions")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Change permissions for ci-bot" }),
    ).toBeInTheDocument();
    expect(screen.getByText("key-1")).toBeInTheDocument();
    expect(screen.getByText("key-2")).toBeInTheDocument();
  });

  it("shows an empty state with no keys", () => {
    render(<ApiKeysPanel />);
    expect(screen.getByText("No API keys")).toBeInTheDocument();
  });

  it("shows a forbidden state and not the table when the query 403s", () => {
    apiKeysState.error = codedGraphQLError("FORBIDDEN");
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
        permissions: { all: true, grants: [] },
      },
    ];
    revoke.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<ApiKeysPanel />);

    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
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
        permissions: { all: true, grants: [] },
      },
    ];
    revoke.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<ApiKeysPanel />);

    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

    expect(refetch).toHaveBeenCalled();
  });

  it("frames the ready table's header while the keys load (w4/686)", () => {
    apiKeysState.keys = [
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
        permissions: { all: true, grants: [] },
      },
    ];
    const ready = render(<ApiKeysPanel />);
    const readyHeaders = tableHeaders(ready.container);
    ready.unmount();

    apiKeysState.keys = [];
    apiKeysState.loading = true;
    const { container } = render(<ApiKeysPanel />);
    expect(tableHeaders(container)).toEqual(readyHeaders);
    const rows = container.querySelectorAll(
      "tbody tr[data-testid='panel-table-row-placeholder']",
    );
    expect(rows).toHaveLength(1);
    // One cell per column, the last the 36 px revoke slot.
    const cells = rows[0].querySelectorAll("td");
    expect(cells).toHaveLength(readyHeaders.length);
    expect(cells[cells.length - 1].firstElementChild).toHaveClass("size-9");
    // The Permissions column (w9/m496): its summary beside the 32 px gear.
    expect(readyHeaders[1]).toBe("Permissions");
    expect(
      cells[1].querySelector(
        "[data-testid='api-key-permissions-placeholder'] .size-8",
      ),
    ).not.toBeNull();
  });

  it("the route's pending frame draws the ready table's header and the panel's own loading row", () => {
    apiKeysState.keys = [
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: null,
        createdBy: null,
        lastUsedAt: null,
        permissions: { all: false, grants: [] },
      },
    ];
    const ready = render(<ApiKeysPanel />);
    const readyHeaders = tableHeaders(ready.container);
    ready.unmount();

    apiKeysState.keys = [];
    apiKeysState.loading = true;
    const loading = render(<ApiKeysPanel />);
    const loadingRow = loading.container.querySelector("tbody")?.innerHTML;
    loading.unmount();

    const { container } = render(<ApiKeysPanelSkeleton />);
    expect(tableHeaders(container)).toEqual(readyHeaders);
    expect(container.querySelector("tbody")?.innerHTML).toBe(loadingRow);
  });
});
