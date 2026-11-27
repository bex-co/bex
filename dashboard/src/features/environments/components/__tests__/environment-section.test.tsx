import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentSection } from "@/features/environments/components/environment-section";
import { ProjectEnvironmentSectionSkeleton } from "@/common/components/route-skeletons";
import { SECTION_FILTER_STORAGE_KEY } from "@/features/environments/lib/section-filter";
import { memoryStorage } from "@/test/mocks/storage";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { EnvGroupView } from "@/features/env-groups/types";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

// The create links, as the router would write them.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    search,
    className,
  }: {
    children: React.ReactNode;
    to: string;
    search?: Record<string, string>;
    className?: string;
  }) => (
    <a
      href={search ? `${to}?${new URLSearchParams(search)}` : to}
      className={className}
    >
      {children}
    </a>
  ),
}));

// The caller's role (w3/m281): unset, every action is open, as the fallback
// capabilities answer; a test sets a Developer or a Viewer.
const role = vi.hoisted(() => ({
  current: null as null | { canManage: boolean; createReason?: string },
}));
vi.mock(
  "@/features/capabilities/hooks/use-capabilities",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/features/capabilities/hooks/use-capabilities")
      >();
    return {
      ...actual,
      useCapabilities: () => ({
        loaded: role.current !== null,
        canManage: role.current?.canManage ?? false,
        reasonKey: (action: string) =>
          action === "can_create" ? role.current?.createReason : undefined,
      }),
    };
  },
);

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
// w22/m603: a selected sandbox group moves through its own writer.
const setGroupsEnvironment = vi.fn(async () => new Set<string>());
vi.mock(
  "@/features/sandbox-groups/hooks/use-set-sandbox-group-environment",
  () => ({
    useSetSandboxGroupEnvironment: () => ({
      setGroupsEnvironment,
      busyId: null,
    }),
  }),
);

// w4/m238: the card consults the stranded-link guard before a bulk move.
// Default: reads resolved, nothing stranded — individual tests override
// strandedFor to exercise the confirmation.
const strandedFor = vi.fn(() => [] as unknown[]);
const guardReady = { current: true };
vi.mock("@/features/env-groups/hooks/use-stranded-link-guard", () => ({
  useStrandedLinkGuard: () => ({
    groups: [],
    ready: guardReady.current,
    loading: false,
    error: undefined,
    retry: () => undefined,
    strandedFor,
    scopeNameFor: () => "Project / Production",
  }),
}));

// The manage-resources dialog reaches Apollo when opened; the card never opens
// it in these tests, but stub it so mounting the (closed) dialog stays inert.
const manageOpen = vi.fn();
vi.mock("@/features/environments/components/manage-resources-dialog", () => ({
  ManageResourcesDialog: ({ open }: { open: boolean }) => {
    manageOpen(open);
    return null;
  },
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  resourceSelectionKey: (row: { kind: string; id: string }) =>
    `${row.kind}:${row.id}`,
  // The section draws two tables (w1/m682): its resources, and its env
  // groups. Each adds to the one selection, as the real table's checkboxes do.
  ResourceTable: ({
    rows,
    envGroupColumns,
    selectedKeys,
    onSelectedKeysChange,
  }: {
    rows: Array<{ kind: string; id: string; name: string }>;
    envGroupColumns?: boolean;
    selectedKeys?: ReadonlySet<string>;
    onSelectedKeysChange?: (keys: Set<string>) => void;
  }) => {
    const table = envGroupColumns ? "groups" : "rows";
    const add = (picked: typeof rows) =>
      onSelectedKeysChange?.(
        new Set([
          ...(selectedKeys ?? []),
          ...picked.map((row) => `${row.kind}:${row.id}`),
        ]),
      );
    return (
      <div data-testid={envGroupColumns ? "env-group-table" : "resource-table"}>
        {rows.map((row) => row.name).join(",")}
        {onSelectedKeysChange && rows[0] ? (
          <>
            <button onClick={() => add([rows[0]])}>
              select first {table.slice(0, -1)}
            </button>
            <button onClick={() => add(rows)}>select all {table}</button>
          </>
        ) : null}
      </div>
    );
  },
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

// A complete ServiceView row for the environment's resource table. The tests
// only read id/name/phase, but the prop is a full ServiceView.
const apiService: ServiceView = {
  id: "srv-api",
  name: "Public API",
  slug: null,
  type: "web_service",
  suspended: false,
  phase: "Running",
  url: "",
  internalAddress: null,
  createdAt: null,
  sshAddress: null,
  replicas: 1,
  revision: "r1",
  plan: null,
  idleTTLSeconds: null,
  schedule: null,
  command: null,
  runs: [],
  repo: null,
  branch: null,
  rootDir: null,
  runtime: null,
  builder: null,
  buildCommand: null,
  startCommand: null,
  dockerfilePath: null,
  dockerContext: null,
  registryCredentialId: null,
  buildFilter: null,
  autoDeploy: null,
  autoDeployTrigger: null,
  notifyOnFail: null,
  notificationsToSend: null,
  healthCheckPath: null,
  maxShutdownDelaySeconds: null,
  preDeployCommand: null,
  renderSubdomainPolicy: null,
  publishPath: null,
  routes: [],
  headers: [],
  ipAllowList: null,
  ipAllowListEntries: null,
  maintenanceMode: null,
  outboundIps: null,
};

function renderSection(
  props: Partial<React.ComponentProps<typeof EnvironmentSection>> = {},
) {
  return render(
    <EnvironmentSection
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
  // The section keeps its switcher choice in localStorage (w1/m682).
  vi.stubGlobal("localStorage", memoryStorage());
  role.current = null;
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

const sharedGroup: EnvGroupView = {
  id: "evg-shared",
  name: "shared",
  ownerId: "tea-1",
  environmentId: env.id,
  createdAt: null,
  updatedAt: null,
  revision: "1",
  availability: null,
  serviceLinks: [],
  envVarKeys: [],
  secretFileNames: [],
};

const switcherButton = (name: RegExp) =>
  within(screen.getByRole("group", { name: "Resource type" })).getByRole(
    "button",
    { name },
  );

describe("EnvironmentSection", () => {
  it("is a section named by its h2, with Render's empty state", () => {
    renderSection();
    const section = screen.getByRole("region", { name: "staging" });
    expect(
      within(section).getByRole("heading", { level: 2, name: "staging" }),
    ).toBeVisible();
    expect(
      within(section).getByRole("heading", {
        level: 3,
        name: "staging is empty",
      }),
    ).toBeVisible();
    // Nothing in the workspace to move: Render offers only creating.
    expect(
      screen.getByText("Kickstart your environment by creating a new service."),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Create new service" }),
    ).toHaveAttribute("href", "/web/new?projectId=prj-1&environmentId=env-1");
    expect(
      screen.queryByRole("button", { name: "Move existing services" }),
    ).toBeNull();
    expect(screen.queryByText("Protected environment")).toBeNull();
  });

  it("offers moving into an empty environment when the workspace has resources", async () => {
    const user = userEvent.setup();
    renderSection({ services: [apiService] });
    expect(
      screen.getByText(
        "Kickstart your environment by creating a new service or by moving an existing one.",
      ),
    ).toBeVisible();
    await user.click(
      screen.getByRole("button", { name: "Move existing services" }),
    );
    // Manage resources stands in for Render's Move Services modal (w1/m685).
    expect(manageOpen).toHaveBeenLastCalledWith(true);
  });

  // Render disables both with the permission's message; bex's reasons are
  // the protected-placement rule (w3/m281) and the role's.
  it.each([
    {
      who: "a Developer in a protected environment",
      caller: { canManage: false },
      protectedStatus: "protected",
      reason:
        "Only an admin can create resources in a protected environment or move them into or out of it.",
    },
    {
      who: "a Viewer",
      caller: {
        canManage: false,
        createReason: "capabilities.reasonCanCreate",
      },
      protectedStatus: "unprotected",
      reason: null,
    },
  ])(
    "disables the empty state's actions for $who",
    async ({ caller, protectedStatus, reason }) => {
      role.current = caller;
      const user = userEvent.setup();
      renderSection({
        services: [apiService],
        environment: { ...env, protectedStatus },
      });
      const create = screen.getByRole("button", { name: "Create new service" });
      const move = screen.getByRole("button", {
        name: "Move existing services",
      });
      expect(create).toBeDisabled();
      expect(move).toBeDisabled();
      expect(
        screen.queryByRole("link", { name: "Create new service" }),
      ).toBeNull();
      expect(
        screen.getByRole("button", { name: "New Service" }),
      ).toBeDisabled();
      if (reason) {
        await user.hover(create.parentElement!);
        expect(await screen.findByRole("tooltip")).toHaveTextContent(reason);
      }
    },
  );

  it("keeps the empty state's actions for a Developer in an unprotected environment", () => {
    role.current = { canManage: false };
    renderSection({ services: [apiService] });
    expect(
      screen.getByRole("link", { name: "Create new service" }),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Move existing services" }),
    ).toBeEnabled();
  });

  it("names a protected environment as Render does", () => {
    renderSection({ environment: { ...env, protectedStatus: "protected" } });
    expect(screen.getByText("Protected environment")).toBeVisible();
    expect(
      screen.getByRole("heading", { level: 2, name: "staging" }),
    ).toBeVisible();
  });

  // Render counts services, Postgres, Key Value and sandbox groups under
  // Services, and keeps env groups in a table of their own.
  it("counts Render's three categories and draws env groups in their own table", () => {
    renderSection({
      services: [apiService],
      envGroups: [sharedGroup],
      environment: {
        ...env,
        serviceIds: ["srv-api"],
        envGroupIds: ["evg-shared"],
      },
    });
    expect(switcherButton(/^All/)).toHaveTextContent("All (2)");
    expect(switcherButton(/^Services/)).toHaveTextContent("Services (1)");
    expect(switcherButton(/^Env Groups/)).toHaveTextContent("Env Groups (1)");
    expect(screen.getByTestId("resource-table")).toHaveTextContent(
      "Public API",
    );
    expect(screen.getByTestId("env-group-table")).toHaveTextContent("shared");
  });

  it("searches only its own rows, by name or id", async () => {
    const user = userEvent.setup();
    renderSection({
      services: [apiService],
      envGroups: [sharedGroup],
      environment: {
        ...env,
        serviceIds: ["srv-api"],
        envGroupIds: ["evg-shared"],
      },
    });
    await user.type(
      screen.getByRole("searchbox", { name: "Search resources in staging" }),
      "evg-",
    );
    expect(screen.queryByTestId("resource-table")).toBeNull();
    expect(screen.getByTestId("env-group-table")).toHaveTextContent("shared");
    // Counts are the section's, before search.
    expect(switcherButton(/^All/)).toHaveTextContent("All (2)");

    await user.clear(
      screen.getByRole("searchbox", { name: "Search resources in staging" }),
    );
    await user.type(
      screen.getByRole("searchbox", { name: "Search resources in staging" }),
      "missing",
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "No matching resources",
    );
  });

  it("shows only the chosen category, and Render's sentence for no env groups", async () => {
    const user = userEvent.setup();
    renderSection({
      services: [apiService],
      environment: { ...env, serviceIds: ["srv-api"] },
    });
    await user.click(switcherButton(/^Env Groups/));
    expect(switcherButton(/^Env Groups/)).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.queryByTestId("resource-table")).toBeNull();
    expect(
      screen.getByText("This environment has no env groups."),
    ).toBeVisible();

    await user.click(switcherButton(/^Services/));
    expect(screen.getByTestId("resource-table")).toBeVisible();
    expect(
      screen.queryByText("This environment has no env groups."),
    ).toBeNull();
  });

  it("bulk-moves selected rows while preserving unrelated target members", async () => {
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
      serviceIds: ["srv-existing"],
    };
    renderSection({
      services: [apiService],
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

  it("confirms before a bulk move that strands a linked group (w4/m238)", async () => {
    strandedFor.mockReset();
    strandedFor.mockReturnValue([
      { id: "evg-1", name: "prod-group", environmentId: "env-1" },
    ]);
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
      serviceIds: ["srv-existing"],
    };
    renderSection({
      services: [apiService],
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

    // The confirmation names the stranded group and writes nothing yet.
    expect(
      await screen.findByText("This move leaves linked groups behind"),
    ).toBeInTheDocument();
    expect(screen.getByText("prod-group")).toBeInTheDocument();
    expect(setServices).not.toHaveBeenCalled();

    // Cancel: still no write.
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(setServices).not.toHaveBeenCalled();

    // Again, and confirm: exactly one write, to the chosen target.
    await user.click(screen.getByRole("button", { name: "Move" }));
    await user.click(
      await screen.findByRole("button", { name: "Move anyway" }),
    );
    expect(setServices).toHaveBeenCalledTimes(1);
    expect(setServices).toHaveBeenCalledWith("env-2", "production", [
      "srv-existing",
      "srv-api",
    ]);
  });

  it("moves immediately when no linked group is stranded (w4/m238)", async () => {
    strandedFor.mockReset();
    strandedFor.mockReturnValue([]);
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
      serviceIds: ["srv-existing"],
    };
    renderSection({
      services: [apiService],
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
    expect(setServices).toHaveBeenCalledTimes(1);
    expect(
      screen.queryByText("This move leaves linked groups behind"),
    ).toBeNull();
  });

  it("retains only failed kinds after a truthful partial bulk Move", async () => {
    setDatabases.mockResolvedValue(false);
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
    };
    renderSection({
      services: [apiService],
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

  // w4/564: a group selected with its services moves with them — the guard
  // is told so — and its write waits for theirs, since bex-api refuses a
  // group whose linked services are still elsewhere.
  it("moves a service with its group, the group after the service", async () => {
    strandedFor.mockReset();
    strandedFor.mockReturnValue([]);
    let serviceSettled = false;
    setServices.mockImplementation(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
      serviceSettled = true;
      return true;
    });
    let groupStartedAfterService: boolean | null = null;
    setEnvGroups.mockImplementation(async () => {
      groupStartedAfterService = serviceSettled;
      return true;
    });
    const user = userEvent.setup();
    const linked: EnvGroupView = {
      id: "evg-shared",
      name: "shared",
      ownerId: "tea-1",
      environmentId: env.id,
      createdAt: null,
      updatedAt: null,
      revision: "1",
      availability: null,
      serviceLinks: ["srv-api"],
      envVarKeys: [],
      secretFileNames: [],
    };
    const source: EnvironmentView = {
      ...env,
      serviceIds: ["srv-api"],
      envGroupIds: ["evg-shared"],
    };
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
    };
    renderSection({
      services: [apiService],
      envGroups: [linked],
      environment: source,
      allEnvironments: [source, production],
    });

    await user.click(screen.getByRole("button", { name: "select all rows" }));
    await user.click(screen.getByRole("button", { name: "select all groups" }));
    await user.click(
      screen.getByRole("combobox", {
        name: "Move selected resources to environment",
      }),
    );
    await user.click(screen.getByRole("option", { name: "production" }));
    await user.click(screen.getByRole("button", { name: "Move" }));

    await waitFor(() => expect(setEnvGroups).toHaveBeenCalled());
    expect(strandedFor).toHaveBeenCalledWith(["srv-api"], "env-2", {
      groupIds: ["evg-shared"],
      destinationServiceIds: [],
    });
    expect(groupStartedAfterService).toBe(true);
    expect(
      screen.queryByText("This move leaves linked groups behind"),
    ).not.toBeInTheDocument();
  });

  // w4/m206: a two-group move timed out after moving one group. The next
  // read removed that group from this environment, but its selection key
  // survived: one row on screen, "Selected: 2", Move enabled.
  it("counts only selections still in this environment once a failed move has partly applied", async () => {
    setEnvGroups.mockResolvedValue(false);
    const user = userEvent.setup();
    const group = (id: string, name: string): EnvGroupView => ({
      id,
      name,
      ownerId: "tea-1",
      environmentId: env.id,
      createdAt: null,
      updatedAt: null,
      revision: "1",
      availability: null,
      serviceLinks: [],
      envVarKeys: [],
      secretFileNames: [],
    });
    const envGroups = [
      group("evg-first", "first"),
      group("evg-second", "second"),
    ];
    const right: EnvironmentView = { ...env, id: "env-2", name: "right" };
    const left = (envGroupIds: string[]): EnvironmentView => ({
      ...env,
      envGroupIds,
    });
    const card = (source: EnvironmentView) => (
      <EnvironmentSection
        environment={source}
        services={[]}
        databases={[]}
        keyValues={[]}
        envGroups={envGroups}
        allEnvironments={[source, right]}
        servicePending={null}
        onRunServiceAction={vi.fn()}
        onDatabaseDeleted={vi.fn()}
        onKeyValueDeleted={vi.fn()}
      />
    );
    const { rerender } = render(card(left(["evg-first", "evg-second"])));

    await user.click(screen.getByRole("button", { name: "select all groups" }));
    expect(screen.getByText("Selected: 2")).toBeInTheDocument();
    // A row hidden only by the search stays selected.
    const search = screen.getByRole("searchbox", {
      name: "Search resources in staging",
    });
    await user.type(search, "second");
    expect(screen.getByText("Selected: 2")).toBeInTheDocument();
    await user.clear(search);

    await user.click(
      screen.getByRole("combobox", {
        name: "Move selected resources to environment",
      }),
    );
    await user.click(screen.getByRole("option", { name: "right" }));
    await user.click(screen.getByRole("button", { name: "Move" }));
    expect(setEnvGroups).toHaveBeenCalledWith("env-2", "right", [
      "evg-first",
      "evg-second",
    ]);
    // The write failed as far as the client knows: both stay selected…
    expect(screen.getByText("Selected: 2")).toBeInTheDocument();

    // …until the placement read shows the first group has left.
    rerender(card(left(["evg-second"])));
    expect(screen.getByText("Selected: 1")).toBeInTheDocument();
    setEnvGroups.mockClear();
    await user.click(screen.getByRole("button", { name: "Move" }));
    expect(setEnvGroups).toHaveBeenCalledWith("env-2", "right", ["evg-second"]);

    // The acknowledgment-lost case: both arrived, nothing is left to move.
    rerender(card(left([])));
    expect(screen.queryByText(/Selected: [12]/)).not.toBeInTheDocument();
  });

  it("renames via the overflow menu", async () => {
    const user = userEvent.setup();
    renderSection();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));

    const dialog = await screen.findByRole("dialog");
    // By accessible name — see the project rename test (w4/134).
    const input = within(dialog).getByRole("textbox", {
      name: "Environment name",
    });
    await user.clear(input);
    await user.type(input, "production");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(rename).toHaveBeenCalledWith("env-1", "production");
  });

  it("refuses an over-long environment name inline and never renames (w21/036)", async () => {
    const user = userEvent.setup();
    renderSection();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByRole("textbox", {
      name: "Environment name",
    });
    await user.clear(input);
    await user.click(input);
    await user.paste("a".repeat(101));

    expect(within(dialog).getByRole("button", { name: "Save" })).toBeDisabled();
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "up to 100 characters",
    );
    await user.type(input, "{Enter}");
    expect(rename).not.toHaveBeenCalled();
  });

  it("deletes behind a confirmation", async () => {
    const user = userEvent.setup();
    renderSection();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText('Delete environment "staging"?'),
    ).toBeInTheDocument();
    // w21/037: a detached group becomes unassigned; it does not stay in the project.
    expect(
      within(dialog).getByText(
        /Its environment groups are kept but become unassigned/,
      ),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(/stay in the project/)).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect(remove).toHaveBeenCalledWith("env-1", "staging");
  });

  // w6/m290: the environment's databases and key value stores are deleted with
  // it, as Render's dashboard deletes all of its services, so the typed
  // confirmation names them; env groups stay.
  it("names the databases and key value stores a delete removes", async () => {
    const user = userEvent.setup();
    renderSection({
      environment: {
        ...env,
        databaseIds: ["dpg-1"],
        keyValueIds: ["red-1"],
        envGroupIds: [],
      },
      databases: [
        { id: "dpg-1", name: "orders-db" } as unknown as DatabaseView,
      ],
      keyValues: [
        { id: "red-1", name: "sessions-kv" } as unknown as KeyValueView,
      ],
    });

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(
        /deletes all of its services, databases, and key value stores: orders-db, sessions-kv\. Its environment groups are kept but become unassigned/,
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Delete" }),
    ).toBeDisabled();
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo delete environment staging",
    );
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect(remove).toHaveBeenCalledWith("env-1", "staging");
  });

  it("full-replaces protection, network isolation, and inbound IP rules", async () => {
    const user = userEvent.setup();
    renderSection();

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
    renderSection({
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

  it("warns that the allowlist sees Cloudflare on proxied member domains (w1/m171)", async () => {
    const user = userEvent.setup();
    renderSection({
      environment: {
        ...env,
        ipAllowListEntries: [{ cidrBlock: "10.0.0.0/8", description: "" }],
        ipAllowListProxiedDomains: ["shop.example.com", "www.example.com"],
      },
    });

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "All settings" }),
    );
    const dialog = await screen.findByRole("dialog");
    const notice = within(dialog).getByTestId("ip-allow-list-proxied-notice");
    expect(notice).toHaveTextContent(
      "The allowlist can't see clients on 2 custom domains",
    );
    expect(within(notice).getByText("shop.example.com")).toBeInTheDocument();
    expect(within(notice).getByText("www.example.com")).toBeInTheDocument();
    // The list stays editable and saveable: a warning, never a refusal.
    expect(
      within(dialog).getByRole("textbox", { name: "New CIDR block" }),
    ).toBeEnabled();
  });

  it("shows no Cloudflare warning when no member domain is proxied", async () => {
    const user = userEvent.setup();
    renderSection({ environment: { ...env, ipAllowListProxiedDomains: [] } });

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "All settings" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).queryByTestId("ip-allow-list-proxied-notice"),
    ).toBeNull();
  });
});

// At 390px a squeezed header stacked "production" one letter per line, and a
// loading preview without the header's rows made the page jump on load
// (w4/715). jsdom has no layout: these pin the classes the browser measured,
// shared by the ready section and its preview (w1/m682).
describe("EnvironmentSection header and loading preview (w4/715)", () => {
  function headerRows(container: HTMLElement) {
    const header = container.querySelector<HTMLElement>(
      "section > div:first-child, [data-skeleton-item] > div:first-child",
    )!;
    const [titleBlock, row] = Array.from(header.children) as HTMLElement[];
    const [switcher, actions] = Array.from(row.children) as HTMLElement[];
    const title = titleBlock.firstElementChild as HTMLElement;
    return { header, titleBlock, title, row, switcher, actions };
  }

  it("keeps the name whole on one line and lets the actions wrap under the switcher", () => {
    renderSection({ environment: { ...env, name: "production" } });
    const heading = screen.getByRole("heading", {
      level: 2,
      name: "production",
    });
    const name = within(heading).getByText("production");
    expect(name).toHaveClass("min-w-0", "truncate");
    expect(name).toHaveAttribute("title", "production");
    expect(heading).toHaveClass("h-7");
    expect(heading.parentElement?.nextElementSibling).toHaveClass(
      "flex-wrap",
      "justify-between",
    );
  });

  it("previews the ready section's header, switcher row and search row", () => {
    const ready = renderSection().container;
    const pending = render(<ProjectEnvironmentSectionSkeleton />).container;

    const readyHeader = headerRows(ready);
    const pendingHeader = headerRows(pending);
    expect(pendingHeader.header.className).toBe(readyHeader.header.className);
    expect(pendingHeader.titleBlock.className).toBe(
      readyHeader.titleBlock.className,
    );
    expect(pendingHeader.title.className).toBe(readyHeader.title.className);
    expect(pendingHeader.row.className).toBe(readyHeader.row.className);
    expect(pendingHeader.actions.className).toBe(readyHeader.actions.className);
    // The three switcher pills, each label at its ready box.
    const labels = (switcher: HTMLElement) =>
      Array.from(switcher.children).map((pill) =>
        (pill.textContent ?? "").replace(/\s*\(.*\)$/, ""),
      );
    expect(labels(pendingHeader.switcher)).toEqual(
      labels(readyHeader.switcher),
    );
    expect(labels(pendingHeader.switcher)).toEqual([
      "All",
      "Services",
      "Env Groups",
    ]);
    // New Service and Manage resources at their ready boxes, then the kebab.
    expect(pendingHeader.actions.children).toHaveLength(3);
    for (const index of [0, 1]) {
      expect(pendingHeader.actions.children[index]).toHaveTextContent(
        readyHeader.actions.children[index].textContent ?? "",
      );
    }
    expect(pendingHeader.actions.children[2]).toHaveClass("size-8");
    // One full-width search box under the header.
    const pendingSearch = pendingHeader.header.nextElementSibling!;
    expect(pendingSearch).toHaveClass("h-9", "w-full");
    expect(readyHeader.header.nextElementSibling).toContainElement(
      screen.getAllByRole("searchbox")[0],
    );
  });
});

// Render remembers each environment's choice in localStorage (w1/m682).
describe("EnvironmentSection's remembered switcher", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", memoryStorage());
  });

  it("keeps each environment's own choice across a reload", async () => {
    const user = userEvent.setup();
    const production: EnvironmentView = {
      ...env,
      id: "env-2",
      name: "production",
    };
    const page = () => (
      <>
        <EnvironmentSection
          environment={env}
          services={[]}
          databases={[]}
          keyValues={[]}
          servicePending={null}
          onRunServiceAction={vi.fn()}
          onDatabaseDeleted={vi.fn()}
          onKeyValueDeleted={vi.fn()}
        />
        <EnvironmentSection
          environment={production}
          services={[]}
          databases={[]}
          keyValues={[]}
          servicePending={null}
          onRunServiceAction={vi.fn()}
          onDatabaseDeleted={vi.fn()}
          onKeyValueDeleted={vi.fn()}
        />
      </>
    );
    const first = render(page());
    const staging = () => screen.getByRole("region", { name: "staging" });
    const prod = () => screen.getByRole("region", { name: "production" });
    const pressed = (section: HTMLElement) =>
      within(section)
        .getAllByRole("button", { pressed: true })
        .map((button) => button.textContent);
    await user.click(
      within(staging()).getByRole("button", { name: /^Env Groups/ }),
    );
    await user.click(within(prod()).getByRole("button", { name: /^Services/ }));
    first.unmount();

    render(page());
    expect(pressed(staging())).toEqual(["Env Groups (0)"]);
    expect(pressed(prod())).toEqual(["Services (0)"]);
  });

  it("falls back to All when the stored value is malformed", () => {
    for (const raw of ["{not json", '["envgroups"]', '{"env-1":"databases"}']) {
      localStorage.setItem(SECTION_FILTER_STORAGE_KEY, raw);
      const view = renderSection();
      expect(
        within(screen.getByRole("group", { name: "Resource type" }))
          .getAllByRole("button", { pressed: true })
          .map((button) => button.textContent),
      ).toEqual(["All (0)"]);
      view.unmount();
    }
  });
});
