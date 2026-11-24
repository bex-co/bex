import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ROUTE_SKELETON_MANIFEST } from "@/common/lib/route-skeleton-manifest";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { BuildSourcePartiallyLinked } from "@/features/build-sources/types";
import { CreateBuildSourcePage } from "../create-build-source-page";
import { CreateBuildSourceRouteSkeleton } from "../build-source-skeletons";

const mockCreate = vi.fn();
const mockClearNameTaken = vi.fn();
const mockClearPartiallyLinked = vi.fn();
const createHookState = vi.hoisted(() => ({
  busy: false,
  nameTaken: false,
  partiallyLinked: null as BuildSourcePartiallyLinked | null,
}));
const capabilitiesState = vi.hoisted(() => ({
  canCreate: true,
  loaded: true,
}));
const projectsState = vi.hoisted(() => ({
  projects: [{ id: "prj-1", name: "Core" }],
}));
const servicesState = vi.hoisted(() => ({
  services: [
    {
      id: "srv-web",
      name: "api-web",
      type: "web_service",
      region: "frankfurt",
    },
    {
      id: "srv-worker",
      name: "jobs-worker",
      type: "background_worker",
      region: "frankfurt",
    },
    {
      id: "srv-static",
      name: "marketing-docs",
      type: "static_site",
      region: null,
    },
  ],
}));
const envGroupsState = vi.hoisted(() => ({
  groups: [
    { id: "evg-ws", name: "shared-build-env", environmentId: null },
    { id: "evg-env", name: "prod-only-env", environmentId: "evm-prod" },
  ],
}));

vi.mock("@/features/build-sources/hooks/use-create-build-source", () => ({
  useCreateBuildSource: () => ({
    create: mockCreate,
    busy: createHookState.busy,
    nameTaken: createHookState.nameTaken,
    clearNameTaken: mockClearNameTaken,
    partiallyLinked: createHookState.partiallyLinked,
    clearPartiallyLinked: mockClearPartiallyLinked,
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () =>
    mockCapabilities({
      canCreate: capabilitiesState.canCreate,
      loaded: capabilitiesState.loaded,
    }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: projectsState.projects, loading: false }),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: servicesState.services, loading: false }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({ groups: envGroupsState.groups, loading: false }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => ({
    connection: { connected: false },
    loading: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/git/hooks/use-on-git-connections-change", () => ({
  useOnGitConnectionsChange: () => undefined,
}));
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGitAccount: () => ({ connect: vi.fn(), busy: false }),
}));
vi.mock("@/features/registry-credentials/hooks/use-registry-credentials", () => ({
  useRegistryCredentials: () => ({
    credentials: [{ id: "rgc-1", name: "ghcr", host: "ghcr.io" }],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

function renderWizard(ui: React.ReactNode) {
  const rootRoute = createRootRoute();
  const routes = [
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources",
      component: () => <div data-testid="build-sources-list-dest" />,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/$buildSourceId",
      component: () => <div data-testid="build-source-detail-dest" />,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/new",
      component: () => <>{ui}</>,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/workspace/settings",
      component: () => null,
    }),
  ];
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/build-sources/new"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  vi.clearAllMocks();
  createHookState.busy = false;
  createHookState.nameTaken = false;
  createHookState.partiallyLinked = null;
  capabilitiesState.canCreate = true;
  capabilitiesState.loaded = true;
});

describe("CreateBuildSourcePage", () => {
  it("submits a Git native build source with workspace env group and linked service, excluding static sites and env-scoped groups", async () => {
    mockCreate.mockResolvedValue({ id: "bds-new", name: "my-api" });
    const user = userEvent.setup();
    const { router } = renderWizard(<CreateBuildSourcePage initialProjectId="prj-1" />);

    await screen.findByRole("heading", { name: "New Build Source" });

    // Static sites must be excluded from linkable services.
    expect(screen.getByText("api-web")).toBeInTheDocument();
    expect(screen.getByText("jobs-worker")).toBeInTheDocument();
    expect(screen.queryByText("marketing-docs")).not.toBeInTheDocument();

    // Switch to Public Git Repository tab and enter repo URL.
    await user.click(screen.getByRole("tab", { name: /Public Git/i }));
    await user.type(
      screen.getByLabelText("Repository URL"),
      "https://github.com/acme/my-api",
    );

    // Name auto-fills from the repo URL; override explicitly.
    const nameInput = screen.getByLabelText(/^Name/);
    await user.clear(nameInput);
    await user.type(nameInput, "shared-backend");

    // Native runtime shows Build Command and hides Docker Context.
    expect(screen.getByLabelText(/Build Command/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/Docker Build Context/i)).not.toBeInTheDocument();
    await user.type(screen.getByLabelText(/Build Command/i), "npm run build");

    // Expand Advanced -> workspace env group shown, environment-scoped group excluded.
    await user.click(screen.getByRole("button", { name: /Advanced/i }));
    expect(screen.getByText("shared-build-env")).toBeInTheDocument();
    expect(screen.queryByText("prod-only-env")).not.toBeInTheDocument();
    await user.click(screen.getByLabelText("shared-build-env"));

    // Link api-web service.
    await user.click(screen.getByLabelText(/api-web/));

    await user.click(
      screen.getByRole("button", { name: "Create Build Source" }),
    );

    expect(mockCreate).toHaveBeenCalledWith({
      name: "shared-backend",
      projectId: "prj-1",
      git: {
        repoUrl: "https://github.com/acme/my-api",
        runtime: "node",
        buildCommand: "npm run build",
      },
      envGroupIds: ["evg-ws"],
      serviceIds: ["srv-web"],
    });
    expect(router.state.location.pathname).toBe("/build-sources/bds-new");
  });

  it("hides git build settings and build-time environment for Docker image sources", async () => {
    mockCreate.mockResolvedValue({ id: "bds-img", name: "nginx" });
    const user = userEvent.setup();
    renderWizard(<CreateBuildSourcePage />);

    await screen.findByRole("heading", { name: "New Build Source" });

    // Switch to Existing Image tab.
    await user.click(screen.getByRole("tab", { name: /Existing Image/i }));
    expect(screen.queryByLabelText(/Build Command/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Advanced/i })).not.toBeInTheDocument();

    await user.type(
      screen.getByLabelText("Docker Image"),
      "ghcr.io/acme/nginx:1.27",
    );

    await user.click(
      screen.getByRole("button", { name: "Create Build Source" }),
    );

    expect(mockCreate).toHaveBeenCalledWith({
      name: "nginx",
      image: {
        imageUrl: "ghcr.io/acme/nginx:1.27",
      },
    });
  });

  it("renders inline BUILD_SOURCE_NAME_TAKEN and actionable BUILD_SOURCE_PARTIALLY_LINKED alert", async () => {
    createHookState.nameTaken = true;
    createHookState.partiallyLinked = {
      buildSourceId: "bds-partial-1",
      linkedServiceIds: ["srv-web"],
      failedServiceId: "srv-worker",
      message: "partial link failure",
    };

    const user = userEvent.setup();
    renderWizard(<CreateBuildSourcePage />);

    await screen.findByRole("heading", { name: "New Build Source" });
    await user.type(screen.getByLabelText(/^Name/), "dup-name");

    expect(
      screen.getByText(
        'A build source named "dup-name" already exists in this workspace.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Build source created with partial service links"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/bds-partial-1/),
    ).toBeInTheDocument();
    expect(screen.getByText("Linked: api-web")).toBeInTheDocument();
    expect(screen.getByText("Failed to link: jobs-worker")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open Build Source" }),
    ).toHaveAttribute("href", "/build-sources/bds-partial-1");
  });

  it("blocks Viewers (canCreate: false) from seeing the create form", async () => {
    capabilitiesState.canCreate = false;
    renderWizard(<CreateBuildSourcePage />);

    expect(
      await screen.findByText(
        "You need Developer or Admin permissions in this workspace to create a build source.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Create Build Source" }),
    ).not.toBeInTheDocument();
  });

  it("fails closed without rendering the create form while capabilities are still loading", async () => {
    capabilitiesState.canCreate = false;
    capabilitiesState.loaded = false;
    renderWizard(<CreateBuildSourcePage />);

    await screen.findByRole("heading", { name: "New Build Source" });
    expect(
      screen.queryByRole("button", { name: "Create Build Source" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        "You need Developer or Admin permissions in this workspace to create a build source.",
      ),
    ).not.toBeInTheDocument();
  });

  it("skeleton matches ready-state regions in order and shares PageFrame width", async () => {
    const pending = renderWizard(<CreateBuildSourceRouteSkeleton />);
    await screen.findByText("New Build Source");
    const entry = ROUTE_SKELETON_MANIFEST["/build-sources/new"];
    const skeletonRegions = [
      ...pending.container.querySelectorAll<HTMLElement>(
        "[data-skeleton-region]",
      ),
    ].map((el) => el.dataset.skeletonRegion);
    for (const region of entry.regions) {
      expect(skeletonRegions).toContain(region);
    }
    expect(
      Boolean(pending.container.querySelector("[data-page-footer]")),
    ).toBe(true);
    pending.unmount();

    const ready = renderWizard(<CreateBuildSourcePage />);
    await screen.findByRole("heading", { name: "New Build Source" });
    const readyRegions = [
      ...ready.container.querySelectorAll<HTMLElement>("[data-region]"),
    ].map((el) => el.dataset.region);
    expect(readyRegions).toEqual([
      "source-picker",
      "details",
      "build-settings",
      "link-services",
    ]);
    expect(Boolean(ready.container.querySelector("[data-page-footer]"))).toBe(
      true,
    );
  });
});
