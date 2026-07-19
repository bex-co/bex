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
const setVar = vi.fn();
const deleteVar = vi.fn();
const setFile = vi.fn();
const deleteFile = vi.fn();

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
        busy: false,
      }),
      useRevealEnvGroupVar: () => vi.fn(async () => "revealed"),
      useRevealEnvGroupSecretFile: () => vi.fn(async () => "revealed"),
      useEnvGroupVarMutations: () => ({
        setVar,
        deleteVar,
        busy: false,
      }),
      useEnvGroupSecretFileMutations: () => ({
        setFile,
        deleteFile,
        busy: false,
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
    createdAt: "2026-07-15T12:00:00Z",
    updatedAt: "2026-07-15T13:00:00Z",
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
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/env-groups/$groupId",
    component: EnvGroupDetailPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([homeRoute, route]),
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
  createGroup.mockReset().mockResolvedValue("eg-new");
  for (const mutation of [
    renameGroup,
    deleteGroup,
    linkGroup,
    unlinkGroup,
    setVar,
    deleteVar,
    setFile,
    deleteFile,
  ]) {
    mutation.mockReset().mockResolvedValue(true);
  }
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
    expect(screen.getByText("2 variable(s)")).toBeInTheDocument();
    expect(screen.getByText("1 secret file(s)")).toBeInTheDocument();
    expect(screen.getByText("3 linked service(s)")).toBeInTheDocument();
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
    const user = userEvent.setup();
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

    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.type(screen.getByLabelText("Key"), "API_TOKEN");
    await user.type(screen.getByLabelText("Value"), "secret");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(setVar).toHaveBeenCalledWith("API_TOKEN", "secret");

    await user.click(screen.getByRole("button", { name: "Add secret file" }));
    await user.type(screen.getByLabelText("File name"), "cert.pem");
    await user.type(screen.getByLabelText("Contents"), "pem-body");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(setFile).toHaveBeenCalledWith("cert.pem", "pem-body");
  });

  it("renames and typed-confirms deletion using the immutable group id", async () => {
    detailState.group = group();
    const user = userEvent.setup();
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Rename" }));
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
    await user.click(screen.getByRole("button", { name: "Delete" }));
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
