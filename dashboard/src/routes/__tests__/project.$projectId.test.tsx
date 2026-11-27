import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { Route as OverviewRoute } from "../project.$projectId._overview";
import { ProjectPage } from "@/features/projects/pages/project-page";
import { parseLegacyProjectSearch } from "@/features/projects/lib/resource-filter";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

// A single project's page (Render parity: dashboard.render.com/project/{id})
// is a pure client of the same hooks the Overview page uses, scoped down to
// one project id from the URL.
const servicesState: {
  services: ServiceView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<ServiceView[]>;
} = {
  services: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => servicesState,
}));

vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run: vi.fn() }),
}));

const databasesState: {
  databases: DatabaseView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<DatabaseView[]>;
} = {
  databases: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => databasesState,
}));
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove: vi.fn(), deleting: null }),
}));

const keyValuesState: {
  keyValues: KeyValueView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<KeyValueView[]>;
} = {
  keyValues: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => keyValuesState,
}));
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove: vi.fn(), deleting: null }),
}));

const projectsState: {
  projects: ProjectView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  projects: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => undefined),
};
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => projectsState,
}));

// The header counts the project's environments against the plan (w1/m682).
const workspaceState = vi.hoisted(() => ({ plan: "hobby" as string }));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    currentWorkspaceId: "tea-1",
    currentWorkspace: { id: "tea-1", plan: workspaceState.plan },
  }),
}));
const environmentsState = vi.hoisted(() => ({
  environments: [] as EnvironmentView[],
  loading: false,
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: environmentsState.environments,
    loading: environmentsState.loading,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/environments/components/new-environment-dialog", () => ({
  NewEnvironmentDialog: ({ open }: { open: boolean }) =>
    open ? <div role="dialog" aria-label="New environment" /> : null,
}));
const capabilityState = vi.hoisted(() => ({
  createReason: undefined as string | undefined,
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({
    reasonKey: (action: string) =>
      action === "can_create" ? capabilityState.createReason : undefined,
  }),
}));

const buildSourcesState = vi.hoisted(() => ({
  buildSources: [] as Array<{
    id: string;
    name: string;
    projectId: string | null;
    kind: "git" | "image";
    git: { repoUrl: string; branch: string; region: string | null } | null;
    image: { imageUrl: string } | null;
    serviceLinks: Array<{ id: string; name: string }>;
    updatedAt: string | null;
  }>,
  loading: false,
  error: undefined as Error | undefined,
  refetch: vi.fn(async () => []),
}));
vi.mock("@/features/build-sources/hooks/use-build-sources-list", () => ({
  useBuildSourcesList: ({ projectId }: { projectId?: string | null } = {}) => ({
    buildSources: projectId
      ? buildSourcesState.buildSources.filter((b) => b.projectId === projectId)
      : buildSourcesState.buildSources,
    loading: buildSourcesState.loading,
    error: buildSourcesState.error,
    refetch: buildSourcesState.refetch,
  }),
}));

// The Environments panel is an Apollo client of its own (covered by its own
// tests); stub it here so this route test stays focused on the project page's
// resource table + rename, without needing an ApolloProvider.
vi.mock("@/features/environments/components/environments-panel", () => ({
  EnvironmentsPanel: ({
    projectRows,
    catalog,
    focusedId,
  }: {
    projectRows: Array<{ name: string }>;
    catalog?: { failed: string[] };
    focusedId?: string | null;
  }) => (
    <div
      data-testid="environments-panel"
      data-failed={catalog?.failed.join(",") ?? ""}
      data-focused={focusedId ?? ""}
    >
      {projectRows.map((row) => row.name).join(",")}
    </div>
  ),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "eden-cms-v2",
    name: "eden-cms-v2",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://eden-cms-v2.onbex.co",
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
    ...overrides,
  };
}

function renderProjectPage(path: string = "/project/prj-1") {
  const rootRoute = createRootRoute();
  const projectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    loader: ({ params }) => {
      const project = projectsState.projects.find(
        (candidate) => candidate.id === params.projectId,
      );
      return project
        ? ({ state: "ready", resource: project } as const)
        : ({ state: "not-found" } as const);
    },
  });
  // The real `_overview` layout's search and redirect, over Render's two URLs.
  const overviewRoute = createRoute({
    getParentRoute: () => projectRoute,
    id: "_overview",
    component: ProjectPage,
    validateSearch: parseLegacyProjectSearch,
    beforeLoad: (ctx) =>
      (OverviewRoute.options.beforeLoad as (c: unknown) => void)(ctx),
  });
  const indexRoute = createRoute({
    getParentRoute: () => overviewRoute,
    path: "/",
  });
  const environmentRoute = createRoute({
    getParentRoute: () => overviewRoute,
    path: "environment/$environmentId",
  });
  const settingsRoute = createRoute({
    getParentRoute: () => projectRoute,
    path: "settings",
    component: () => <p>settings page</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      projectRoute.addChildren([
        overviewRoute.addChildren([indexRoute, environmentRoute]),
        settingsRoute,
      ]),
    ]),
    history: createMemoryHistory({ initialEntries: [path] }),
    context: { client: {} as never, session: null },
  });
  return { router, ...render(<RouterProvider router={router} />) };
}

beforeEach(() => {
  servicesState.services = [];
  servicesState.loading = false;
  servicesState.error = undefined;
  databasesState.databases = [];
  databasesState.loading = false;
  databasesState.error = undefined;
  keyValuesState.keyValues = [];
  keyValuesState.loading = false;
  keyValuesState.error = undefined;
  projectsState.projects = [];
  projectsState.loading = false;
  projectsState.error = undefined;
  buildSourcesState.buildSources = [];
  buildSourcesState.loading = false;
  buildSourcesState.error = undefined;
  workspaceState.plan = "hobby";
  environmentsState.environments = [];
  environmentsState.loading = false;
  capabilityState.createReason = undefined;
});

afterEach(() => {
  vi.useRealTimers();
});

const storefront: ProjectView = {
  id: "prj-1",
  name: "storefront",
  ownerId: "tea-1",
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
};

function environment(id: string, name: string): EnvironmentView {
  return {
    id,
    projectId: "prj-1",
    name,
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
}

describe("ProjectPage", () => {
  it("renders the project's own resources", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: ["eden-cms-v2"],
        databaseIds: [],
        keyValueIds: [],
      },
    ];
    servicesState.services = [svc()];

    renderProjectPage();

    expect(
      await screen.findByRole("heading", { name: "storefront" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("environments-panel")).toHaveTextContent(
      "eden-cms-v2",
    );
  });

  // w4/m227: the page hands its resource lists' availability to the panel,
  // so a failed read is never joined as an empty list.
  it("passes a failed Services read to the panel as unavailable", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: ["eden-cms-v2"],
        databaseIds: [],
        keyValueIds: [],
      },
    ];
    servicesState.error = new Error("deadline");
    renderProjectPage();
    expect(
      await screen.findByRole("heading", { name: "storefront" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("environments-panel")).toHaveAttribute(
      "data-failed",
      "services",
    );
  });

  it("renders nothing for an unknown project id — the layout's redirect home is in flight (w9/m55)", async () => {
    renderProjectPage("/project/prj-missing");

    await waitFor(() =>
      expect(
        screen.queryByTestId("environments-panel"),
      ).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  });

  // Render's header: the name links to Project Settings with its Name card
  // editing, and there is no rename dialog on the page (w1/m682).
  it("links the name to Project Settings' Name editor", async () => {
    projectsState.projects = [storefront];
    renderProjectPage();

    const heading = await screen.findByRole("heading", {
      level: 1,
      name: "storefront",
    });
    const link = screen.getByRole("link", {
      name: "storefront Edit project name",
    });
    expect(link).toContainElement(heading);
    expect(link).toHaveAttribute("href", "/project/prj-1/settings?edit=name");
    expect(screen.getByText("Project")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  });

  it("opens the new-environment dialog below the plan's limit", async () => {
    projectsState.projects = [storefront];
    environmentsState.environments = [environment("evm-1", "production")];
    const user = userEvent.setup();
    renderProjectPage();

    await user.click(
      await screen.findByRole("button", { name: "Add environment" }),
    );
    expect(
      screen.getByRole("dialog", { name: "New environment" }),
    ).toBeVisible();
  });

  it("shows Render's paywall instead at Hobby's two environments", async () => {
    projectsState.projects = [storefront];
    environmentsState.environments = [
      environment("evm-1", "production"),
      environment("evm-2", "staging"),
    ];
    const user = userEvent.setup();
    renderProjectPage();

    const button = await screen.findByRole("button", {
      name: /Add environment/,
    });
    expect(button).toHaveTextContent("2/2");
    await user.click(button);
    const paywall = await screen.findByRole("dialog", {
      name: "Get more environments with Pro",
    });
    expect(paywall).toHaveTextContent(
      "Upgrade to get unlimited environments and all Pro plan features.",
    );
    expect(
      screen.queryByRole("dialog", { name: "New environment" }),
    ).toBeNull();
  });

  it("has no environment limit from Pro up", async () => {
    workspaceState.plan = "pro";
    projectsState.projects = [storefront];
    environmentsState.environments = [
      environment("evm-1", "production"),
      environment("evm-2", "staging"),
    ];
    const user = userEvent.setup();
    renderProjectPage();

    const button = await screen.findByRole("button", {
      name: "Add environment",
    });
    expect(button).not.toHaveTextContent("2/2");
    await user.click(button);
    expect(
      screen.getByRole("dialog", { name: "New environment" }),
    ).toBeVisible();
  });

  it("disables Add environment with the role's reason", async () => {
    projectsState.projects = [storefront];
    capabilityState.createReason = "capabilities.reasonCanCreate";
    renderProjectPage();
    expect(
      await screen.findByRole("button", { name: "Add environment" }),
    ).toBeDisabled();
  });
});

// Render's `/project/<id>/environment/<envId>` (w1/m682).
describe("ProjectPage environment URL", () => {
  it("outlines the named section for two seconds, then resets the address on scroll", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    projectsState.projects = [storefront];
    environmentsState.environments = [
      environment("evm-1", "production"),
      environment("evm-2", "staging"),
    ];
    const { router } = renderProjectPage("/project/prj-1/environment/evm-2");

    const panel = await screen.findByTestId("environments-panel");
    expect(panel).toHaveAttribute("data-focused", "evm-2");
    // A scroll inside the two seconds (the smooth scroll itself) keeps it.
    const scroller = document.querySelector("[data-page-scroll]")!;
    fireEvent.scroll(scroller);
    expect(router.state.location.pathname).toBe(
      "/project/prj-1/environment/evm-2",
    );

    await act(async () => {
      vi.advanceTimersByTime(2000);
    });
    expect(panel).toHaveAttribute("data-focused", "");
    expect(router.state.location.pathname).toBe(
      "/project/prj-1/environment/evm-2",
    );

    fireEvent.scroll(scroller);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/project/prj-1"),
    );
    // The same page: no remount, nothing focused.
    expect(screen.getByTestId("environments-panel")).toBe(panel);
  });

  it("lands an unknown environment on the project's own page, with no error", async () => {
    projectsState.projects = [storefront];
    environmentsState.environments = [environment("evm-1", "production")];
    const { router } = renderProjectPage("/project/prj-1/environment/evm-gone");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/project/prj-1"),
    );
    expect(screen.getByTestId("environments-panel")).toHaveAttribute(
      "data-focused",
      "",
    );
    expect(
      screen.getByRole("heading", { level: 1, name: "storefront" }),
    ).toBeVisible();
  });

  it("waits for the environments before judging the id", async () => {
    projectsState.projects = [storefront];
    environmentsState.loading = true;
    const { router } = renderProjectPage("/project/prj-1/environment/evm-2");
    await screen.findByTestId("environments-panel");
    expect(router.state.location.pathname).toBe(
      "/project/prj-1/environment/evm-2",
    );
  });

  it.each([
    ["?env=evm-2&q=api&kind=databases", "/project/prj-1/environment/evm-2", ""],
    ["?env=unassigned", "/project/prj-1/environment/unassigned", ""],
    ["?q=api&kind=services", "/project/prj-1", ""],
  ])("redirects an old %s link once", async (search, pathname, searchStr) => {
    projectsState.projects = [storefront];
    environmentsState.environments = [environment("evm-2", "staging")];
    const { router } = renderProjectPage(`/project/prj-1${search}`);
    await waitFor(() => expect(router.state.location.pathname).toBe(pathname));
    expect(router.state.location.searchStr).toBe(searchStr);
  });
});
