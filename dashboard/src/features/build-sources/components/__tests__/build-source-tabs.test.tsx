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
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { BuildSourceView } from "@/features/build-sources/types";
import { BuildSourceDetailContext } from "../build-source-detail-context";
import { BuildSourceServicesTab } from "../build-source-services-tab";
import { BuildSourceEnvironmentTab } from "../build-source-environment-tab";
import { BuildSourceSettingsTab } from "../build-source-settings-tab";

const capabilitiesState = vi.hoisted(() => ({
  canCreate: true,
  canViewSensitive: true,
}));
const linkMock = vi.hoisted(() => vi.fn(async () => ({ ok: true })));
const updateMock = vi.hoisted(() => vi.fn());
const removeMock = vi.hoisted(() => vi.fn(async () => true));
const updateState = vi.hoisted(() => ({
  nameTaken: false,
  busy: false,
  clearNameTaken: vi.fn(),
}));
const envSaveMock = vi.hoisted(() =>
  vi.fn(async () => ({ refreshFailed: false, rolledOut: false })),
);
const envGroupLinkMock = vi.hoisted(() => vi.fn(async () => true));
const envGroupUnlinkMock = vi.hoisted(() => vi.fn(async () => true));

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({
    refetchQueries: vi.fn(async () => []),
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () =>
    mockCapabilities({
      canCreate: capabilitiesState.canCreate,
      canViewSensitive: capabilitiesState.canViewSensitive,
    }),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [
      {
        id: "srv-linked",
        name: "api-web",
        type: "web_service",
        region: "frankfurt",
        autoDeploy: true,
        suspended: false,
        startCommand: "npm start",
      },
      {
        id: "srv-eligible",
        name: "worker-eligible",
        type: "background_worker",
        region: "frankfurt",
        autoDeploy: false,
        suspended: false,
        startCommand: "npm run worker",
      },
      {
        id: "srv-other-region",
        name: "oregon-svc",
        type: "web_service",
        region: "oregon",
        autoDeploy: true,
        suspended: false,
        startCommand: "npm start",
      },
      {
        id: "srv-static",
        name: "docs-site",
        type: "static_site",
        region: "frankfurt",
        autoDeploy: true,
        suspended: false,
      },
    ],
  }),
}));
vi.mock("@/features/services/hooks/use-link-build-source", () => ({
  useLinkBuildSource: () => ({
    linkBuildSource: linkMock,
    busy: false,
    lastError: null,
  }),
}));
vi.mock("@/features/services/hooks/use-env-vars", () => ({
  useEnvVarKeys: () => ({
    keys: [{ id: "API_TOKEN", key: "API_TOKEN" }],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));
vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: () => ({
    names: [{ id: "cert.pem", name: "cert.pem" }],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));
vi.mock("@/features/services/hooks/use-environment-draft-save", () => ({
  useEnvironmentDraftSave: () => ({
    save: envSaveMock,
    saving: false,
  }),
}));
vi.mock("@/features/build-sources/hooks/use-build-source-env", () => ({
  useRevealBuildSourceEnvVar: () => vi.fn(async () => "secret-val"),
  useRevealBuildSourceSecretFile: () => vi.fn(async () => "file-content"),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: [
      {
        id: "evg-ws-linked",
        name: "shared-build",
        ownerId: "tea-1",
        environmentId: null,
        serviceLinks: ["bds-git-1"],
        envVarKeys: ["NPM_TOKEN"],
        secretFileNames: [],
      },
      {
        id: "evg-ws-avail",
        name: "extra-build",
        ownerId: "tea-1",
        environmentId: null,
        serviceLinks: [],
        envVarKeys: ["CACHE_KEY"],
        secretFileNames: [],
      },
      {
        id: "evg-env-scoped",
        name: "prod-only",
        ownerId: "tea-1",
        environmentId: "evm-1",
        serviceLinks: [],
        envVarKeys: ["SECRET"],
        secretFileNames: [],
      },
    ],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
  useEnvGroupMutations: () => ({
    linkGroup: envGroupLinkMock,
    unlinkGroup: envGroupUnlinkMock,
    busy: false,
  }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: [{ id: "prj-1", name: "Core" }] }),
}));
vi.mock("@/features/services/components/registry-credential-select", () => ({
  RegistryCredentialSelect: () => <div data-testid="registry-cred-select" />,
}));
vi.mock("@/features/build-sources/hooks/use-update-build-source", () => ({
  useUpdateBuildSource: () => ({
    update: updateMock,
    busy: updateState.busy,
    nameTaken: updateState.nameTaken,
    clearNameTaken: updateState.clearNameTaken,
  }),
}));
vi.mock("@/features/build-sources/hooks/use-delete-build-source", () => ({
  useDeleteBuildSource: () => ({
    remove: removeMock,
    busy: false,
  }),
}));

const GIT_SOURCE: BuildSourceView = {
  id: "bds-git-1",
  name: "shared-api",
  ownerId: "tea-1",
  projectId: "prj-1",
  kind: "git",
  git: {
    repoUrl: "https://github.com/acme/shared-api",
    branch: "main",
    runtime: "node",
    region: "frankfurt",
    buildCommand: "npm run build",
    rootDir: null,
    dockerfilePath: null,
    dockerContext: null,
    registryCredentialId: null,
    buildFilter: { paths: ["src/**"], ignoredPaths: [] },
  },
  image: null,
  serviceLinks: [{ id: "srv-linked", name: "api-web" }],
  createdAt: "2026-11-23T00:00:00Z",
  updatedAt: "2026-11-24T00:00:00Z",
  cursor: "bds-git-1",
};

function renderTab(ui: React.ReactNode, source: BuildSourceView = GIT_SOURCE) {
  const rootRoute = createRootRoute();
  const routes = [
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => (
        <BuildSourceDetailContext.Provider
          value={{
            buildSource: source,
            refetch: vi.fn(async () => source),
            openTriggerBuild: vi.fn(),
          }}
        >
          {ui}
        </BuildSourceDetailContext.Provider>
      ),
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources",
      component: () => <div data-testid="list-route">List</div>,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/$buildSourceId",
      component: () => <div data-testid="detail-route">Detail</div>,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/project/$projectId",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId/settings",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/env-groups/$groupId",
      component: () => null,
    }),
  ];
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return { router, ...render(<RouterProvider router={router} />) };
}

beforeEach(() => {
  vi.clearAllMocks();
  capabilitiesState.canCreate = true;
  capabilitiesState.canViewSensitive = true;
  updateState.nameTaken = false;
  updateState.busy = false;
});

describe("BuildSourceServicesTab (w2/m614/t005)", () => {
  it("lists linked services and gates Link existing service on canCreate", async () => {
    renderTab(<BuildSourceServicesTab />);
    expect(await screen.findByRole("link", { name: "api-web" })).toHaveAttribute(
      "href",
      "/services/srv-linked",
    );
    expect(screen.getByText("Auto-deploy on")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Change source/i }),
    ).toHaveAttribute("href", "/services/srv-linked/settings");
  });

  it("disables Link existing service button for viewers", async () => {
    capabilitiesState.canCreate = false;
    renderTab(<BuildSourceServicesTab />);
    expect(
      await screen.findByRole("button", { name: "Link existing service" }),
    ).toBeDisabled();
  });

  it("links an eligible service with required startCommand for native git build sources", async () => {
    const user = userEvent.setup();
    renderTab(<BuildSourceServicesTab />);

    await user.click(
      await screen.findByRole("button", { name: "Link existing service" }),
    );
    await user.click(screen.getByRole("combobox", { name: "Service" }));
    // Static sites and already-linked services are excluded; different-region service is marked.
    expect(screen.queryByRole("option", { name: /docs-site/ })).toBeNull();
    expect(screen.queryByRole("option", { name: /api-web/ })).toBeNull();
    expect(
      screen.getByRole("option", { name: /oregon-svc/ }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("option", { name: /worker-eligible/ }));
    const startInput = screen.getByLabelText("Start Command");
    expect(startInput).toHaveValue("npm run worker");
    await user.clear(startInput);
    const submit = screen.getByRole("button", { name: "Link service" });
    expect(submit).toBeDisabled();

    await user.type(startInput, "node dist/worker.js");
    expect(submit).toBeEnabled();
    await user.click(submit);

    expect(linkMock).toHaveBeenCalledWith(
      "srv-eligible",
      "bds-git-1",
      "node dist/worker.js",
      false,
    );
  });
});

describe("BuildSourceEnvironmentTab (w2/m614/t005)", () => {
  it("renders save-only environment editor and workspace env groups (excluding env-scoped)", async () => {
    const user = userEvent.setup();
    renderTab(<BuildSourceEnvironmentTab />);

    expect(
      await screen.findByText(
        "Changes are saved without starting a build and apply on the next build.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("API_TOKEN")).toBeInTheDocument();
    expect(screen.getByText("cert.pem")).toBeInTheDocument();
    expect(screen.getByText("shared-build")).toBeInTheDocument();
    expect(screen.getByText("extra-build")).toBeInTheDocument();
    expect(screen.queryByText("prod-only")).toBeNull();

    // Enter edit mode and add a variable -> single "Save changes" button, no deploy/rebuild menu.
    await user.click(screen.getByRole("button", { name: /^Edit$/i }));
    await user.click(screen.getByRole("button", { name: /Add variable/i }));
    const saveBtn = screen.getByRole("button", { name: /Save changes/i });
    expect(saveBtn).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Save and deploy/i })).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Save, rebuild, and deploy/i }),
    ).toBeNull();

    // Link available workspace env group.
    await user.click(
      screen.getByRole("button", { name: /Link extra-build/i }),
    );
    expect(envGroupLinkMock).toHaveBeenCalledWith("evg-ws-avail", "bds-git-1");
  });

  it("gates environment group linking for viewers (canCreate: false)", async () => {
    capabilitiesState.canCreate = false;
    renderTab(<BuildSourceEnvironmentTab />);
    expect(
      await screen.findByRole("button", { name: /Link extra-build/i }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /Unlink shared-build/i }),
    ).toBeDisabled();
  });

  it("redirects image build sources away from /environment", async () => {
    const { router } = renderTab(<BuildSourceEnvironmentTab />, {
      ...GIT_SOURCE,
      id: "bds-img-1",
      kind: "image",
      git: null,
      image: {
        ownerId: "tea-1",
        imageUrl: "nginx:1.27",
        registryCredentialId: null,
      },
    });
    expect(await screen.findByTestId("detail-route")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/build-sources/bds-img-1");
  });
});

describe("BuildSourceSettingsTab (w2/m614/t006)", () => {
  it("submits updateBuildSource without region and blocks delete while services are linked", async () => {
    updateMock.mockResolvedValue({ ...GIT_SOURCE, name: "renamed-api" });
    const user = userEvent.setup();
    renderTab(<BuildSourceSettingsTab />);

    const nameInput = await screen.findByLabelText("Name");
    await user.clear(nameInput);
    await user.type(nameInput, "renamed-api");

    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(updateMock).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "bds-git-1",
        name: "renamed-api",
        git: expect.objectContaining({
          repoUrl: "https://github.com/acme/shared-api",
          branch: "main",
          runtime: "node",
        }),
      }),
    );
    // Region is immutable on git sources and must not be sent.
    expect(updateMock.mock.calls[0][0].git).not.toHaveProperty("region");

    // Delete is blocked while api-web is linked.
    expect(
      screen.getByTestId("delete-blocked-linked-services"),
    ).toHaveTextContent("api-web");
    expect(
      screen.getByRole("button", { name: "Delete Build Source" }),
    ).toBeDisabled();
  });

  it("requires sudo phrase confirmation and deletes cleanly once unlinked", async () => {
    const user = userEvent.setup();
    const { router } = renderTab(<BuildSourceSettingsTab />, {
      ...GIT_SOURCE,
      serviceLinks: [],
    });

    await user.click(
      await screen.findByRole("button", { name: "Delete Build Source" }),
    );
    const input = await screen.findByLabelText("Sudo Command");
    const confirmButtons = screen.getAllByRole("button", {
      name: "Delete Build Source",
    });
    const confirmButton = confirmButtons.at(-1)!;
    expect(confirmButton).toBeDisabled();

    await user.type(input, "sudo delete build source shared-api");
    expect(confirmButton).not.toBeDisabled();
    await user.click(confirmButton);

    expect(removeMock).toHaveBeenCalledWith("bds-git-1", "shared-api");
    expect(await screen.findByTestId("list-route")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/build-sources");
  });

  it("shows inline BUILD_SOURCE_NAME_TAKEN error and disables saving while taken", async () => {
    updateState.nameTaken = true;
    renderTab(<BuildSourceSettingsTab />);

    expect(
      await screen.findByText(
        'A build source named "shared-api" already exists in this workspace.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toBeDisabled();
  });

  it("updates Docker image build sources without git fields", async () => {
    const user = userEvent.setup();
    const imageSource: BuildSourceView = {
      ...GIT_SOURCE,
      id: "bds-img-1",
      name: "nginx-base",
      kind: "image",
      git: null,
      image: {
        ownerId: "tea-1",
        imageUrl: "ghcr.io/acme/nginx:1.27",
        registryCredentialId: null,
      },
      serviceLinks: [],
    };
    updateMock.mockResolvedValue({
      ...imageSource,
      image: {
        ownerId: "tea-1",
        imageUrl: "ghcr.io/acme/nginx:1.28",
        registryCredentialId: null,
      },
    });
    renderTab(<BuildSourceSettingsTab />, imageSource);

    const imageInput = await screen.findByLabelText("Image");
    await user.clear(imageInput);
    await user.type(imageInput, "ghcr.io/acme/nginx:1.28");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateMock).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "bds-img-1",
        image: {
          imageUrl: "ghcr.io/acme/nginx:1.28",
          registryCredentialId: "",
        },
      }),
    );
  });
});
