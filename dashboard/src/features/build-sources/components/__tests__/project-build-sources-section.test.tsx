import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { BuildSourceView } from "@/features/build-sources/types";
import { ProjectBuildSourcesSection } from "../project-build-sources-section";

const state = vi.hoisted(() => ({
  buildSources: [] as BuildSourceView[],
  loading: false,
  error: undefined as Error | undefined,
  refetch: vi.fn(async () => []),
  canCreate: true,
}));

vi.mock("@/features/build-sources/hooks/use-build-sources-list", () => ({
  useBuildSourcesList: ({ projectId }: { projectId?: string | null } = {}) => ({
    buildSources: projectId
      ? state.buildSources.filter((b) => b.projectId === projectId)
      : state.buildSources,
    loading: state.loading,
    error: state.error,
    refetch: state.refetch,
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => mockCapabilities({ canCreate: state.canCreate }),
}));

function renderSection(projectId = "prj-1") {
  const rootRoute = createRootRoute();
  const routes = [
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => <ProjectBuildSourcesSection projectId={projectId} />,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/new",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/$buildSourceId",
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
  state.buildSources = [];
  state.loading = false;
  state.error = undefined;
  state.canCreate = true;
});

describe("ProjectBuildSourcesSection", () => {
  it("lists only build sources scoped to the project and links to /build-sources/new?projectId=...", async () => {
    state.buildSources = [
      {
        id: "bds-1",
        name: "storefront-shared",
        ownerId: "tea-1",
        projectId: "prj-1",
        kind: "git",
        git: {
          repoUrl: "https://github.com/acme/storefront",
          branch: "main",
          runtime: "node",
          region: "oregon",
          buildCommand: null,
          rootDir: null,
          dockerfilePath: null,
          dockerContext: null,
          registryCredentialId: null,
          buildFilter: null,
        },
        image: null,
        serviceLinks: [{ id: "srv-1", name: "web" }],
        createdAt: "2026-11-24T00:00:00Z",
        updatedAt: "2026-11-24T00:00:00Z",
        cursor: "bds-1",
      },
      {
        id: "bds-2",
        name: "other-project-src",
        ownerId: "tea-1",
        projectId: "prj-other",
        kind: "image",
        git: null,
        image: {
          ownerId: "tea-1",
          imageUrl: "nginx:1.27",
          registryCredentialId: null,
        },
        serviceLinks: [],
        createdAt: "2026-11-24T00:00:00Z",
        updatedAt: "2026-11-24T00:00:00Z",
        cursor: "bds-2",
      },
    ];

    renderSection("prj-1");

    expect(
      await screen.findByRole("heading", { name: "Build Sources" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("storefront-shared").length).toBeGreaterThan(0);
    expect(screen.queryByText("other-project-src")).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "storefront-shared" })[0],
    ).toHaveAttribute("href", "/build-sources/bds-1");
    expect(
      screen.getByRole("link", { name: "New Build Source" }),
    ).toHaveAttribute("href", "/build-sources/new?projectId=prj-1");
  });

  it("hides New Build Source for Viewers and shows empty state", async () => {
    state.canCreate = false;
    renderSection("prj-1");

    expect(
      await screen.findByText(
        "No build sources are scoped to this project yet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "New Build Source" }),
    ).not.toBeInTheDocument();
  });
});
