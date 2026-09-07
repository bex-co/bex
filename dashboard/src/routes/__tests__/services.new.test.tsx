import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  HeadContent,
  Outlet,
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NewServicePage } from "../services.new";
import { translatedTitleHead } from "@/common/lib/document-head";
import {
  parseNewServiceSearch,
  serviceTypeCreateCopy,
} from "@/features/services/lib/create-context";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

// ── mocks ────────────────────────────────────────────────────────────────────

const instanceTypesState: {
  instanceTypes: InstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
  byID: (id: string | null | undefined) => InstanceTypeView | undefined;
} = {
  instanceTypes: [],
  loading: false,
  error: undefined,
  byID: () => undefined,
};

vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => instanceTypesState,
}));

vi.mock("@/features/services/hooks/use-repo-runtime-detection", () => ({
  useRepoRuntimeDetection: () => null,
}));

const create = vi.fn();
const clearNameConflict = vi.fn();
const createServiceState: { capLimit: string | null; nameConflict: boolean } = {
  capLimit: null,
  nameConflict: false,
};
vi.mock("@/features/services/hooks/use-create-service", () => ({
  useCreateService: () => ({
    create,
    busy: false,
    capLimit: createServiceState.capLimit,
    nameConflict: createServiceState.nameConflict,
    clearNameConflict,
  }),
}));

const nameAvailabilityState: {
  taken: boolean;
  suggestion: string | null;
  checking: boolean;
} = { taken: false, suggestion: null, checking: false };
vi.mock("@/features/services/hooks/use-service-name-availability", () => ({
  useServiceNameAvailability: () => nameAvailabilityState,
}));

const reposState: {
  repos: RepoView[];
  loading: boolean;
  error: Error | undefined;
} = { repos: [], loading: false, error: undefined };

vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({ ...reposState, refetch: vi.fn() }),
}));

const connectionState: {
  connections: Array<{
    accountLogin: string;
    installationId: number;
    createdAt: string;
    installUrl: string;
  }>;
  loading: boolean;
} = { connections: [], loading: false };

vi.mock("@/features/git/hooks/use-git-connection", () => ({
  // GitCredentialsMenu (w8/m31) reads the full list.
  useGitConnections: () => ({
    ...connectionState,
    connected: connectionState.connections.length > 0,
    error: undefined,
    refetch: vi.fn(),
  }),
  // ServiceSourcePicker reads the singular view for its disconnected gate.
  useGitConnection: () => {
    const first = connectionState.connections[0];
    return {
      connection: {
        connected: connectionState.connections.length > 0,
        accountLogin: first?.accountLogin ?? "",
        installUrl: first?.installUrl ?? "",
      },
      loading: connectionState.loading,
      refetch: vi.fn(),
    };
  },
}));

const connectGit = vi.fn();
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGit: () => ({ connect: connectGit, busy: false }),
}));
vi.mock("@/features/git/hooks/use-claim-git", () => ({
  useClaimGit: () => ({ claim: vi.fn(), busy: false }),
}));
vi.mock("@/features/git/hooks/use-disconnect-git", () => ({
  useDisconnectGit: () => ({ disconnect: vi.fn(), busy: false }),
}));

const PRIVATE_REGISTRY_CREDENTIAL: RegistryCredentialView = {
  id: "rgc-private",
  name: "Private GHCR",
  host: "ghcr.io",
  username: "robot",
  expiresAt: null,
  status: "active",
  createdAt: null,
};
const registryCredentialsState: {
  credentials: RegistryCredentialView[];
  loading: boolean;
} = {
  credentials: [PRIVATE_REGISTRY_CREDENTIAL],
  loading: false,
};

vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credentials",
  () => ({
    useRegistryCredentials: () => ({
      ...registryCredentialsState,
      error: undefined,
      refetch: vi.fn(),
    }),
  }),
);

const projectsState = {
  projects: [
    {
      id: "prj-platform",
      name: "Platform",
      ownerId: "tea-1",
      serviceIds: [],
      databaseIds: [],
      keyValueIds: [],
    },
  ],
};
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ ...projectsState, loading: false }),
}));

const environmentsState = {
  environments: [
    {
      id: "env-production",
      projectId: "prj-platform",
      name: "Production",
      ownerId: "tea-1",
      createdAt: null,
      serviceIds: [],
      databaseIds: [],
      keyValueIds: [],
      protectedStatus: "unprotected",
      networkIsolationEnabled: false,
      ipAllowList: [],
    },
  ],
};
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({ ...environmentsState, loading: false }),
}));

// ── fixtures ─────────────────────────────────────────────────────────────────

const FREE: InstanceTypeView = {
  id: "free",
  name: "Free",
  cpu: "0.1",
  memory: "512Mi",
  monthlyUsd: "0.00",
};
const STARTER: InstanceTypeView = {
  id: "starter",
  name: "Starter",
  cpu: "0.5",
  memory: "1Gi",
  monthlyUsd: "4.90",
};

const REPO: RepoView = {
  id: 1001,
  fullName: "acme-corp/web-frontend",
  private: false,
  defaultBranch: "main",
  htmlUrl: "https://github.com/acme-corp/web-frontend",
  cloneUrl: "https://github.com/acme-corp/web-frontend.git",
  accountLogin: "acme-corp",
};

// ── helpers ───────────────────────────────────────────────────────────────────

function renderPage(initialEntry = "/") {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: NewServicePage,
    validateSearch: parseNewServiceSearch,
  });
  const serviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => <div>service landing</div>,
  });
  const deployRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/deploys/$deployId",
    component: () => <div>deploy landing</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute, serviceRoute, deployRoute]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

/** Like renderPage, but with the real route's head() resolver and a root that
 *  renders HeadContent — so document.title tracks ?type= exactly as in the app
 *  (w6/045). */
function renderPageWithHead(initialEntry = "/") {
  const rootRoute = createRootRoute({
    component: () => (
      <>
        <HeadContent />
        <Outlet />
      </>
    ),
  });
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: NewServicePage,
    validateSearch: parseNewServiceSearch,
    // Same resolver as routes/services.new.tsx.
    head: ({ match }) =>
      translatedTitleHead(
        serviceTypeCreateCopy(match.search?.type).titleKey,
        match,
      ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

// ── setup ─────────────────────────────────────────────────────────────────────

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, STARTER];
  reposState.repos = [REPO];
  connectionState.connections = [
    {
      accountLogin: "acme-corp",
      installationId: 42,
      createdAt: "2026-08-29T00:00:00Z",
      installUrl: "https://github.com/settings/installations/42",
    },
  ];
  connectionState.loading = false;
  connectGit.mockReset();
  create.mockReset();
  create.mockResolvedValue({
    id: "srv-abc123",
    deployId: "dep-first",
  });
  clearNameConflict.mockReset();
  createServiceState.capLimit = null;
  createServiceState.nameConflict = false;
  nameAvailabilityState.taken = false;
  nameAvailabilityState.suggestion = null;
  nameAvailabilityState.checking = false;
  registryCredentialsState.credentials = [PRIVATE_REGISTRY_CREDENTIAL];
  registryCredentialsState.loading = false;
  projectsState.projects = [
    {
      id: "prj-platform",
      name: "Platform",
      ownerId: "tea-1",
      serviceIds: [],
      databaseIds: [],
      keyValueIds: [],
    },
  ];
  environmentsState.environments = [
    {
      id: "env-production",
      projectId: "prj-platform",
      name: "Production",
      ownerId: "tea-1",
      createdAt: null,
      serviceIds: [],
      databaseIds: [],
      keyValueIds: [],
      protectedStatus: "unprotected",
      networkIsolationEnabled: false,
      ipAllowList: [],
    },
  ];
});

// ── tests ─────────────────────────────────────────────────────────────────────

describe("NewServicePage", () => {
  describe("source tabs", () => {
    it("defaults to the GitHub tab and shows the repo list", async () => {
      renderPage();
      expect(
        await screen.findByPlaceholderText("Search repositories…"),
      ).toBeInTheDocument();
      expect(screen.getByText("acme-corp/web-frontend")).toBeInTheDocument();
    });

    it("switches to Public Git URL tab and shows the URL input", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Public Git URL/i }),
      );
      expect(
        screen.getByPlaceholderText("https://github.com/you/your-repo"),
      ).toBeInTheDocument();
    });

    it("switches to Existing Image tab and shows the image input", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Existing Image/i }),
      );
      expect(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
      ).toBeInTheDocument();
    });
  });

  // w8/m32: a static site builds from a Git repo (ADR029) — the Docker source
  // must be unreachable for it while the four image-valid types keep all
  // three tabs.
  describe("static site source gating", () => {
    it("offers no Existing Image tab on the static-site deep link", async () => {
      renderPage("/?type=static_site");
      expect(
        await screen.findByRole("tab", { name: /GitHub/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: /Public Git URL/i }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("tab", { name: /Existing Image/i }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByPlaceholderText("docker.io/library/nginx:latest"),
      ).not.toBeInTheDocument();
    });

    it("keeps the Existing Image tab for the four image-valid types", async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findAllByRole("radiogroup");
      for (const label of [
        /Web Service/i,
        /Private Service/i,
        /Background Worker/i,
        /Cron Job/i,
      ]) {
        await user.click(screen.getByRole("radio", { name: label }));
        expect(
          screen.getByRole("tab", { name: /Existing Image/i }),
        ).toBeInTheDocument();
      }
    });

    it("returns to the GitHub tab when Static Site is chosen from the image tab", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Existing Image/i }),
      );
      expect(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
      ).toBeInTheDocument();

      await user.click(screen.getByRole("radio", { name: /Static Site/i }));
      expect(
        screen.queryByRole("tab", { name: /Existing Image/i }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByPlaceholderText("docker.io/library/nginx:latest"),
      ).not.toBeInTheDocument();
      // The selection fell back to GitHub — the repo list is visible again.
      expect(
        screen.getByPlaceholderText("Search repositories…"),
      ).toBeInTheDocument();
    });
  });

  describe("repo picker", () => {
    it("auto-fills service name from selected repo slug", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      expect(screen.getByLabelText("Name")).toHaveValue("web-frontend");
    });

    it("shows connect prompt when not connected", async () => {
      connectionState.connections = [];
      renderPage();
      expect(
        await screen.findByText(/Connect your GitHub account/i),
      ).toBeInTheDocument();
      expect(
        screen.queryByPlaceholderText("Search repositories…"),
      ).not.toBeInTheDocument();
    });

    it("shows connect prompt when connection is null after load", async () => {
      connectionState.connections = [];
      renderPage();
      expect(
        await screen.findByText(/Connect your GitHub account/i),
      ).toBeInTheDocument();
    });

    it("configures GitHub or connects another account without leaving the create flow", async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(
        await screen.findByRole("button", { name: /Credentials \(1\)/ }),
      );

      expect(screen.getByText("acme-corp")).toBeInTheDocument();
      const configure = screen.getByLabelText("Configure in GitHub");
      expect(configure).toHaveAttribute(
        "href",
        "https://github.com/settings/installations/42",
      );
      expect(configure).toHaveAttribute("target", "_blank");

      await user.click(
        screen.getByRole("button", { name: "Connect another account" }),
      );
      expect(connectGit).toHaveBeenCalledOnce();
    });
  });

  describe("form validation", () => {
    it("keeps Deploy disabled when no repo is selected", async () => {
      renderPage();
      const deploy = await screen.findByRole("button", {
        name: /Deploy Service/i,
      });
      expect(deploy).toBeDisabled();
    });

    it("keeps Deploy disabled when name is invalid", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      const nameInput = screen.getByLabelText("Name");
      await user.clear(nameInput);
      await user.type(nameInput, "-invalid");
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeDisabled();
    });

    it("enables Deploy when repo is selected and name is valid", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeEnabled();
    });

    it("re-enables auto-fill when name is cleared and a new repo is selected", async () => {
      const REPO2: RepoView = {
        id: 1002,
        fullName: "acme-corp/api-service",
        private: true,
        defaultBranch: "main",
        htmlUrl: "https://github.com/acme-corp/api-service",
        cloneUrl: "https://github.com/acme-corp/api-service.git",
        accountLogin: "acme-corp",
      };
      reposState.repos = [REPO, REPO2];
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      const nameInput = screen.getByLabelText("Name");
      // Clear the auto-filled name → nameEdited resets to false
      await user.clear(nameInput);
      // Selecting a different repo now re-triggers auto-fill
      await user.click(
        screen.getByRole("button", { name: /acme-corp\/api-service/ }),
      );
      expect(nameInput).toHaveValue("api-service");
    });
  });

  describe("name availability (w4/m19)", () => {
    it("shows 'Name is already in use' with a working suggestion that fills the name on accept", async () => {
      nameAvailabilityState.taken = true;
      nameAvailabilityState.suggestion = "web-1";
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );

      expect(screen.getByText("Name is already in use")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeDisabled();

      await user.click(screen.getByRole("button", { name: "Use web-1" }));
      expect(screen.getByLabelText("Name")).toHaveValue("web-1");

      // The mocked check is static (doesn't recompute per keystroke, unlike
      // the real debounced hook) — flip it to free and force one more render
      // to see accepting the suggestion actually clears the blocked state.
      nameAvailabilityState.taken = false;
      nameAvailabilityState.suggestion = null;
      await user.type(screen.getByLabelText("Name"), "!");
      await user.keyboard("{Backspace}");
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeEnabled();
    });

    it("maps a raced create-mutation conflict to the same inline message", async () => {
      createServiceState.nameConflict = true;
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      expect(screen.getByText("Name is already in use")).toBeInTheDocument();
    });
  });

  describe("plan picker", () => {
    it("renders a card per catalog tier, defaulting to the first", async () => {
      renderPage();
      // Page now has two radiogroups: type picker and plan picker.
      await screen.findAllByRole("radiogroup");
      expect(screen.getByText("Free")).toBeInTheDocument();
      expect(screen.getByText("Starter")).toBeInTheDocument();
      expect(screen.getByText("$0.00/month")).toBeInTheDocument();
      expect(screen.getByText("$4.90/month")).toBeInTheDocument();
      expect(screen.getByRole("radio", { name: /Free/i })).toHaveAttribute(
        "aria-checked",
        "true",
      );
    });

    it("offers no Free tier for a background worker and defaults to the first paid tier (w6/025)", async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findAllByRole("radiogroup");
      await user.click(
        screen.getByRole("radio", { name: /Background Worker/i }),
      );
      expect(screen.queryByText("Free")).not.toBeInTheDocument();
      expect(screen.getByRole("radio", { name: /Starter/i })).toHaveAttribute(
        "aria-checked",
        "true",
      );
    });
  });

  describe("create flow", () => {
    it("preselects Project and Environment from validated URL context", async () => {
      const user = userEvent.setup();
      renderPage("/?projectId=prj-platform&environmentId=env-production");

      expect(
        await screen.findByRole("combobox", { name: "Project" }),
      ).toHaveTextContent("Platform");
      expect(
        screen.getByRole("combobox", { name: "Environment" }),
      ).toHaveTextContent("Production");
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ environmentId: "env-production" }),
      );
    });

    it("clears stale Project and Environment URL context", async () => {
      renderPage("/?projectId=prj-stale&environmentId=env-production");

      expect(
        await screen.findByRole("combobox", { name: "Project" }),
      ).toHaveTextContent("No project");
      expect(
        screen.getByRole("combobox", { name: "Environment" }),
      ).toBeDisabled();
    });

    it("calls create with repo and selected plan, then resolves", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("radio", { name: /Starter/i }));
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "web-frontend",
          repo: "https://github.com/acme-corp/web-frontend.git",
          plan: "starter",
          autoDeploy: true,
          runtime: "node",
          buildCommand: "npm install",
          startCommand: "npm start",
        }),
      );
    });

    it("submits a paid plan for a background worker even after Free was picked under another type (w6/025)", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("radio", { name: /Free/i }));
      await user.click(
        screen.getByRole("radio", { name: /Background Worker/i }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "background_worker",
          plan: "starter",
        }),
      );
    });

    it("submits the selected Render runtime and commands", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("combobox", { name: /Runtime/i }));
      await user.click(screen.getByRole("option", { name: "Python 3" }));
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          runtime: "python",
          buildCommand: "pip install -r requirements.txt",
          startCommand: "gunicorn app:app",
        }),
      );
    });

    it("shows Docker-only fields and submits Dockerfile Path plus Docker Command", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("combobox", { name: /Runtime/i }));
      await user.click(screen.getByRole("option", { name: "Docker" }));

      expect(screen.getByLabelText("Dockerfile Path")).toBeInTheDocument();
      expect(screen.getByLabelText("Docker Command")).toBeInTheDocument();
      expect(screen.getByLabelText("Registry credential")).toBeInTheDocument();
      expect(screen.queryByLabelText("Build Command")).not.toBeInTheDocument();
      await user.type(
        screen.getByLabelText("Dockerfile Path"),
        "docker/Dockerfile.prod",
      );
      await user.type(screen.getByLabelText("Docker Command"), "bin/server");
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          runtime: "docker",
          dockerfilePath: "docker/Dockerfile.prod",
          startCommand: "bin/server",
        }),
      );
    });

    it("submits the selected registry credential for a private Dockerfile base image", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("combobox", { name: /Runtime/i }));
      await user.click(screen.getByRole("option", { name: "Docker" }));
      await user.click(
        screen.getByRole("combobox", { name: /Registry credential/i }),
      );
      await user.click(
        screen.getByRole("option", { name: /Private GHCR \(ghcr.io\)/i }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          repo: "https://github.com/acme-corp/web-frontend.git",
          runtime: "docker",
          registryCredentialId: "rgc-private",
        }),
      );
    });

    it("submits create-time secret files and environment assignment", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("combobox", { name: "Project" }));
      await user.click(screen.getByRole("option", { name: "Platform" }));
      await user.click(screen.getByRole("combobox", { name: "Environment" }));
      await user.click(screen.getByRole("option", { name: "Production" }));
      await user.click(screen.getByRole("button", { name: "Add Secret File" }));
      await user.type(
        screen.getByRole("textbox", { name: "Secret file name" }),
        "credentials.json",
      );
      await user.type(
        screen.getByRole("textbox", { name: "Secret file contents" }),
        "secret-content",
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          environmentId: "env-production",
          secretFiles: [
            { name: "credentials.json", content: "secret-content" },
          ],
        }),
      );
    });

    it("clears a stale Dockerfile Path when switching back to a native runtime", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      const runtime = screen.getByRole("combobox", { name: /Runtime/i });
      await user.click(runtime);
      await user.click(screen.getByRole("option", { name: "Docker" }));
      await user.type(
        screen.getByLabelText("Dockerfile Path"),
        "docker/Dockerfile.stale",
      );

      await user.click(runtime);
      await user.click(screen.getByRole("option", { name: "Node" }));
      expect(
        screen.queryByLabelText("Dockerfile Path"),
      ).not.toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ runtime: "node", dockerfilePath: undefined }),
      );
    });
    it("shows the image field but no branch/autoDeploy when on Existing Image tab", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Existing Image/i }),
      );
      expect(screen.queryByLabelText("Branch")).not.toBeInTheDocument();
      expect(screen.queryByLabelText(/Auto-deploy/i)).not.toBeInTheDocument();
      expect(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
      ).toBeInTheDocument();
    });

    it("links to credential settings when the workspace has no stored credentials", async () => {
      registryCredentialsState.credentials = [];
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Existing Image/i }),
      );

      expect(
        screen.getByText("No stored registry credentials are available."),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Manage registry credentials" }),
      ).toHaveAttribute("href", "/settings#registry-credentials");
    });

    it("keeps the credential selector hidden for a native Git source", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );

      expect(
        screen.queryByRole("combobox", { name: /Registry credential/i }),
      ).not.toBeInTheDocument();
    });

    it("submits the selected registry credential with an existing image", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Existing Image/i }),
      );
      await user.type(screen.getByLabelText("Name"), "private-web");
      await user.type(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
        "ghcr.io/acme/private-web:latest",
      );
      await user.click(
        screen.getByRole("combobox", { name: /Registry credential/i }),
      );
      await user.click(
        screen.getByRole("option", { name: /Private GHCR \(ghcr.io\)/i }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          image: "ghcr.io/acme/private-web:latest",
          registryCredentialId: "rgc-private",
        }),
      );
    });

    it("submits an explicit no-credential choice for a public image", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("tab", { name: /Existing Image/i }),
      );
      await user.type(screen.getByLabelText("Name"), "public-web");
      await user.type(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
        "docker.io/library/nginx:latest",
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          image: "docker.io/library/nginx:latest",
          registryCredentialId: "",
        }),
      );
    });
  });

  describe("service type picker", () => {
    it("renders all five service types", async () => {
      renderPage();
      await screen.findAllByRole("radiogroup");
      expect(
        screen.getByRole("radio", { name: /Web Service/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Private Service/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Background Worker/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Cron Job/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Static Site/i }),
      ).toBeInTheDocument();
    });

    it("defaults to Web Service selected", async () => {
      renderPage();
      await screen.findAllByRole("radiogroup");
      expect(
        screen.getByRole("radio", { name: /Web Service/i }),
      ).toHaveAttribute("aria-checked", "true");
    });

    it("preselects the type from ?type= (Render /cron/new deep link)", async () => {
      renderPage("/?type=cron_job");
      await screen.findAllByRole("radiogroup");
      expect(screen.getByRole("radio", { name: /Cron Job/i })).toHaveAttribute(
        "aria-checked",
        "true",
      );
      // the cron-only Schedule field is shown with no click needed
      expect(screen.getByLabelText("Schedule")).toBeInTheDocument();
    });

    it("shows cron-specific chrome and a pre-filled schedule on the cron deep link", async () => {
      renderPage("/?type=cron_job");
      await screen.findAllByRole("radiogroup");
      // Heading + subtitle read as a cron form, not the generic New Service.
      expect(
        screen.getByRole("heading", { level: 1, name: "New Cron Job" }),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Run a command on a recurring schedule."),
      ).toBeInTheDocument();
      // Schedule is pre-filled with a valid default (Render parity).
      expect(screen.getByLabelText("Schedule")).toHaveValue("*/5 * * * *");
    });

    it("preselects static site from ?type=static_site", async () => {
      renderPage("/?type=static_site");
      await screen.findAllByRole("radiogroup");
      expect(
        screen.getByRole("radio", { name: /Static Site/i }),
      ).toHaveAttribute("aria-checked", "true");
    });

    // The unknown-?type= fallback to web_service is enforced by the route's
    // validateSearch (parseNewServiceSearch drops it → undefined → default);
    // that drop is unit-tested in create-context.test.ts. The test harness
    // mounts this component under a stand-in route, so its Route.useSearch()
    // reads raw (unvalidated) search and can't faithfully model the drop here.
  });

  // w6/m43 t003: the heading/subtitle used to branch only on cron, so the
  // other four types all read "New Service / Deploy a web service …" — the
  // page contradicting the type its own picker showed as selected.
  describe("per-type page heading and subtitle", () => {
    const CASES: {
      label: RegExp;
      heading: string;
      subtitle: string;
    }[] = [
      {
        label: /Web Service/i,
        heading: "New Web Service",
        subtitle: "Deploy a web service from a Git repo or Docker image.",
      },
      {
        label: /Private Service/i,
        heading: "New Private Service",
        subtitle: "Deploy a private service from a Git repo or Docker image.",
      },
      {
        label: /Background Worker/i,
        heading: "New Background Worker",
        subtitle: "Deploy a background worker from a Git repo or Docker image.",
      },
      {
        label: /Cron Job/i,
        heading: "New Cron Job",
        subtitle: "Run a command on a recurring schedule.",
      },
      {
        label: /Static Site/i,
        heading: "New Static Site",
        subtitle: "Build and deploy a static site from a Git repo.",
      },
    ];

    for (const { label, heading, subtitle } of CASES) {
      it(`names the type in the heading and subtitle: ${heading}`, async () => {
        const user = userEvent.setup();
        renderPage();
        await screen.findAllByRole("radiogroup");
        await user.click(screen.getByRole("radio", { name: label }));
        expect(
          screen.getByRole("heading", { level: 1, name: heading }),
        ).toBeInTheDocument();
        expect(screen.getByText(subtitle)).toBeInTheDocument();
      });
    }

    it("never describes a non-web type as a web service", async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findAllByRole("radiogroup");
      await user.click(
        screen.getByRole("radio", { name: /Background Worker/i }),
      );
      expect(
        screen.queryByText(
          "Deploy a web service from a Git repo or Docker image.",
        ),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("heading", { level: 1, name: "New Service" }),
      ).not.toBeInTheDocument();
    });
  });

  // w6/045: w6/m43 made the tab title and <h1> agree on initial load, but
  // clicking a Service Type radio only updated in-memory state — head() reads
  // match.search.type, so document.title stayed frozen at the load-time type.
  // The radio's onChange now mirrors the choice into ?type= (replace: true).
  describe("tab title follows the Service Type radio", () => {
    it("updates ?type=, the tab title, and the heading together on a radio click", async () => {
      const user = userEvent.setup();
      const { router } = renderPageWithHead();
      await screen.findAllByRole("radiogroup");
      await waitFor(() =>
        expect(document.title).toBe("New Web Service ・ bex Dashboard"),
      );

      await user.click(screen.getByRole("radio", { name: /Private Service/i }));

      expect(
        screen.getByRole("heading", { level: 1, name: "New Private Service" }),
      ).toBeInTheDocument();
      await waitFor(() =>
        expect(router.state.location.search).toMatchObject({
          type: "private_service",
        }),
      );
      await waitFor(() =>
        expect(document.title).toBe("New Private Service ・ bex Dashboard"),
      );
    });

    it("replaces history and keeps other search params on a type switch", async () => {
      const user = userEvent.setup();
      const { router } = renderPageWithHead(
        "/?projectId=prj-platform&environmentId=env-production",
      );
      await screen.findAllByRole("radiogroup");

      await user.click(screen.getByRole("radio", { name: /Cron Job/i }));

      await waitFor(() =>
        expect(router.state.location.search).toMatchObject({
          type: "cron_job",
          projectId: "prj-platform",
          environmentId: "env-production",
        }),
      );
      // replace: true — no extra history entry for the same form page.
      expect(router.history.length).toBe(1);
    });

    it("keeps in-progress form state across a type switch (no remount)", async () => {
      const user = userEvent.setup();
      const { router } = renderPageWithHead();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      expect(screen.getByLabelText("Name")).toHaveValue("web-frontend");

      await user.click(
        screen.getByRole("radio", { name: /Background Worker/i }),
      );

      await waitFor(() =>
        expect(router.state.location.search).toMatchObject({
          type: "background_worker",
        }),
      );
      // The selected repo and derived name survive the search-param update.
      expect(screen.getByLabelText("Name")).toHaveValue("web-frontend");
      expect(
        screen.getByRole("radio", { name: /Background Worker/i }),
      ).toHaveAttribute("aria-checked", "true");
    });
  });

  // w6/m43 t001: the Existing Image tab's "$PORT" hint rendered for every
  // service type, directly contradicting the "no public URL" note shown a few
  // fields below it on the same form for a worker/cron.
  describe("Existing Image port hint", () => {
    const PORT_HINT = /must listen on \$PORT/i;

    async function selectTypeThenImageTab(label: RegExp) {
      const user = userEvent.setup();
      renderPage();
      await screen.findAllByRole("radiogroup");
      await user.click(screen.getByRole("radio", { name: label }));
      await user.click(screen.getByRole("tab", { name: /Existing Image/i }));
    }

    it("shows the hint for a web service, which does bind a port", async () => {
      await selectTypeThenImageTab(/Web Service/i);
      expect(screen.getByText(PORT_HINT)).toBeInTheDocument();
    });

    it("shows the hint for a private service, which also binds a port", async () => {
      await selectTypeThenImageTab(/Private Service/i);
      expect(screen.getByText(PORT_HINT)).toBeInTheDocument();
    });

    it("hides the hint for a background worker, which never gets a port", async () => {
      await selectTypeThenImageTab(/Background Worker/i);
      expect(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
      ).toBeInTheDocument();
      expect(screen.queryByText(PORT_HINT)).not.toBeInTheDocument();
    });

    it("hides the hint for a cron job, which never gets a port either", async () => {
      await selectTypeThenImageTab(/Cron Job/i);
      expect(screen.queryByText(PORT_HINT)).not.toBeInTheDocument();
    });
  });

  describe("per-type conditional fields", () => {
    it("shows schedule and command fields for cron job", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      expect(screen.getByLabelText("Schedule")).toBeInTheDocument();
      expect(screen.getByLabelText("Start Command")).toBeInTheDocument();
    });

    it("shows publish directory for static site and hides plan picker", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("radio", { name: /Static Site/i }),
      );
      expect(screen.getByLabelText("Publish Directory")).toBeInTheDocument();
      expect(screen.queryByText("Free")).not.toBeInTheDocument();
    });

    it("shows no-public-url note for private service", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("radio", { name: /Private Service/i }),
      );
      expect(screen.getByText(/no public URL/i)).toBeInTheDocument();
    });

    it("shows no-public-url note for background worker", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("radio", { name: /Background Worker/i }),
      );
      expect(screen.getByText(/no public URL/i)).toBeInTheDocument();
    });

    it("hides schedule, command, and publish-path for web service (default)", async () => {
      renderPage();
      await screen.findAllByRole("radiogroup");
      expect(screen.queryByLabelText("Schedule")).not.toBeInTheDocument();
      expect(screen.queryByLabelText("Command")).not.toBeInTheDocument();
      expect(
        screen.queryByLabelText("Publish Directory"),
      ).not.toBeInTheDocument();
    });
  });

  describe("cron job validation", () => {
    it("keeps Deploy disabled when schedule is empty for cron job", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeDisabled();
    });

    it("keeps Deploy disabled and shows error when schedule has fewer than 5 fields", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.type(screen.getByLabelText("Schedule"), "* * *");
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeDisabled();
      expect(
        screen.getByText(/valid 5-field cron expression/i),
      ).toBeInTheDocument();
    });

    it("keeps Deploy disabled and shows error for a 5-field schedule with out-of-range values", async () => {
      // The 99 99 * * * bug: 5 fields but minute/hour out of range. A
      // field-count-only check let this through to the operator, which flipped
      // the service to Failed. The form must refuse it up front.
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.type(screen.getByLabelText("Schedule"), "99 99 * * *");
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeDisabled();
      expect(
        screen.getByText(/valid 5-field cron expression/i),
      ).toBeInTheDocument();
    });

    it("shows a human-readable preview for a valid schedule", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      await user.type(screen.getByLabelText("Schedule"), "0 0 * * *");
      expect(
        screen.getByText(/Every day at 00:00 · runs in UTC/i),
      ).toBeInTheDocument();
    });

    it("enables Deploy when schedule and start command are valid", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.type(screen.getByLabelText("Schedule"), "0 0 * * *");
      await user.type(screen.getByLabelText("Start Command"), "npm run job");
      expect(
        screen.getByRole("button", { name: /Deploy Service/i }),
      ).toBeEnabled();
    });

    it("submits with type=cron_job, schedule, command but no publishPath", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("radio", { name: /Cron Job/i }));
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.type(screen.getByLabelText("Schedule"), "0 0 * * *");
      await user.type(screen.getByLabelText("Start Command"), "python job.py");
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "cron_job",
          schedule: "0 0 * * *",
          command: "python job.py",
          runtime: "node",
          buildCommand: "npm install",
          startCommand: "python job.py",
        }),
      );
      const callArg = create.mock.calls[0][0] as Record<string, unknown>;
      expect(callArg.publishPath).toBeUndefined();
    });

    it("submits with type=static_site and publishPath but no schedule or command", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("radio", { name: /Static Site/i }),
      );
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.type(screen.getByLabelText("Publish Directory"), "dist");
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "static_site",
          publishPath: "dist",
        }),
      );
      const callArg = create.mock.calls[0][0] as Record<string, unknown>;
      expect(callArg.schedule).toBeUndefined();
      expect(callArg.command).toBeUndefined();
    });

    it("submits type=web_service by default with no schedule, command, or publishPath", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ type: "web_service" }),
      );
      const callArg = create.mock.calls[0][0] as Record<string, unknown>;
      expect(callArg.schedule).toBeUndefined();
      expect(callArg.publishPath).toBeUndefined();
    });

    it("lands a successful create on its first deploy detail page", async () => {
      const user = userEvent.setup();
      const { router } = renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(await screen.findByText("deploy landing")).toBeInTheDocument();
      expect(router.state.location.pathname).toBe(
        "/services/srv-abc123/deploys/dep-first",
      );
    });

    it("falls back to the service page when create has no deploy record", async () => {
      create.mockResolvedValueOnce({
        id: "srv-local",
        deployId: null,
      });
      const user = userEvent.setup();
      const { router } = renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(await screen.findByText("service landing")).toBeInTheDocument();
      expect(router.state.location.pathname).toBe("/services/srv-local");
    });

    it("does not navigate when create fails", async () => {
      create.mockResolvedValueOnce(null);
      const user = userEvent.setup();
      const { router } = renderPage();
      await user.click(
        await screen.findByRole("button", { name: /acme-corp\/web-frontend/ }),
      );
      await user.click(screen.getByRole("button", { name: /Deploy Service/i }));

      expect(create).toHaveBeenCalled();
      expect(router.state.location.pathname).toBe("/");
      expect(screen.getByLabelText("Name")).toHaveValue("web-frontend");
    });
  });
});
