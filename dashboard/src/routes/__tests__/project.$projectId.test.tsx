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
import { ProjectPage } from "../project.$projectId.index";
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

const rename = vi.fn();
vi.mock("@/features/projects/hooks/use-rename-project", () => ({
  useRenameProject: () => ({ rename, busy: false }),
}));

// The Environments panel is an Apollo client of its own (covered by its own
// tests); stub it here so this route test stays focused on the project page's
// resource table + rename, without needing an ApolloProvider.
vi.mock("@/features/environments/components/environments-panel", () => ({
  EnvironmentsPanel: ({
    projectRows,
  }: {
    projectRows: Array<{ name: string }>;
  }) => (
    <div data-testid="environments-panel">
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

function renderProjectPage(projectId = "prj-1") {
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
  const projectIndexRoute = createRoute({
    getParentRoute: () => projectRoute,
    path: "/",
    component: ProjectPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      projectRoute.addChildren([projectIndexRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: [`/project/${projectId}`] }),
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
  rename.mockReset();
});

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

  it("renders nothing for an unknown project id — the layout's redirect home is in flight (w9/m55)", async () => {
    renderProjectPage("prj-missing");

    await waitFor(() =>
      expect(
        screen.queryByTestId("environments-panel"),
      ).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  });

  it("renames the project via the inline pencil button", async () => {
    projectsState.projects = [
      {
        id: "prj-1",
        name: "storefront",
        ownerId: "tea-1",
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      },
    ];
    rename.mockResolvedValue(true);
    const user = userEvent.setup();

    renderProjectPage();

    await screen.findByRole("heading", { name: "storefront" });
    await user.click(screen.getByRole("button", { name: "Edit" }));

    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByRole("textbox");
    await user.clear(input);
    await user.type(input, "new-name");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(rename).toHaveBeenCalledWith("prj-1", "new-name");
  });
});
