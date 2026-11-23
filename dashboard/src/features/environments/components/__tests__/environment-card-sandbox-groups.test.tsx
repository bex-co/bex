import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentCard } from "@/features/environments/components/environment-card";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { SandboxGroupItem } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import type { ServiceView } from "@/features/services/types";

/**
 * w22/m603: Render's project page lists an environment's sandbox groups in
 * the same resource table as its services, and moves selected groups with the
 * same bulk Move. The card and its real table; the writers are stubbed.
 */
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

vi.mock("@tanstack/react-router", async () => {
  const { PlainLink } = await import("@/test/mocks/router-link");
  return { Link: PlainLink };
});
vi.mock("@/features/environments/hooks/use-rename-environment", () => ({
  useRenameEnvironment: () => ({ rename: vi.fn(), busy: false }),
}));
vi.mock("@/features/environments/hooks/use-delete-environment", () => ({
  useDeleteEnvironment: () => ({ remove: vi.fn(), deleting: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-acl", () => ({
  useSetEnvironmentACL: () => ({ saveACL: vi.fn(), saving: false }),
}));
const setServices = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-services", () => ({
  useSetEnvironmentServices: () => ({ setServices, busyId: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-databases", () => ({
  useSetEnvironmentDatabases: () => ({ setDatabases: vi.fn(), busyId: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-keyvalues", () => ({
  useSetEnvironmentKeyValues: () => ({ setKeyValues: vi.fn(), busyId: null }),
}));
vi.mock("@/features/environments/hooks/use-set-environment-env-groups", () => ({
  useSetEnvironmentEnvGroups: () => ({ setEnvGroups: vi.fn(), busyId: null }),
}));
const setGroupsEnvironment = vi.fn();
vi.mock(
  "@/features/sandbox-groups/hooks/use-set-sandbox-group-environment",
  () => ({
    useSetSandboxGroupEnvironment: () => ({
      setGroupsEnvironment,
      busyId: null,
    }),
  }),
);
vi.mock("@/features/env-groups/hooks/use-stranded-link-guard", () => ({
  useStrandedLinkGuard: () => ({
    groups: [],
    ready: true,
    loading: false,
    error: undefined,
    retry: () => undefined,
    strandedFor: () => [],
    scopeNameFor: () => undefined,
  }),
}));
// The dialog has its own tests; here it only has to receive the groups.
const dialogGroups = vi.hoisted(() => ({ current: null as unknown }));
vi.mock("@/features/environments/components/manage-resources-dialog", () => ({
  ManageResourcesDialog: ({ sandboxGroups }: { sandboxGroups: unknown }) => {
    dialogGroups.current = sandboxGroups;
    return null;
  },
}));
vi.mock("@/features/services/components/service-status-badge", () => ({
  ServiceStatusBadge: () => <span>Running</span>,
}));
vi.mock("@/features/services/components/service-row-actions", () => ({
  ServiceRowActions: () => <button>service options</button>,
}));

const staging: EnvironmentView = {
  id: "evm-staging",
  projectId: "prj-1",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: ["srv-api"],
  databaseIds: [],
  keyValueIds: [],
  envGroupIds: [],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowListEntries: [],
};
const production: EnvironmentView = {
  ...staging,
  id: "evm-production",
  name: "production",
  serviceIds: [],
};

const api = {
  id: "srv-api",
  name: "api",
  type: "web_service",
  suspended: false,
  createdAt: null,
  runtime: "node",
  region: "oregon",
} as unknown as ServiceView;

function group(
  id: string,
  name: string,
  environmentId: string | null,
  isDefault = false,
): SandboxGroupItem {
  return {
    __typename: "SandboxGroup",
    id,
    name,
    ownerId: "tea-1",
    region: "oregon",
    isDefault,
    environmentId,
    concurrencyLimit: 1000,
    createdAt: null,
    updatedAt: null,
  };
}

const groups = [
  group("sbg-agents", "agents", "evm-staging", true),
  group("sbg-batch", "batch", "evm-staging"),
  group("sbg-prod", "prod-runners", "evm-production"),
  group("sbg-loose", "loose", null),
];

function renderCard(sandboxGroups?: SandboxGroupItem[]) {
  return render(
    <EnvironmentCard
      environment={staging}
      allEnvironments={[staging, production]}
      services={[api]}
      databases={[]}
      keyValues={[]}
      sandboxGroups={sandboxGroups}
      servicePending={null}
      onRunServiceAction={vi.fn()}
      onDatabaseDeleted={vi.fn()}
      onKeyValueDeleted={vi.fn()}
    />,
  );
}

const bodyRowNames = () =>
  screen
    .getAllByRole("row")
    .slice(1)
    .map((row) => within(row).getAllByRole("link")[0].textContent);

async function moveSelectionTo(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) {
  await user.click(
    screen.getByRole("combobox", {
      name: "Move selected resources to environment",
    }),
  );
  await user.click(screen.getByRole("option", { name }));
  await user.click(screen.getByRole("button", { name: "Move" }));
}

beforeEach(() => {
  setServices.mockReset();
  setServices.mockResolvedValue(true);
  setGroupsEnvironment.mockReset();
  setGroupsEnvironment.mockImplementation(
    async (_env: unknown, moves: Array<{ id: string }>) =>
      new Set(moves.map((move) => move.id)),
  );
  dialogGroups.current = null;
});

describe("EnvironmentCard sandbox groups", () => {
  it("lists only this environment's groups, after its services, in its count", () => {
    renderCard(groups);

    expect(bodyRowNames()).toEqual(["api", "agents", "batch"]);
    expect(screen.getByText("3 resources")).toBeInTheDocument();
    // "All" counts them; no type pill of their own (the skeleton's five stay).
    expect(screen.getByRole("button", { name: "All (3)" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Sandbox Groups/ }),
    ).not.toBeInTheDocument();
    // Manage resources offers every group of the workspace.
    expect(dialogGroups.current).toBe(groups);
  });

  it("matches a group placed under the environment's historical id", () => {
    renderCard([group("sbg-legacy", "legacy", "env-davm6imde41s73canq0g")]);
    expect(bodyRowNames()).toEqual(["api"]);

    const legacy: EnvironmentView = {
      ...staging,
      id: "evm-davm6imde41s73canq0g",
    };
    render(
      <EnvironmentCard
        environment={legacy}
        services={[]}
        databases={[]}
        keyValues={[]}
        sandboxGroups={[
          group("sbg-legacy", "legacy", "env-davm6imde41s73canq0g"),
        ]}
        servicePending={null}
        onRunServiceAction={vi.fn()}
        onDatabaseDeleted={vi.fn()}
        onKeyValueDeleted={vi.fn()}
      />,
    );
    expect(screen.getByRole("link", { name: "legacy" })).toBeInTheDocument();
  });

  it("shows a group row as Render does, with no row actions", () => {
    renderCard(groups);

    const agents = screen.getByRole("row", { name: /agents/ });
    expect(
      within(agents).getByRole("link", { name: "agents" }),
    ).toHaveAttribute("href", "/sandbox-groups/sbg-agents");
    expect(within(agents).getByText("Sandbox Group")).toBeInTheDocument();
    expect(within(agents).getByText("Default")).toBeInTheDocument();
    expect(within(agents).getByText("Ready")).toBeInTheDocument();
    expect(within(agents).getByText("Oregon, USA")).toBeInTheDocument();
    expect(within(agents).getByLabelText("Not applicable")).toBeInTheDocument();
    expect(
      within(agents).getByRole("checkbox", { name: "Select agents" }),
    ).toBeInTheDocument();
    expect(within(agents).queryAllByRole("button")).toEqual([]);
    // The service row keeps its menu.
    expect(
      within(screen.getByRole("row", { name: /api/ })).getByRole("button", {
        name: "service options",
      }),
    ).toBeInTheDocument();

    expect(
      within(screen.getByRole("row", { name: /batch/ })).queryByText("Default"),
    ).toBeNull();
  });

  it("moves a selected group into the chosen environment and deselects it", async () => {
    const user = userEvent.setup();
    renderCard(groups);

    await user.click(screen.getByRole("checkbox", { name: "Select batch" }));
    expect(screen.getByText("Selected: 1")).toBeInTheDocument();
    await moveSelectionTo(user, "production");

    expect(setGroupsEnvironment).toHaveBeenCalledTimes(1);
    expect(setGroupsEnvironment).toHaveBeenCalledWith(production, [
      { id: "sbg-batch", name: "batch", into: true },
    ]);
    // Nothing else was selected, so nothing else is written.
    expect(setServices).not.toHaveBeenCalled();
    expect(screen.getByText("Selected: 0")).toBeInTheDocument();
  });

  it("moves groups with services, and keeps a refused group selected", async () => {
    setGroupsEnvironment.mockResolvedValue(new Set(["sbg-agents"]));
    const user = userEvent.setup();
    renderCard(groups);

    await user.click(
      screen.getByRole("checkbox", { name: "Select all visible resources" }),
    );
    expect(screen.getByText("Selected: 3")).toBeInTheDocument();
    await moveSelectionTo(user, "production");

    expect(setServices).toHaveBeenCalledWith("evm-production", "production", [
      "srv-api",
    ]);
    expect(setGroupsEnvironment).toHaveBeenCalledWith(production, [
      { id: "sbg-agents", name: "agents", into: true },
      { id: "sbg-batch", name: "batch", into: true },
    ]);
    // The service and the group that moved leave the selection; the refused
    // group stays for a retry.
    expect(screen.getByText("Selected: 1")).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Select batch" }),
    ).toBeChecked();
  });

  it("reads as it does without groups when the workspace has none to show", () => {
    // React's generated ids differ per mount; everything else must not.
    const markup = (container: HTMLElement) =>
      container.innerHTML.replace(/_r_[0-9a-z]+_/g, "_r_");
    const { container: without } = renderCard();
    const before = markup(without);
    without.remove();

    // A failed or missing groups read hands the card no groups (the panel's
    // useSandboxGroups answers []), and groups placed elsewhere add nothing.
    for (const sandboxGroups of [
      [],
      [group("sbg-prod", "prod-runners", "evm-production")],
    ]) {
      const { container } = renderCard(sandboxGroups);
      expect(markup(container)).toBe(before);
      container.remove();
    }
  });
});
