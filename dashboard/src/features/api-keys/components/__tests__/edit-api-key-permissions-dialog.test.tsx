import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditApiKeyPermissionsDialog } from "@/features/api-keys/components/edit-api-key-permissions-dialog";
import type { UseApiKeyPermissionCatalogResult } from "@/features/api-keys/hooks/use-api-key-permission-catalog";
import type { ApiKeyView } from "@/features/api-keys/types";
import { CATALOG } from "@/features/api-keys/__tests__/catalog-fixtures";

const update = vi.fn();
let busy = false;
vi.mock("@/features/api-keys/hooks/use-update-api-key-permissions", () => ({
  useUpdateApiKeyPermissions: () => ({ update, busy }),
}));

let catalog: UseApiKeyPermissionCatalogResult;
vi.mock("@/features/api-keys/hooks/use-api-key-permission-catalog", () => ({
  useApiKeyPermissionCatalog: () => catalog,
}));

const entry: ApiKeyView = {
  id: "key-1",
  name: "deploy-agent",
  createdAt: null,
  createdBy: null,
  lastUsedAt: null,
  permissions: {
    all: false,
    grants: [
      { permission: "services", level: "read" },
      { permission: "git", level: "read" },
    ],
  },
};

beforeEach(() => {
  update.mockReset();
  busy = false;
  catalog = { areas: CATALOG, error: undefined, refetch: vi.fn() };
});

async function openEditor(
  user: ReturnType<typeof userEvent.setup>,
  key: ApiKeyView = entry,
) {
  render(<EditApiKeyPermissionsDialog entry={key} />);
  await user.click(
    screen.getByRole("button", { name: `Change permissions for ${key.name}` }),
  );
  return screen.findByRole("dialog", { name: "Edit permissions" });
}

function radio(dialog: HTMLElement, permission: string, level: string) {
  return within(
    within(dialog).getByRole("radiogroup", { name: permission }),
  ).getByRole("radio", { name: level });
}

describe("EditApiKeyPermissionsDialog — the gear on a key's row (w9/m496)", () => {
  it("opens on the key's current grant, with Save changes off until something changes", async () => {
    const user = userEvent.setup();
    const dialog = await openEditor(user);

    expect(
      within(dialog).getByText(
        "Change what deploy-agent can use, then click Save changes. The new permissions apply right away.",
      ),
    ).toBeInTheDocument();
    expect(
      radio(
        dialog,
        "Web and private services, static sites, background workers, and cron jobs",
        "Read",
      ),
    ).toBeChecked();
    expect(
      radio(dialog, "Git connections and repositories", "Read"),
    ).toBeChecked();
    expect(
      within(dialog).getByRole("button", { name: "Save changes" }),
    ).toBeDisabled();
  });

  it("saves the edited grants through updateApiKeyPermissions and closes", async () => {
    update.mockResolvedValue(true);
    const user = userEvent.setup();
    const dialog = await openEditor(user);

    await user.click(
      radio(
        dialog,
        "Web and private services, static sites, background workers, and cron jobs",
        "Read & write",
      ),
    );
    await user.click(radio(dialog, "Git connections and repositories", "None"));
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );

    expect(update).toHaveBeenCalledWith("key-1", "deploy-agent", {
      grants: [{ permission: "services", level: "read_write" }],
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("All saves {all: true}, and Clear all saves an empty grant list", async () => {
    update.mockResolvedValue(false);
    const user = userEvent.setup();
    const dialog = await openEditor(user);

    await user.click(within(dialog).getByRole("button", { name: "All" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );
    expect(update).toHaveBeenLastCalledWith("key-1", "deploy-agent", {
      all: true,
    });

    await user.click(within(dialog).getByRole("button", { name: "Clear all" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );
    expect(update).toHaveBeenLastCalledWith("key-1", "deploy-agent", {
      grants: [],
    });
  });

  it("a refused save keeps the editor open with the user's choice", async () => {
    // The hook toasts the server's refusal (api_key_permissions_editor_forbidden,
    // api_key_permission_above_role) and resolves false.
    update.mockResolvedValue(false);
    const user = userEvent.setup();
    const dialog = await openEditor(user);

    await user.click(within(dialog).getByRole("button", { name: "Read only" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(
      radio(dialog, "Environment variables and secret files", "Read"),
    ).toBeChecked();
  });

  it("a key with All opens on All", async () => {
    const user = userEvent.setup();
    const dialog = await openEditor(user, {
      ...entry,
      permissions: { all: true, grants: [] },
    });
    expect(
      within(dialog).getByText(
        /All grants every permission the creator's member role allows/,
      ),
    ).toBeInTheDocument();
    expect(
      radio(dialog, "Sandbox execution and file transfer", "Execute"),
    ).toBeChecked();
  });

  it("cannot save while the catalog is unavailable, and offers to read it again", async () => {
    catalog = {
      areas: undefined,
      error: new Error("boom"),
      refetch: vi.fn().mockResolvedValue(undefined),
    };
    const user = userEvent.setup();
    const dialog = await openEditor(user);

    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Couldn't load the permission list.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Try again" }));
    expect(catalog.refetch).toHaveBeenCalledTimes(1);
    expect(
      within(dialog).getByRole("button", { name: "Save changes" }),
    ).toBeDisabled();
  });
});
