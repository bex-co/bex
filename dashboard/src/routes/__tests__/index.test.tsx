import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { HomePage } from "../index";
import type { ServiceView } from "@/features/services/types";
import type {
  DatabaseView,
  DatabaseInstanceTypeView,
} from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

// The Overview page (Render parity: dashboard.render.com's workspace
// homepage — a project card grid + an ungrouped-resources table) is a pure
// client of the services/databases/key-value/projects hooks; mock them so
// the test drives the list/loading/error/empty/grouped states directly.
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

const run = vi.fn();
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run }),
}));

const databasesState: {
  databases: DatabaseView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<DatabaseView[]>;
  startPolling: (ms: number) => void;
  stopPolling: () => void;
} = {
  databases: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
  startPolling: vi.fn(),
  stopPolling: vi.fn(),
};
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => databasesState,
}));

const keyValuesState: {
  keyValues: KeyValueView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<KeyValueView[]>;
  startPolling: (ms: number) => void;
  stopPolling: () => void;
} = {
  keyValues: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
  startPolling: vi.fn(),
  stopPolling: vi.fn(),
};
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => keyValuesState,
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

// The "+ New" menu's database option and the "New Project" dialog mount
// unconditionally in the page header; stub their hooks (mirrors
// databases.index.test.tsx's handling of the same create dialog).
const instanceTypes: DatabaseInstanceTypeView[] = [
  { id: "free", name: "Free", cpu: "100m", memory: "256Mi", storageGB: 1 },
];
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes,
    loading: false,
    error: undefined,
  }),
}));
const createDatabase = vi.fn();
vi.mock("@/features/databases/hooks/use-create-database", () => ({
  useCreateDatabase: () => ({ create: createDatabase, busy: false }),
}));
// DatabaseRowActions/KeyValueRowActions call their delete hook unconditionally
// (not gated behind the closed "•••" menu), same as useCreateDatabase above.
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove: vi.fn(), deleting: null }),
}));
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove: vi.fn(), deleting: null }),
}));
const createProject = vi.fn();
vi.mock("@/features/projects/hooks/use-create-project", () => ({
  useCreateProject: () => ({ create: createProject, busy: false }),
}));
// The database dialog's Project/Environment selector reads environments over
// Apollo when OPEN (the ?new=database cases below) — same stub
// keyvalue.new.test.tsx uses.
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
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
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
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
    ...overrides,
  };
}

function db(overrides: Partial<DatabaseView> = {}): DatabaseView {
  return {
    id: "shop-db",
    name: "shop-db",
    status: "available",
    plan: "free",
    version: "18",
    diskSizeGB: 1,
    createdAt: null,
    public: false,
    suspended: "not_suspended",
    ...overrides,
  };
}

function kv(overrides: Partial<KeyValueView> = {}): KeyValueView {
  return {
    id: "sessions-cache",
    name: "sessions-cache",
    status: "available",
    plan: "starter",
    version: "8",
    createdAt: null,
    externalHost: null,
    public: false,
    suspended: false,
    ...overrides,
  };
}

function renderHomePage(initialPath = "/") {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: HomePage,
    validateSearch: (
      search: Record<string, unknown>,
    ): { new?: "database" | "project" } =>
      search.new === "database" || search.new === "project"
        ? { new: search.new }
        : {},
  });
  const databaseRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/databases/$databaseId",
    component: () => <p>Database destination</p>,
  });
  const projectRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/project/$projectId",
    component: () => <p>Project destination</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, databaseRoute, projectRoute]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  vi.clearAllMocks();
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
  createDatabase.mockResolvedValue("dpg-returned-id");
  createProject.mockResolvedValue("prj-returned-id");
  run.mockReset();
});

describe("HomePage", () => {
  it("renders ungrouped resources together with a Type column", async () => {
    servicesState.services = [svc({ id: "hello-go", name: "hello-go" })];
    databasesState.databases = [db({ id: "shop-db", name: "shop-db" })];
    keyValuesState.keyValues = [
      kv({ id: "sessions-cache", name: "sessions-cache" }),
    ];

    renderHomePage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("hello-go")).toBeInTheDocument();
    expect(within(table).getByText("shop-db")).toBeInTheDocument();
    expect(within(table).getByText("sessions-cache")).toBeInTheDocument();

    // Each kind has a desktop Type-cell badge and a mobile badge below its
    // name; responsive display classes ensure exactly one is visible.
    expect(within(table).getAllByText("Service")).toHaveLength(2);
    expect(within(table).getAllByText("Database")).toHaveLength(2);
    expect(within(table).getAllByText("Key Value")).toHaveLength(2);

    // each row still links to its own kind's detail page
    expect(within(table).getByText("hello-go").closest("a")).toHaveAttribute(
      "href",
      "/services/hello-go",
    );
    expect(within(table).getByText("shop-db").closest("a")).toHaveAttribute(
      "href",
      "/databases/shop-db",
    );
    expect(
      within(table).getByText("sessions-cache").closest("a"),
    ).toHaveAttribute("href", "/keyvalue/sessions-cache");

    // one actions trigger per row
    expect(
      within(table).getAllByRole("button", { name: "Open actions menu" }),
    ).toHaveLength(3);
  });

  it("renders a project card linking to the project's own page", async () => {
    servicesState.services = [svc({ id: "grouped-svc", name: "grouped-svc" })];
    databasesState.databases = [
      db({ id: "ungrouped-db", name: "ungrouped-db" }),
    ];
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: ["grouped-svc"],
        databaseIds: [],
        keyValueIds: [],
      },
    ];

    renderHomePage();

    const card = (await screen.findByText("storefront")).closest("a");
    expect(card).toHaveAttribute("href", "/project/prj-1");
    // healthy (Running) service in the project -> the green status line
    expect(
      within(card!).getByText("All resources running"),
    ).toBeInTheDocument();

    // the ungrouped database still shows in the ungrouped table, not the card
    expect(screen.getByText("ungrouped-db")).toBeInTheDocument();
    expect(screen.getByText("Ungrouped Resources")).toBeInTheDocument();
  });

  it("shows an unhealthy count on a project card when a resource needs attention", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: ["broken-db"],
        keyValueIds: [],
      },
    ];
    databasesState.databases = [db({ id: "broken-db", status: "unavailable" })];

    renderHomePage();

    const card = (await screen.findByText("storefront")).closest("a");
    expect(
      within(card!).getByText("1 resource(s) need attention"),
    ).toBeInTheDocument();
  });

  it("shows an in-progress count instead of an attention warning while a resource is building", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: ["building-svc"],
        databaseIds: [],
        keyValueIds: [],
      },
    ];
    servicesState.services = [svc({ id: "building-svc", phase: "Building" })];

    renderHomePage();

    const card = (await screen.findByText("storefront")).closest("a");
    expect(
      within(card!).getByText("1 resource(s) in progress"),
    ).toBeInTheDocument();
    expect(
      within(card!).queryByText("1 resource(s) need attention"),
    ).not.toBeInTheDocument();
  });

  it("shows skeleton placeholders while loading with no data", async () => {
    servicesState.loading = true;
    renderHomePage();

    await screen.findByText("Overview");
    expect(
      screen.queryByRole("button", { name: "Open actions menu" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("No resources yet")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Couldn't load resources"),
    ).not.toBeInTheDocument();
  });

  it("shows an error card when a query fails with no data", async () => {
    servicesState.error = new Error("network down");
    renderHomePage();

    expect(
      await screen.findByText("Couldn't load resources"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an empty state when there are no resources or projects", async () => {
    renderHomePage();

    expect(await screen.findByText("No resources yet")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});

// w1/m45: the URL owns the create dialogs, so Render's New-menu aliases
// (`/d/new`, `/new/database`, `/new/project`) can land straight in them.
describe("URL-owned create dialogs (w1/m45)", () => {
  it("?new=database opens the database create dialog", async () => {
    renderHomePage("/?new=database");
    expect(await screen.findByText("New Postgres")).toBeInTheDocument();
  });

  it("?new=project opens the project create dialog", async () => {
    renderHomePage("/?new=project");
    expect(await screen.findByText("New Project")).toBeInTheDocument();
  });

  it("plain / opens no dialog", () => {
    renderHomePage("/");
    expect(screen.queryByText("New Postgres")).not.toBeInTheDocument();
    expect(screen.queryByText("New Project")).not.toBeInTheDocument();
  });

  it("closes the Postgres search dialog before landing on the returned immutable id", async () => {
    const user = userEvent.setup();
    const { router } = renderHomePage("/?new=database");

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "friendly-name");
    await user.click(
      within(dialog).getByRole("button", { name: "Create database" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/databases/dpg-returned-id"),
    );
    expect(router.state.location.search).toEqual({});
    expect(databasesState.refetch).toHaveBeenCalled();
    expect(projectsState.refetch).toHaveBeenCalled();
  });

  it("keeps the Postgres form and route in place when create fails", async () => {
    createDatabase.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderHomePage("/?new=database");

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "friendly-name");
    await user.click(
      within(dialog).getByRole("button", { name: "Create database" }),
    );

    await waitFor(() => expect(createDatabase).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/");
    expect(router.state.location.search).toEqual({ new: "database" });
    expect(within(dialog).getByLabelText("Name")).toHaveValue("friendly-name");
  });

  it("lands a new Project on the returned immutable id after closing its search dialog", async () => {
    const user = userEvent.setup();
    const { router } = renderHomePage("/?new=project");

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "Friendly project");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Project" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/project/prj-returned-id"),
    );
    expect(router.state.location.search).toEqual({});
    expect(projectsState.refetch).toHaveBeenCalled();
  });

  it("keeps the Project form and route in place when create fails", async () => {
    createProject.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderHomePage("/?new=project");

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "Friendly project");
    await user.click(
      within(dialog).getByRole("button", { name: "Create Project" }),
    );

    await waitFor(() => expect(createProject).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/");
    expect(router.state.location.search).toEqual({ new: "project" });
    expect(within(dialog).getByLabelText("Name")).toHaveValue(
      "Friendly project",
    );
  });
});
