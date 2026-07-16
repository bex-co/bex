import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentCard } from "@/features/environments/components/environment-card";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

const rename = vi.fn();
vi.mock("@/features/environments/hooks/use-rename-environment", () => ({
  useRenameEnvironment: () => ({ rename, busy: false }),
}));

const remove = vi.fn();
vi.mock("@/features/environments/hooks/use-delete-environment", () => ({
  useDeleteEnvironment: () => ({ remove, deleting: null }),
}));

const saveACL = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-acl", () => ({
  useSetEnvironmentACL: () => ({ saveACL, saving: false }),
}));

// The manage-resources dialog reaches Apollo when opened; the card never opens
// it in these tests, but stub it so mounting the (closed) dialog stays inert.
vi.mock("@/features/environments/components/manage-resources-dialog", () => ({
  ManageResourcesDialog: () => null,
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  ResourceTable: ({ rows }: { rows: Array<{ id: string; name: string }> }) => (
    <div data-testid="resource-table">
      {rows.map((row) => row.name).join(",")}
    </div>
  ),
}));

// An environment with no resolvable resources renders the empty state instead
// of the shared ResourceTable (which pulls in service row-actions + Apollo).
const env: EnvironmentView = {
  id: "env-1",
  projectId: "prj-1",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
  envGroupIds: [],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowList: [],
};

function renderCard(
  props: Partial<React.ComponentProps<typeof EnvironmentCard>> = {},
) {
  return render(
    <EnvironmentCard
      environment={env}
      services={[]}
      databases={[]}
      keyValues={[]}
      servicePending={null}
      onRunServiceAction={vi.fn()}
      onDatabaseDeleted={vi.fn()}
      onKeyValueDeleted={vi.fn()}
      {...props}
    />,
  );
}

beforeEach(() => {
  rename.mockReset();
  rename.mockResolvedValue(true);
  remove.mockReset();
  remove.mockResolvedValue(true);
  saveACL.mockReset();
  saveACL.mockResolvedValue(true);
});

describe("EnvironmentCard", () => {
  it("shows the environment name and its empty-resources state", () => {
    renderCard();
    expect(screen.getByText("staging")).toBeInTheDocument();
    expect(
      screen.getByText("No resources in this environment yet."),
    ).toBeInTheDocument();
  });

  it("keeps search and type state scoped to this environment", async () => {
    const onResourceFilterChange = vi.fn();
    const user = userEvent.setup();
    renderCard({
      services: [
        {
          id: "srv-api",
          name: "Public API",
          suspended: false,
          phase: "Running",
          url: "",
          createdAt: null,
          replicas: 1,
          revision: "r1",
        },
      ],
      environment: { ...env, serviceIds: ["srv-api"] },
      resourceFilter: {
        environmentId: "env-1",
        query: "",
        kind: "all",
      },
      onResourceFilterChange,
    });

    await user.type(
      screen.getByRole("searchbox", { name: "Search resources in staging" }),
      "a",
    );
    expect(onResourceFilterChange).toHaveBeenLastCalledWith({
      environmentId: "env-1",
      query: "a",
      kind: "all",
    });

    await user.click(screen.getByRole("button", { name: "Env Groups" }));
    expect(onResourceFilterChange).toHaveBeenLastCalledWith({
      environmentId: "env-1",
      query: "",
      kind: "envgroups",
    });
  });

  it("renders an accessible no-match state for a selected filter", () => {
    renderCard({
      services: [
        {
          id: "srv-api",
          name: "Public API",
          suspended: false,
          phase: "Running",
          url: "",
          createdAt: null,
          replicas: 1,
          revision: "r1",
        },
      ],
      environment: { ...env, serviceIds: ["srv-api"] },
      resourceFilter: {
        environmentId: "env-1",
        query: "missing",
        kind: "services",
      },
    });

    expect(screen.getByRole("status")).toHaveTextContent(
      "No matching resources",
    );
  });

  it("renames via the overflow menu", async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));

    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByRole("textbox");
    await user.clear(input);
    await user.type(input, "production");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(rename).toHaveBeenCalledWith("env-1", "production");
  });

  it("deletes behind a confirmation", async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText('Delete environment "staging"?'),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect(remove).toHaveBeenCalledWith("env-1", "staging");
  });

  it("full-replaces protection, network isolation, and inbound IP rules", async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "All settings" }),
    );

    const dialog = await screen.findByRole("dialog");
    await user.click(
      within(dialog).getByRole("switch", { name: "Protected environment" }),
    );
    await user.click(
      within(dialog).getByRole("switch", {
        name: "Block cross-environment connections",
      }),
    );
    await user.type(
      within(dialog).getByPlaceholderText("203.0.113.0/24"),
      "10.0.0.0/24",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Add source" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(saveACL).toHaveBeenCalledWith("env-1", "staging", {
      protectedStatus: "protected",
      networkIsolationEnabled: true,
      ipAllowList: ["10.0.0.0/24"],
    });
  });
});
