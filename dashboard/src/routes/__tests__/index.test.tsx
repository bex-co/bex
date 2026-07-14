import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { HomePage } from "../index";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView, DatabaseInstanceTypeView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

// The unified Projects page (Render parity, w1/m31 extension) is a pure client
// of the services/databases/key-value/projects hooks; mock them so the test
// drives the list/loading/error/empty/grouped states directly.
const servicesState: {
  services: ServiceView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<ServiceView[]>;
} = { services: [], loading: false, error: undefined, refetch: vi.fn(async () => []) };
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
} = { projects: [], loading: false, error: undefined, refetch: vi.fn(async () => undefined) };
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
  useDatabaseInstanceTypes: () => ({ instanceTypes, loading: false, error: undefined }),
}));
vi.mock("@/features/databases/hooks/use-create-database", () => ({
  useCreateDatabase: () => ({ create: vi.fn(), busy: false }),
}));
// DatabaseRowActions/KeyValueRowActions call their delete hook unconditionally
// (not gated behind the closed "•••" menu), same as useCreateDatabase above.
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove: vi.fn(), deleting: null }),
}));
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove: vi.fn(), deleting: null }),
}));
vi.mock("@/features/projects/hooks/use-create-project", () => ({
  useCreateProject: () => ({ create: vi.fn(), busy: false }),
}));
// Each project section's "•••" menu mounts unconditionally whenever there's
// at least one project group.
vi.mock("@/features/projects/hooks/use-rename-project", () => ({
  useRenameProject: () => ({ rename: vi.fn(), busy: false }),
}));
vi.mock("@/features/projects/hooks/use-delete-project", () => ({
  useDeleteProject: () => ({ remove: vi.fn(), deleting: null }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: null,
    replicas: 1,
    revision: "r1",
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

function renderHomePage() {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: HomePage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
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
  run.mockReset();
});

describe("HomePage", () => {
  it("renders services, databases, and key value stores together with a Type column", async () => {
    servicesState.services = [svc({ id: "hello-go", name: "hello-go" })];
    databasesState.databases = [db({ id: "shop-db", name: "shop-db" })];
    keyValuesState.keyValues = [kv({ id: "sessions-cache", name: "sessions-cache" })];

    renderHomePage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("hello-go")).toBeInTheDocument();
    expect(within(table).getByText("shop-db")).toBeInTheDocument();
    expect(within(table).getByText("sessions-cache")).toBeInTheDocument();

    // one Type badge per row, telling the three kinds apart in the merged table
    expect(within(table).getByText("Service")).toBeInTheDocument();
    expect(within(table).getByText("Database")).toBeInTheDocument();
    expect(within(table).getAllByText("Key Value")).toHaveLength(1);

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

  it("groups resources under their project section", async () => {
    servicesState.services = [svc({ id: "grouped-svc", name: "grouped-svc" })];
    databasesState.databases = [db({ id: "ungrouped-db", name: "ungrouped-db" })];
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

    expect(await screen.findByText(/storefront/)).toBeInTheDocument();
    expect(screen.getByText("grouped-svc")).toBeInTheDocument();
    expect(screen.getByText("ungrouped-db")).toBeInTheDocument();
    expect(screen.getByText("No Project")).toBeInTheDocument();
  });

  it("shows skeleton placeholders while loading with no data", async () => {
    servicesState.loading = true;
    renderHomePage();

    await screen.findByText("All Resources");
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

  it("shows an empty state when there are no resources", async () => {
    renderHomePage();

    expect(await screen.findByText("No resources yet")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
