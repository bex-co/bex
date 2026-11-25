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
import { LOGS_REASON, mockCapabilities } from "@/test/mocks/capabilities";
import type {
  BuildSourceBuildView,
  BuildSourceView,
} from "@/features/build-sources/types";
import { BuildSourceDetailContext } from "../build-source-detail-context";
import { BuildSourceBuildsTab } from "../build-source-builds-tab";
import { BuildDetailPage } from "../build-detail-page";

const buildsListState = vi.hoisted(() => ({
  builds: [] as BuildSourceBuildView[],
  loading: false,
  loadingMore: false,
  hasMore: false,
  error: undefined as Error | undefined,
  loadMore: vi.fn(async () => undefined),
  refetch: vi.fn(async () => []),
}));

const singleBuildState = vi.hoisted(() => ({
  build: null as BuildSourceBuildView | null,
  loading: false,
  error: undefined as Error | undefined,
  notFound: false,
  refetch: vi.fn(async () => null),
}));

const buildLogsCalls = vi.hoisted(() => [] as Array<Record<string, unknown>>);
const capabilitiesState = vi.hoisted(() => ({
  canCreate: true,
  canViewLogs: true,
}));

vi.mock("@/features/build-sources/hooks/use-build-source-builds", () => ({
  useBuildSourceBuilds: () => buildsListState,
  useBuildSourceBuild: () => singleBuildState,
}));
vi.mock("@/features/build-sources/hooks/use-build-logs", () => ({
  useBuildLogs: (opts: Record<string, unknown>) => {
    buildLogsCalls.push(opts);
    return {
      lines: [
        {
          key: "log-1",
          id: "log-1",
          timestamp: "2026-11-24T02:00:05Z",
          message: "==> Build succeeded 🎉",
          resource: "bds-git-1",
          type: "build",
          instance: "bld-1",
          level: "info",
        },
      ],
      loading: false,
      error: undefined,
      buildStoreUnavailable: false,
      buildLiveStatus: "open",
      hasMore: false,
      loadingOlder: false,
      loadOlder: vi.fn(),
    };
  },
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [
      { id: "srv-1", name: "api-web" },
      { id: "srv-2", name: "jobs-worker" },
    ],
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () =>
    mockCapabilities({
      canCreate: capabilitiesState.canCreate,
      canViewLogs: capabilitiesState.canViewLogs,
    }),
}));

const GIT_SOURCE: BuildSourceView = {
  id: "bds-git-1",
  name: "shared-api",
  ownerId: "tea-1",
  projectId: null,
  kind: "git",
  git: {
    repoUrl: "https://github.com/acme/shared-api",
    branch: "main",
    runtime: "go",
    region: "frankfurt",
    buildCommand: "go build .",
    rootDir: null,
    dockerfilePath: null,
    dockerContext: null,
    registryCredentialId: null,
    buildFilter: null,
  },
  image: null,
  serviceLinks: [{ id: "srv-1", name: "api-web" }],
  createdAt: "2026-11-23T00:00:00Z",
  updatedAt: "2026-11-24T00:00:00Z",
  cursor: "bds-git-1",
};

const BUILD_ROW: BuildSourceBuildView = {
  id: "bld-1",
  buildSourceId: "bds-git-1",
  status: "succeeded",
  succeeded: true,
  commitId: "abcdef1234567890",
  commitUrl: "https://github.com/acme/shared-api/commit/abcdef1234567890",
  runtime: "go",
  buildStartedAt: "2026-11-24T02:00:00Z",
  buildFinishedAt: "2026-11-24T02:01:30Z",
  imageRef: "registry.bex.internal/tea-1/bds-git-1:bld-1",
  imageSha: "sha256:1234567890abcdef1234567890abcdef",
  registryCredentialId: null,
  deploys: [{ serviceId: "srv-1", deployId: "dep-10" }],
  skippedServices: [{ serviceId: "srv-2", reason: "suspended" }],
  createdAt: "2026-11-24T02:00:00Z",
  updatedAt: "2026-11-24T02:01:30Z",
  cursor: "dep-1",
};

function renderWithContext(
  ui: React.ReactNode,
  source: BuildSourceView = GIT_SOURCE,
  openTriggerBuild = vi.fn(),
) {
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
            openTriggerBuild,
          }}
        >
          {ui}
        </BuildSourceDetailContext.Provider>
      ),
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/$buildSourceId",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/$buildSourceId/builds/$buildId",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId/deploys/$deployId",
      component: () => null,
    }),
  ];
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  vi.clearAllMocks();
  buildsListState.builds = [];
  buildsListState.loading = false;
  buildsListState.hasMore = false;
  buildsListState.error = undefined;
  singleBuildState.build = null;
  singleBuildState.loading = false;
  singleBuildState.error = undefined;
  singleBuildState.notFound = false;
  buildLogsCalls.length = 0;
  capabilitiesState.canCreate = true;
  capabilitiesState.canViewLogs = true;
});

describe("BuildSourceBuildsTab & BuildDetailPage (w2/m614/t003-t004)", () => {
  it("lists builds with status, commit link, deploys count, and paginates via Load more", async () => {
    buildsListState.builds = [BUILD_ROW];
    buildsListState.hasMore = true;
    const user = userEvent.setup();
    renderWithContext(<BuildSourceBuildsTab />);

    const buildLinks = await screen.findAllByRole("link", { name: "bld-1" });
    expect(buildLinks[0]).toHaveAttribute(
      "href",
      "/build-sources/bds-git-1/builds/bld-1",
    );
    expect(screen.getAllByText("Succeeded").length).toBeGreaterThan(0);
    expect(screen.getAllByText("1 deploy").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Load more builds" }));
    expect(buildsListState.loadMore).toHaveBeenCalledTimes(1);
  });

  it("renders Build Detail with fan-out deploys, skippedServices, and build-scoped logs", async () => {
    singleBuildState.build = BUILD_ROW;
    renderWithContext(
      <BuildDetailPage buildSourceId="bds-git-1" buildId="bld-1" />,
    );

    expect(
      await screen.findByRole("heading", { name: "bld-1" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /dep-10/ })).toHaveAttribute(
      "href",
      "/services/srv-1/deploys/dep-10",
    );
    expect(screen.getByTestId("build-skipped-services")).toHaveTextContent(
      "jobs-worker",
    );
    expect(buildLogsCalls.at(-1)).toMatchObject({
      buildSourceId: "bds-git-1",
      buildId: "bld-1",
      followBuild: false,
    });
  });

  it("shows the role reason instead of the log panel to a member without can_view_logs", async () => {
    capabilitiesState.canViewLogs = false;
    singleBuildState.build = BUILD_ROW;
    renderWithContext(
      <BuildDetailPage buildSourceId="bds-git-1" buildId="bld-1" />,
    );

    // The build itself reads through can_view, so its facts still render;
    // only the logs stand replaced by the role's reason (w9/145).
    expect(
      await screen.findByRole("heading", { name: "bld-1" }),
    ).toBeInTheDocument();
    expect(await screen.findByText(LOGS_REASON)).toBeInTheDocument();
    expect(buildLogsCalls).toHaveLength(0);
  });

  it("shows image-build-logs notice instead of querying logs for image sources", async () => {
    singleBuildState.build = {
      ...BUILD_ROW,
      id: "bld-img-1",
      buildSourceId: "bds-img-1",
      commitId: null,
      commitUrl: null,
      runtime: null,
    };
    renderWithContext(
      <BuildDetailPage buildSourceId="bds-img-1" buildId="bld-img-1" />,
      {
        ...GIT_SOURCE,
        id: "bds-img-1",
        kind: "image",
        git: null,
        image: {
          ownerId: "tea-1",
          imageUrl: "nginx:1.27",
          registryCredentialId: null,
        },
      },
    );

    expect(
      await screen.findByText(
        "Docker image build sources reuse an existing image and do not produce build logs.",
      ),
    ).toBeInTheDocument();
    expect(buildLogsCalls).toHaveLength(0);
  });
});
