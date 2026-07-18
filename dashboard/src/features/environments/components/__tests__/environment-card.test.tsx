import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentCard } from "@/features/environments/components/environment-card";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

const rename = vi.fn();

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});
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

const setServices = vi.fn();
const setDatabases = vi.fn();
const setKeyValues = vi.fn();
const setEnvGroups = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-services", () => ({
  useSetEnvironmentServices: () => ({ setServices, busyId: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-databases", () => ({
  useSetEnvironmentDatabases: () => ({ setDatabases, busyId: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-keyvalues", () => ({
  useSetEnvironmentKeyValues: () => ({ setKeyValues, busyId: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-env-groups", () => ({
  useSetEnvironmentEnvGroups: () => ({ setEnvGroups, busyId: null }),
}));

// The manage-resources dialog reaches Apollo when opened; the card never opens
// it in these tests, but stub it so mounting the (closed) dialog stays inert.
vi.mock("@/features/environments/components/manage-resources-dialog", () => ({
  ManageResourcesDialog: () => null,
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  resourceSelectionKey: (row: { kind: string; id: string }) =>
    `${row.kind}:${row.id}`,
  ResourceTable: ({
    rows,
    selectedKeys,
    onSelectedKeysChange,
  }: {
    rows: Array<{ kind: string; id: string; name: string }>;
    selectedKeys?: ReadonlySet<string>;
    onSelectedKeysChange?: (keys: Set<string>) => void;
  }) => (
    <div data-testid="resource-table">
      {rows.map((row) => row.name).join(",")}
      {onSelectedKeysChange && rows[0] ? (
        <>
          <button
            onClick={() =>
              onSelectedKeysChange(new Set([`${rows[0].kind}:${rows[0].id}`]))
            }
          >
            select first row
          </button>
          <button
            onClick={() =>
              onSelectedKeysChange(
                new Set(rows.map((row) => `${row.kind}:${row.id}`)),
              )
            }
          >
            select all rows
          </button>
        </>
      ) : null}
      <span>{selectedKeys?.size ?? 0}</span>
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
  ipAllowListEntries: [],
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
  for (const mutate of [
    setServices,
    setDatabases,
    setKeyValues,
    setEnvGroups,
  ]) {
    mutate.mockReset();
    mutate.mockResolvedValue(true);
  }
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

    await user.click(screen.getByRole("button", { name: "Env Groups (0)" }));
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

  it("bulk-moves selected rows while preserving unrelated target members", async () => {
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
      serviceIds: ["srv-existing"],
    };
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
      allEnvironments: [{ ...env, serviceIds: ["srv-api"] }, production],
    });

    await user.click(screen.getByRole("button", { name: "select first row" }));
    await user.click(
      screen.getByRole("combobox", {
        name: "Move selected resources to environment",
      }),
    );
    await user.click(screen.getByRole("option", { name: "production" }));
    await user.click(screen.getByRole("button", { name: "Move" }));

    expect(setServices).toHaveBeenCalledWith("env-2", "production", [
      "srv-existing",
      "srv-api",
    ]);
    expect(screen.getByText("Selected: 0")).toBeInTheDocument();
  });

  it("retains only failed kinds after a truthful partial bulk Move", async () => {
    setDatabases.mockResolvedValue(false);
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
    };
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
      databases: [
        {
          id: "dpg-main",
          name: "Main DB",
          status: "available",
          plan: null,
          version: "16",
          diskSizeGB: 1,
          createdAt: null,
          public: false,
          suspended: "not_suspended",
        },
      ],
      environment: {
        ...env,
        serviceIds: ["srv-api"],
        databaseIds: ["dpg-main"],
      },
      allEnvironments: [
        { ...env, serviceIds: ["srv-api"], databaseIds: ["dpg-main"] },
        production,
      ],
    });

    await user.click(screen.getByRole("button", { name: "select all rows" }));
    await user.click(
      screen.getByRole("combobox", {
        name: "Move selected resources to environment",
      }),
    );
    await user.click(screen.getByRole("option", { name: "production" }));
    await user.click(screen.getByRole("button", { name: "Move" }));

    expect(setServices).toHaveBeenCalled();
    expect(setDatabases).toHaveBeenCalled();
    expect(screen.getByText("Selected: 1")).toBeInTheDocument();
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
    expect(
      within(dialog).getByText(/will DENY all traffic/),
    ).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("switch", { name: "Protected environment" }),
    );
    await user.click(
      within(dialog).getByRole("switch", {
        name: "Block cross-environment connections",
      }),
    );
    await user.type(
      within(dialog).getByRole("textbox", { name: "New CIDR block" }),
      "10.0.0.0/24",
    );
    await user.type(
      within(dialog).getByRole("textbox", { name: "New rule description" }),
      "office",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Add source" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(saveACL).toHaveBeenCalledWith("env-1", "staging", {
      protectedStatus: "protected",
      networkIsolationEnabled: true,
      ipAllowListEntries: [{ cidrBlock: "10.0.0.0/24", description: "office" }],
    });
  });

  it("edits and removes existing rules and keeps descriptions optional", async () => {
    const user = userEvent.setup();
    renderCard({
      environment: {
        ...env,
        ipAllowListEntries: [
          { cidrBlock: "10.0.0.0/8", description: "office" },
          { cidrBlock: "192.168.0.0/16", description: "" },
        ],
      },
    });

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "All settings" }),
    );

    const dialog = await screen.findByRole("dialog");
    const description = within(dialog).getByRole("textbox", {
      name: "Description for rule 1",
    });
    await user.clear(description);
    await user.type(description, "headquarters");
    await user.click(
      within(dialog).getByRole("button", { name: "Remove 192.168.0.0/16" }),
    );
    await user.type(
      within(dialog).getByRole("textbox", { name: "New CIDR block" }),
      "203.0.113.0/24",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Add source" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(saveACL).toHaveBeenCalledWith("env-1", "staging", {
      protectedStatus: "unprotected",
      networkIsolationEnabled: false,
      ipAllowListEntries: [
        { cidrBlock: "10.0.0.0/8", description: "headquarters" },
        { cidrBlock: "203.0.113.0/24", description: "" },
      ],
    });
  });
});
