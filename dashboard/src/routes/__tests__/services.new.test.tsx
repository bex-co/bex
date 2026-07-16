import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NewServicePage } from "../services.new";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";
import type { RepoView } from "@/features/services/hooks/use-repos";

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
  useRepos: () => reposState,
}));

const connectionState: {
  connection: {
    connected: boolean;
    accountLogin: string;
    installUrl: string;
  } | null;
  loading: boolean;
} = { connection: null, loading: false };

vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => connectionState,
}));

const registryCredentialsState = {
  credentials: [
    {
      id: "rgc-private",
      name: "Private GHCR",
      host: "ghcr.io",
      username: "robot",
      expiresAt: null,
      status: "active",
      createdAt: null,
    },
  ],
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
};
const STARTER: InstanceTypeView = {
  id: "starter",
  name: "Starter",
  cpu: "0.5",
  memory: "1Gi",
};

const REPO: RepoView = {
  id: 1001,
  fullName: "acme-corp/web-frontend",
  private: false,
  defaultBranch: "main",
  htmlUrl: "https://github.com/acme-corp/web-frontend",
  cloneUrl: "https://github.com/acme-corp/web-frontend.git",
};

// ── helpers ───────────────────────────────────────────────────────────────────

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: NewServicePage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

// ── setup ─────────────────────────────────────────────────────────────────────

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, STARTER];
  reposState.repos = [REPO];
  connectionState.connection = {
    connected: true,
    accountLogin: "acme-corp",
    installUrl: "",
  };
  connectionState.loading = false;
  create.mockReset();
  create.mockResolvedValue("srv-abc123");
  clearNameConflict.mockReset();
  createServiceState.capLimit = null;
  createServiceState.nameConflict = false;
  nameAvailabilityState.taken = false;
  nameAvailabilityState.suggestion = null;
  nameAvailabilityState.checking = false;
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
      connectionState.connection = {
        connected: false,
        accountLogin: "",
        installUrl: "https://github.com/apps/bex/installations/new",
      };
      renderPage();
      expect(
        await screen.findByText(/Connect your GitHub account/i),
      ).toBeInTheDocument();
      expect(
        screen.queryByPlaceholderText("Search repositories…"),
      ).not.toBeInTheDocument();
    });

    it("shows connect prompt when connection is null after load", async () => {
      connectionState.connection = null;
      renderPage();
      expect(
        await screen.findByText(/Connect your GitHub account/i),
      ).toBeInTheDocument();
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
      expect(screen.getByRole("radio", { name: /Free/i })).toHaveAttribute(
        "aria-checked",
        "true",
      );
    });
  });

  describe("create flow", () => {
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
  });
});
