import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { formatDateTime } from "@/common/lib/format";
import type { EnvGroupView } from "@/features/env-groups/types";
import type { ServiceView } from "@/features/services/types";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

const listState: {
  groups: EnvGroupView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<EnvGroupView[]>;
} = {
  groups: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};

const detailState: {
  group: EnvGroupView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<EnvGroupView | null>;
} = {
  group: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => null),
};

const createGroup = vi.fn();
const renameGroup = vi.fn();
const deleteGroup = vi.fn();
const linkGroup = vi.fn();
const unlinkGroup = vi.fn();
const moveGroup = vi.fn();
const cloneGroup = vi.fn();
const savePatch = vi.fn();
const retryRollout = vi.fn();
const setCurrentWorkspaceId = vi.fn();

const scopeState = {
  projects: [],
  environments: [],
  byId: new Map<string, { id: string; name: string }>(),
  serviceEnvironmentById: new Map<string, string>(),
  loading: false,
  error: undefined as Error | undefined,
};

vi.mock("@/features/env-groups/hooks/use-env-group-scope-index", () => ({
  useEnvGroupScopeIndex: () => scopeState,
  useWorkspaceEnvironmentIndex: () => scopeState,
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: [
      { id: "tea-1", name: "Tea One" },
      { id: "tea-2", name: "Tea Two" },
    ],
    currentWorkspaceId: "tea-1",
    setCurrentWorkspaceId,
  }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

vi.mock(
  "@/features/env-groups/hooks/use-env-groups",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/features/env-groups/hooks/use-env-groups")
      >();
    return {
      ...actual,
      useEnvGroups: () => listState,
      useEnvGroup: () => detailState,
      useEnvGroupMutations: () => ({
        createGroup,
        renameGroup,
        deleteGroup,
        linkGroup,
        unlinkGroup,
        moveGroup,
        cloneGroup,
        busy: false,
      }),
      useRevealEnvGroupVar: () => vi.fn(async () => "revealed"),
      useRevealEnvGroupSecretFile: () => vi.fn(async () => "revealed"),
      useEnvGroupEnvironmentPatch: () => ({
        save: savePatch,
        retryRollout,
        saving: false,
      }),
    };
  },
);

const servicesState: {
  services: ServiceView[];
  loading: boolean;
  error: Error | undefined;
} = {
  services: [],
  loading: false,
  error: undefined,
};

vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => servicesState,
}));

import { EnvGroupsPage } from "../env-groups";
import { EnvGroupDetailPage } from "../env-groups_.$groupId";

function group(overrides: Partial<EnvGroupView> = {}): EnvGroupView {
  return {
    id: "eg1",
    name: "shared",
    ownerId: "tea-1",
    environmentId: null,
    createdAt: "2026-07-15T12:00:00Z",
    updatedAt: "2026-07-15T13:00:00Z",
    revision: "egr1_test",
    serviceLinks: [],
    envVarKeys: [],
    secretFileNames: [],
    ...overrides,
  };
}

function service(id: string, name: string): ServiceView {
  return { id, name } as ServiceView;
}

function renderList() {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/env-groups",
    component: EnvGroupsPage,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/env-groups/$groupId",
    component: () => <p>Environment group destination</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/env-groups"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

function renderDetail(groupId = "eg1") {
  const rootRoute = createRootRoute();
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>services home</div>,
  });
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/env-groups",
    component: () => <div>environment groups list</div>,
  });
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/env-groups/$groupId",
    component: EnvGroupDetailPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([homeRoute, listRoute, route]),
    history: createMemoryHistory({
      initialEntries: [`/env-groups/${groupId}`],
    }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  listState.groups = [];
  listState.loading = false;
  listState.error = undefined;
  detailState.group = null;
  detailState.loading = false;
  detailState.error = undefined;
  servicesState.services = [];
  servicesState.loading = false;
  servicesState.error = undefined;
  scopeState.environments = [];
  scopeState.byId = new Map();
  scopeState.serviceEnvironmentById = new Map();
  scopeState.loading = false;
  scopeState.error = undefined;
  setCurrentWorkspaceId.mockReset();
  createGroup.mockReset().mockResolvedValue("eg-new");
  for (const mutation of [
    renameGroup,
    deleteGroup,
    linkGroup,
    unlinkGroup,
    moveGroup,
    cloneGroup,
    savePatch,
    retryRollout,
  ]) {
    mutation
      .mockReset()
      .mockResolvedValue(
        mutation === savePatch
          ? { affectedServiceIds: [], failedServiceIds: [] }
          : true,
      );
  }
  cloneGroup.mockResolvedValue("eg-clone");
});

describe("EnvGroupsPage", () => {
  it("lists workspace groups with variable, file, and service counts", async () => {
    listState.groups = [
      group({
        name: "production-shared",
        envVarKeys: ["TOKEN", "REGION"],
        secretFileNames: ["cert.pem"],
        serviceLinks: ["web", "worker", "cron"],
      }),
    ];

    renderList();

    expect(await screen.findByText("production-shared")).toBeInTheDocument();
    expect(screen.getByText(/3 linked service/)).toBeInTheDocument();
    expect(screen.getByText("Workspace (no Environment)")).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Env Vars" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Secret Files" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "2" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "1" })).toBeInTheDocument();
    expect(
      screen.getByText(formatDateTime("2026-07-15T13:00:00Z")!),
    ).toBeInTheDocument();
  });

  it("renders distinct empty and error states", async () => {
    const view = renderList();
    expect(
      await screen.findByText("No environment groups"),
    ).toBeInTheDocument();

    view.unmount();
    listState.error = new Error("secret store not configured");
    renderList();

    expect(
      await screen.findByText("Environment groups unavailable"),
    ).toBeInTheDocument();
  });

  it("searches the complete workspace result and resets a no-match state", async () => {
    listState.groups = [
      group({ id: "eg-prod", name: "production" }),
      group({ id: "eg-stage", name: "staging" }),
    ];
    const user = userEvent.setup();
    renderList();

    const search = await screen.findByRole("textbox", {
      name: "Search environment groups by name",
    });
    await user.type(search, "prod");
    expect(screen.getByText("production")).toBeInTheDocument();
    expect(screen.queryByText("staging")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "missing");
    expect(
      screen.getByText("No matching environment groups"),
    ).toBeInTheDocument();
    await user.click(
      screen.getAllByRole("button", { name: "Reset search" }).at(-1)!,
    );
    expect(screen.getByText("staging")).toBeInTheDocument();
  });

  it("renders a known Environment name and an explicit missing-target fallback", async () => {
    scopeState.byId = new Map([
      ["env-prod", { id: "env-prod", name: "Production" }],
    ]);
    listState.groups = [
      group({ id: "known", name: "known", environmentId: "env-prod" }),
      group({ id: "gone", name: "gone", environmentId: "env-deleted" }),
    ];
    renderList();

    expect(await screen.findByText("Production")).toBeInTheDocument();
    expect(
      screen.getByText("Unknown Environment (env-deleted)"),
    ).toBeInTheDocument();
  });

  it("creates a group and navigates to its detail route", async () => {
    const user = userEvent.setup();
    const { router } = renderList();

    await user.click(
      await screen.findByRole("button", { name: "New Environment Group" }),
    );
    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(createGroup).toHaveBeenCalledWith({
      name: "Shared production",
      envVars: [],
      secretFiles: [],
      serviceIds: [],
    });
    expect(
      await screen.findByText("Environment group destination"),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/env-groups/eg-new");
  });

  it("keeps the create dialog and list route in place when create fails", async () => {
    createGroup.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderList();

    await user.click(
      await screen.findByRole("button", { name: "New Environment Group" }),
    );
    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    await waitFor(() => expect(createGroup).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/env-groups");
    expect(screen.getByLabelText("Group name")).toHaveValue(
      "Shared production",
    );
  });
});

describe("EnvGroupDetailPage", () => {
  it("keeps an unlinked group fully editable", async () => {
    detailState.group = group();
    renderDetail();

    expect(
      await screen.findByText(
        "This group isn't linked to any services yet. It is still fully editable.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Group Metadata")).toBeInTheDocument();
    expect(screen.getByText("tea-1")).toBeInTheDocument();
    // Timestamps render through the shared formatter ("July 15, 2026 at …"),
    // not as raw ISO — computed via the helper so the assertion holds in any
    // test-runner timezone.
    expect(
      screen.getByText(formatDateTime("2026-07-15T12:00:00Z")!),
    ).toBeInTheDocument();
    expect(
      screen.getByText(formatDateTime("2026-07-15T13:00:00Z")!),
    ).toBeInTheDocument();

    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
  });

  it("stages a server-generated value and sends one group patch", async () => {
    detailState.group = group();
    const user = userEvent.setup();
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Generated secret" }),
    );
    expect(screen.getByDisplayValue("NEW_SECRET")).toBeInTheDocument();
    expect(savePatch).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Save and deploy" }));

    expect(savePatch).toHaveBeenCalledTimes(1);
    expect(savePatch).toHaveBeenCalledWith(
      {
        envVars: [{ key: "NEW_SECRET", generateValue: true }],
        secretFiles: [],
      },
      "deploy",
    );
  });

  it("renames and typed-confirms deletion using the immutable group id", async () => {
    detailState.group = group();
    const user = userEvent.setup();
    const router = renderDetail();

    await user.click(await screen.findByRole("button", { name: "Manage" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    const renameDialog = screen.getByRole("dialog");
    const name = within(renameDialog).getByLabelText("Group name");
    await user.clear(name);
    await user.type(name, "shared-renamed");
    await user.click(
      within(renameDialog).getByRole("button", { name: "Save name" }),
    );
    expect(renameGroup).toHaveBeenCalledWith("eg1", "shared-renamed");

    await waitFor(() =>
      expect(
        screen.queryByText("Rename environment group"),
      ).not.toBeInTheDocument(),
    );
    await user.click(screen.getByRole("button", { name: "Manage" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const deleteDialog = screen.getByRole("dialog");
    const confirm = within(deleteDialog).getByLabelText("Sudo Command");
    expect(
      within(deleteDialog).getByRole("button", {
        name: "Delete Environment Group",
      }),
    ).toBeDisabled();
    await user.type(confirm, "sudo delete env group shared");
    await user.click(
      within(deleteDialog).getByRole("button", {
        name: "Delete Environment Group",
      }),
    );
    expect(deleteGroup).toHaveBeenCalledWith("eg1");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/env-groups"),
    );
  });

  it("moves scope and clones through server-side management verbs", async () => {
    detailState.group = group({ environmentId: null });
    scopeState.environments = [{ id: "env-prod", name: "Production" } as never];
    const user = userEvent.setup();
    const router = renderDetail();

    await user.click(await screen.findByRole("button", { name: "Manage" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Move group" }),
    );
    let dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("combobox"));
    await user.click(await screen.findByRole("option", { name: "Production" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Move group" }),
    );
    expect(moveGroup).toHaveBeenCalledWith("eg1", "env-prod");

    await user.click(screen.getByRole("button", { name: "Manage" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Clone group" }),
    );
    dialog = screen.getByRole("dialog");
    const selects = within(dialog).getAllByRole("combobox");
    await user.click(selects[0]);
    await user.click(await screen.findByRole("option", { name: "Tea Two" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Clone group" }),
    );

    expect(cloneGroup).toHaveBeenCalledWith(
      "eg1",
      "shared-copy",
      "tea-2",
      null,
    );
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-2");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/env-groups/eg-clone"),
    );
  });

  it("unlinks a linked service from the detail page", async () => {
    detailState.group = group({ serviceLinks: ["web"] });
    servicesState.services = [service("web", "Web API")];
    const user = userEvent.setup();
    renderDetail();

    expect(await screen.findByText("Web API")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Unlink" }));

    expect(unlinkGroup).toHaveBeenCalledWith("eg1", "web");
  });

  it("links an available workspace service from the detail page", async () => {
    detailState.group = group();
    servicesState.services = [service("worker", "Background Worker")];
    const user = userEvent.setup();
    renderDetail();

    await user.click(await screen.findByRole("combobox"));
    await user.click(
      await screen.findByRole("option", { name: "Background Worker" }),
    );
    await user.click(screen.getByRole("button", { name: "Link Service" }));

    expect(linkGroup).toHaveBeenCalledWith("eg1", "worker");
  });

  it("offers only same-Environment services and flags legacy drift", async () => {
    detailState.group = group({
      environmentId: "env-a",
      serviceLinks: ["legacy-b"],
    });
    servicesState.services = [
      service("web-a", "Service A"),
      service("web-b", "Service B"),
      service("legacy-b", "Legacy B"),
    ];
    scopeState.byId = new Map([["env-a", { id: "env-a", name: "A" }]]);
    scopeState.serviceEnvironmentById = new Map([
      ["web-a", "env-a"],
      ["web-b", "env-b"],
      ["legacy-b", "env-b"],
    ]);
    const user = userEvent.setup();
    renderDetail();

    expect(
      await screen.findByText("Legacy link outside this group's Environment"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("combobox"));
    expect(
      await screen.findByRole("option", { name: "Service A" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Service B" }),
    ).not.toBeInTheDocument();
  });

  it("keeps existing links visible when the workspace-service query fails", async () => {
    detailState.group = group({ serviceLinks: ["web"] });
    servicesState.error = new Error("network down");
    renderDetail();

    expect(
      await screen.findByText(
        "Couldn't load workspace services. Existing links remain available below.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("web")).toBeInTheDocument();
  });

  it("redirects a dead group id home instead of a generic crash (w9/m55)", async () => {
    detailState.error = new Error("environment group not found");
    const router = renderDetail("missing");

    expect(await screen.findByText("services home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("redirects home when the row is null without any error (w9/m55)", async () => {
    detailState.group = null;
    detailState.error = undefined;
    const router = renderDetail("missing");

    expect(await screen.findByText("services home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("stays put on the inline error state when the query fails (w9/m55)", async () => {
    detailState.group = null;
    detailState.error = new Error("forbidden");
    const router = renderDetail("missing");

    expect(await screen.findByText("Not authorized")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/env-groups/missing");
  });
});
