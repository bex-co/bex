import { describe, it, expect, vi, beforeEach } from "vitest";
import { tableHeaders } from "@/test/table";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import { ApiKeysPanel } from "@/features/api-keys/components/api-keys-panel";
import { ApiKeysPanelSkeleton } from "@/features/api-keys/components/api-keys-panel-skeleton";
import type { ApiKeyView } from "@/features/api-keys/types";

const apiKeysState: {
  keys: ApiKeyView[];
  loading: boolean;
  error: Error | undefined;
} = { keys: [], loading: false, error: undefined };
const refetch = vi.fn();
// Each render's allMembers argument, newest last (w9/m539).
const listCalls: boolean[] = [];
const allMembersKeys: { keys: ApiKeyView[] } = { keys: [] };
vi.mock("@/features/api-keys/hooks/use-api-keys", () => ({
  useApiKeys: (allMembers = false) => {
    listCalls.push(allMembers);
    return allMembers
      ? { ...apiKeysState, keys: allMembersKeys.keys, refetch }
      : { ...apiKeysState, refetch };
  },
}));

// The caller's role as the capability check last answered: null is unknown.
let admin: boolean | null = null;
vi.mock("@/features/api-keys/hooks/use-workspace-admin", () => ({
  useWorkspaceAdmin: () => admin,
}));

vi.mock("@/common/hooks/use-self-subjects", () => ({
  useSelfSubjects: () => ({ subjects: new Set(["admin-1"]), resolving: false }),
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
  allMembersKeys.keys = [];
  listCalls.length = 0;
  admin = null;
  refetch.mockReset();
  revoke.mockReset();
});

/** The card body's markup, Radix's per-mount ids blanked. */
function cardContent(container: HTMLElement): string | undefined {
  return container
    .querySelector("[data-slot='card-content']")
    ?.innerHTML.replace(/radix-[^"]*?-trigger/g, "radix-trigger");
}

function key(id: string, name: string, createdBy: string | null): ApiKeyView {
  return {
    id,
    name,
    createdAt: null,
    createdBy,
    lastUsedAt: null,
    permissions: { all: true, grants: [] },
  };
}

describe("ApiKeysPanel", () => {
  // Search's API Keys shortcut sends `/settings#api-keys`, Render's anchor
  // for this card (w4/711): the fragment must land on the card itself, clear
  // of the sticky section bar.
  it("anchors settings#api-keys on the card", () => {
    render(<ApiKeysPanel />);

    const target = document.getElementById("api-keys");
    expect(target).not.toBeNull();
    expect(within(target!).getByText("API Keys")).toBeInTheDocument();
    expect(target).toHaveClass(...SECTION_NAVIGATION_TARGET_CLASS.split(" "));
  });

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

  it("tells every role who sees which keys, in the panel and in its skeleton alike (w9/m539)", () => {
    const description =
      "You see and manage the keys you created; an Admin can also view and revoke every member's keys.";
    const ready = render(<ApiKeysPanel />);
    expect(ready.container).toHaveTextContent(description);
    ready.unmount();
    const { container } = render(<ApiKeysPanelSkeleton />);
    expect(container).toHaveTextContent(description);
  });

  it("lists the keys a member below Developer minted, each with its gear and revoke (w9/m514)", () => {
    // bex-api answers a Viewer with their own keys only; the panel shows them
    // as it shows a Developer's workspace-wide list, with no forbidden state.
    apiKeysState.keys = [
      {
        id: "key-mine",
        name: "status-page",
        createdAt: null,
        createdBy: "viewer-1",
        lastUsedAt: null,
        permissions: {
          all: false,
          grants: [{ permission: "services", level: "read" }],
        },
      },
    ];
    render(<ApiKeysPanel />);

    expect(screen.queryByText("Not authorized")).not.toBeInTheDocument();
    expect(screen.getByText("status-page")).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Change permissions for status-page",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Revoke API key status-page" }),
    ).toBeInTheDocument();
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

  it("shows a member who is not an Admin only their own keys, with no switch (w9/m539)", () => {
    admin = false;
    apiKeysState.keys = [key("key-mine", "ci", "dev-1")];
    render(<ApiKeysPanel />);

    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(listCalls.every((all) => !all)).toBe(true);
    expect(screen.getByText("ci")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Change permissions for ci" }),
    ).toBeInTheDocument();
  });

  it("holds the Admin's switch, disabled, while the role is unknown", () => {
    render(<ApiKeysPanel />);
    const tabs = within(screen.getByRole("tablist"));
    expect(tabs.getByRole("tab", { name: "Your keys" })).toBeDisabled();
    expect(tabs.getByRole("tab", { name: "All members' keys" })).toBeDisabled();
    expect(listCalls.every((all) => !all)).toBe(true);
  });

  it("lets an Admin switch to every member's keys: each names its creator, only their own carries the gear, and another's revoke names its creator (w9/m539)", async () => {
    admin = true;
    apiKeysState.keys = [key("key-own", "mine", "admin-1")];
    allMembersKeys.keys = [
      key("key-own", "mine", "admin-1"),
      key("key-dev", "dev-ci", "dev-1"),
      key("key-legacy", "legacy", null),
    ];
    revoke.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<ApiKeysPanel />);

    // The default is the Admin's own keys, as on Render.
    expect(screen.getByRole("tab", { name: "Your keys" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.queryByText("dev-ci")).not.toBeInTheDocument();
    expect(listCalls.at(-1)).toBe(false);

    await user.click(screen.getByRole("tab", { name: "All members' keys" }));
    expect(listCalls.at(-1)).toBe(true);
    expect(screen.getByText("dev-ci")).toBeInTheDocument();
    expect(screen.getByText("dev-1")).toBeInTheDocument();
    expect(screen.getByText("legacy")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Change permissions for mine" }),
    ).toBeInTheDocument();
    for (const name of ["dev-ci", "legacy"]) {
      expect(
        screen.queryByRole("button", {
          name: `Change permissions for ${name}`,
        }),
      ).not.toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: `Revoke API key ${name}` }),
      ).toBeInTheDocument();
    }

    await user.click(
      screen.getByRole("button", { name: "Revoke API key dev-ci" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("dev-1 created this key.");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );
    expect(revoke).toHaveBeenCalledWith("key-dev", "dev-ci");
    expect(refetch).toHaveBeenCalled();
  });

  it("draws the skeleton as the panel draws the caller's role: the switch for an Admin or an unknown role, none for anyone else", () => {
    for (const role of [true, null, false]) {
      admin = role;
      apiKeysState.keys = [];
      apiKeysState.loading = true;
      const loading = render(<ApiKeysPanel />);
      const loadingTabs = loading.container.querySelector("[role='tablist']");
      const loadingBody = cardContent(loading.container);
      loading.unmount();

      const { container, unmount } = render(<ApiKeysPanelSkeleton />);
      expect(container.querySelector("[role='tablist']") !== null).toBe(
        role !== false,
      );
      expect(loadingTabs !== null).toBe(role !== false);
      // The same switch (disabled until the role is known) over the same
      // loading table.
      if (role !== true) {
        expect(cardContent(container)).toBe(loadingBody);
      }
      unmount();
    }
  });
});

describe("ApiKeysPanel — Render's columns at fixed widths (w9/132)", () => {
  /** The width class of each column, as the table's colgroup sets it. */
  function colWidths(root: ParentNode): string[] {
    return [...root.querySelectorAll("table colgroup col")].map(
      (col) => col.className,
    );
  }

  /** The table's class, and each column's width class. */
  function layout(root: ParentNode) {
    return {
      table: root.querySelector("table")?.className,
      cols: colWidths(root),
    };
  }

  /** Whether a row's first cell holds at the left and its last at the right. */
  function holdsEnds(row: Element | null): boolean {
    const cells = row ? [...row.children] : [];
    return (
      cells.length > 0 &&
      cells[0].classList.contains("sticky") &&
      cells[0].classList.contains("left-0") &&
      cells[cells.length - 1].classList.contains("sticky") &&
      cells[cells.length - 1].classList.contains("right-0")
    );
  }

  it("pins the column set: Name, Permissions, Client ID, Created, Last used and revoke, with no Created by", () => {
    apiKeysState.keys = [key("key-1", "deploy-agent", "admin-1")];
    const { container } = render(<ApiKeysPanel />);

    expect(tableHeaders(container)).toEqual([
      "Name",
      "Permissions",
      "Client ID",
      "Created",
      "Last used",
      "Actions",
    ]);
    expect(container.querySelector("table")).toHaveClass("table-fixed");
    // Name takes what the fixed columns leave.
    expect(colWidths(container)).toEqual([
      "",
      "w-36",
      "w-34",
      "w-18",
      "w-22",
      "w-13",
    ]);
    expect(container.querySelectorAll("tbody tr td")).toHaveLength(6);
    expect(screen.queryByText("admin-1")).not.toBeInTheDocument();
  });

  it("lays out the loading frame and the route's skeleton with the ready table's columns", () => {
    apiKeysState.keys = [key("key-1", "deploy-agent", "admin-1")];
    const ready = render(<ApiKeysPanel />);
    const readyLayout = layout(ready.container);
    expect(readyLayout.cols).toHaveLength(6);
    ready.unmount();

    apiKeysState.keys = [];
    apiKeysState.loading = true;
    const loading = render(<ApiKeysPanel />);
    expect(layout(loading.container)).toEqual(readyLayout);
    loading.unmount();

    const { container } = render(<ApiKeysPanelSkeleton />);
    expect(layout(container)).toEqual(readyLayout);
  });

  it("holds the name and the revoke at the table's ends, in the table and its frame alike", () => {
    apiKeysState.keys = [key("key-1", "deploy-agent", "admin-1")];
    const ready = render(<ApiKeysPanel />);
    expect(holdsEnds(ready.container.querySelector("thead tr"))).toBe(true);
    expect(holdsEnds(ready.container.querySelector("tbody tr"))).toBe(true);
    ready.unmount();

    apiKeysState.keys = [];
    apiKeysState.loading = true;
    const { container } = render(<ApiKeysPanel />);
    expect(holdsEnds(container.querySelector("thead tr"))).toBe(true);
    expect(holdsEnds(container.querySelector("tbody tr"))).toBe(true);
  });

  it("names each key's creator under its name in an Admin's every-member view, and its loading frame reserves that line", async () => {
    admin = true;
    apiKeysState.keys = [key("key-own", "mine", "admin-1")];
    allMembersKeys.keys = [
      key("key-own", "mine", "admin-1"),
      key("key-dev", "dev-ci", "dev-1"),
    ];
    const user = userEvent.setup();
    const ready = render(<ApiKeysPanel />);
    expect(screen.queryByText("admin-1")).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "All members' keys" }));
    const names = [...ready.container.querySelectorAll("tbody tr")].map(
      (row) => row.querySelector("td")?.textContent,
    );
    expect(names).toEqual(["mineCreated by admin-1", "dev-ciCreated by dev-1"]);
    ready.unmount();

    apiKeysState.keys = [];
    allMembersKeys.keys = [];
    apiKeysState.loading = true;
    const { container } = render(<ApiKeysPanel />);
    const creatorLine = "[data-testid='api-key-creator-placeholder']";
    expect(container.querySelector(creatorLine)).toBeNull();
    await user.click(screen.getByRole("tab", { name: "All members' keys" }));
    expect(
      container.querySelector(`tbody tr td:first-child ${creatorLine}`),
    ).not.toBeNull();
  });
});
