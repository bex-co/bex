import { describe, it, expect, vi, beforeEach } from "vitest";
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
vi.mock("@/features/services/hooks/use-create-service", () => ({
  useCreateService: () => ({ create, busy: false }),
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
  connection: { connected: boolean; accountLogin: string; installUrl: string } | null;
  loading: boolean;
} = { connection: null, loading: false };

vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => connectionState,
}));

// ── fixtures ─────────────────────────────────────────────────────────────────

const FREE: InstanceTypeView = { id: "free", name: "Free", cpu: "0.1", memory: "512Mi" };
const STARTER: InstanceTypeView = { id: "starter", name: "Starter", cpu: "0.5", memory: "1Gi" };

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
  connectionState.connection = { connected: true, accountLogin: "acme-corp", installUrl: "" };
  connectionState.loading = false;
  create.mockReset();
  create.mockResolvedValue("srv-abc123");
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
      await user.click(await screen.findByRole("tab", { name: /Public Git URL/i }));
      expect(
        screen.getByPlaceholderText("https://github.com/you/your-repo"),
      ).toBeInTheDocument();
    });

    it("switches to Existing Image tab and shows the image input", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("tab", { name: /Existing Image/i }));
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
      const deploy = await screen.findByRole("button", { name: /Deploy Service/i });
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
      await user.type(nameInput, "1invalid");
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

  describe("plan picker", () => {
    it("renders a card per catalog tier, defaulting to the first", async () => {
      renderPage();
      const radiogroup = await screen.findByRole("radiogroup");
      expect(radiogroup).toBeInTheDocument();
      expect(screen.getByText("Free")).toBeInTheDocument();
      expect(screen.getByText("Starter")).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Free/i }),
      ).toHaveAttribute("aria-checked", "true");
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
      await user.click(
        screen.getByRole("button", { name: /Deploy Service/i }),
      );
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "web-frontend",
          repo: "https://github.com/acme-corp/web-frontend.git",
          plan: "starter",
          autoDeploy: true,
        }),
      );
    });

    it("shows the image field but no branch/autoDeploy when on Existing Image tab", async () => {
      const user = userEvent.setup();
      renderPage();
      await user.click(await screen.findByRole("tab", { name: /Existing Image/i }));
      expect(screen.queryByLabelText("Branch")).not.toBeInTheDocument();
      expect(
        screen.queryByLabelText(/Auto-deploy/i),
      ).not.toBeInTheDocument();
      expect(
        screen.getByPlaceholderText("docker.io/library/nginx:latest"),
      ).toBeInTheDocument();
    });
  });
});
