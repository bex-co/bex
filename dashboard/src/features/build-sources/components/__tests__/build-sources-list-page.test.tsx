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
import type { BuildSourceView } from "@/features/build-sources/types";
import { BuildSourcesListPage } from "../build-sources-list-page";
import { BuildSourcesListRouteSkeleton } from "../build-source-skeletons";

const listState = vi.hoisted(() => ({
  buildSources: [] as BuildSourceView[],
  loading: false,
  error: undefined as Error | undefined,
  refetch: vi.fn(async () => []),
}));

const projectsState = vi.hoisted(() => ({
  projects: [] as Array<{ id: string; name: string }>,
}));

const capabilitiesState = vi.hoisted(() => ({ canCreate: true }));

vi.mock("@/features/build-sources/hooks/use-build-sources-list", () => ({
  useBuildSourcesList: () => listState,
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: projectsState.projects, loading: false }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () =>
    mockCapabilities({ canCreate: capabilitiesState.canCreate }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

const GIT_SOURCE: BuildSourceView = {
  id: "bds-git-1",
  name: "monorepo-backend",
  ownerId: "tea-1",
  projectId: "prj-1",
  kind: "git",
  git: {
    repoUrl: "https://github.com/acme/mono",
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
  serviceLinks: [
    { id: "srv-1", name: "api" },
    { id: "srv-2", name: "worker" },
  ],
  createdAt: "2026-11-23T00:00:00Z",
  updatedAt: "2026-11-24T00:00:00Z",
  cursor: "bds-git-1",
};

const IMAGE_SOURCE: BuildSourceView = {
  id: "bds-img-1",
  name: "nginx-base",
  ownerId: "tea-1",
  projectId: null,
  kind: "image",
  git: null,
  image: {
    ownerId: "tea-1",
    imageUrl: "ghcr.io/acme/nginx:1.27",
    registryCredentialId: null,
  },
  serviceLinks: [{ id: "srv-3", name: "edge" }],
  createdAt: "2026-11-23T00:00:00Z",
  updatedAt: "2026-11-24T01:00:00Z",
  cursor: "bds-img-1",
};

function renderPage(ui: React.ReactNode) {
  const rootRoute = createRootRoute();
  const routes = [
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources",
      component: () => <>{ui}</>,
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
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/project/$projectId",
      component: () => null,
    }),
  ];
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/build-sources"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  vi.clearAllMocks();
  listState.buildSources = [GIT_SOURCE, IMAGE_SOURCE];
  listState.loading = false;
  listState.error = undefined;
  projectsState.projects = [{ id: "prj-1", name: "Core Platform" }];
  capabilitiesState.canCreate = true;
});

describe("BuildSourcesListPage", () => {
  it("renders Git and Docker image build sources with metadata, linked counts, and project chip", async () => {
    renderPage(<BuildSourcesListPage />);

    expect(
      await screen.findByRole("heading", { name: "Build Sources" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("monorepo-backend").length).toBeGreaterThan(0);
    expect(screen.getAllByText("nginx-base").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("github.com · acme / mono").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText("ghcr.io/acme/nginx:1.27").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("2 services").length).toBeGreaterThan(0);
    expect(screen.getAllByText("1 service").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Core Platform").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Frankfurt, Germany").length).toBeGreaterThan(0);
    expect(
      screen.getAllByRole("link", { name: "monorepo-backend" })[0],
    ).toHaveAttribute("href", "/build-sources/bds-git-1");
    expect(
      screen.getAllByRole("link", { name: "nginx-base" })[0],
    ).toHaveAttribute("href", "/build-sources/bds-img-1");
    expect(
      screen.getByRole("link", { name: "New Build Source" }),
    ).toHaveAttribute("href", "/build-sources/new");
  });

  it("filters by search query and source kind, and resets cleanly", async () => {
    const user = userEvent.setup();
    renderPage(<BuildSourcesListPage />);

    await screen.findByRole("heading", { name: "Build Sources" });

    // Filter to Docker image only.
    await user.click(screen.getByRole("button", { name: "Docker image" }));
    expect(screen.queryByText("monorepo-backend")).not.toBeInTheDocument();
    expect(screen.getAllByText("nginx-base").length).toBeGreaterThan(0);

    // Search for a non-existent term -> no-matches state.
    const input = screen.getByRole("textbox", { name: "Search build sources" });
    await user.type(input, "does-not-exist");
    expect(screen.getByText("No matching build sources")).toBeInTheDocument();

    // Reset filters restores both rows.
    const resetButtons = screen.getAllByRole("button", {
      name: "Reset filters",
    });
    await user.click(resetButtons[0]);
    expect(screen.getAllByText("monorepo-backend").length).toBeGreaterThan(0);
    expect(screen.getAllByText("nginx-base").length).toBeGreaterThan(0);
  });

  it("hides create CTAs for Viewers without can_create", async () => {
    capabilitiesState.canCreate = false;
    listState.buildSources = [];
    renderPage(<BuildSourcesListPage />);

    expect(await screen.findByText("No build sources yet")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "New Build Source" }),
    ).not.toBeInTheDocument();
  });

  it("renders ResourceLoadError with retry on load failure", async () => {
    listState.buildSources = [];
    listState.error = new Error("network down");
    const user = userEvent.setup();
    renderPage(<BuildSourcesListPage />);

    const retry = await screen.findByRole("button", { name: "Try again" });
    await user.click(retry);
    expect(listState.refetch).toHaveBeenCalledTimes(1);
  });

  it("keeps cached rows mounted with an inline refresh alert when refresh fails", async () => {
    listState.error = new Error("stale refresh");
    renderPage(<BuildSourcesListPage />);

    expect(
      await screen.findByText(
        "Couldn't refresh build sources. Showing cached results.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("monorepo-backend").length).toBeGreaterThan(0);
  });

  it("skeleton matches ready-state regions, desktop table columns, and mobile card container", async () => {
    const pending = renderPage(<BuildSourcesListRouteSkeleton />);
    await screen.findByText("Build Sources");
    const entry = ROUTE_SKELETON_MANIFEST["/build-sources"];
    const skeletonRegions = [
      ...pending.container.querySelectorAll<HTMLElement>(
        "[data-skeleton-region]",
      ),
    ].map((el) => el.dataset.skeletonRegion);
    expect(skeletonRegions).toEqual(entry.regions);
    const skeletonCols = [
      ...pending.container.querySelectorAll("thead th"),
    ].map((th) => th.className);
    const skeletonHasMobileCards = Boolean(
      pending.container.querySelector(".space-y-3.md\\:hidden"),
    );
    pending.unmount();

    const ready = renderPage(<BuildSourcesListPage />);
    await screen.findByRole("heading", { name: "Build Sources" });
    const readyRegions = [
      ...ready.container.querySelectorAll<HTMLElement>("[data-region]"),
    ].map((el) => el.dataset.region);
    expect(readyRegions).toEqual(["filters", "build-sources-list"]);
    const readyCols = [...ready.container.querySelectorAll("thead th")].map(
      (th) => th.className,
    );
    expect(skeletonCols).toEqual(readyCols);
    expect(skeletonHasMobileCards).toBe(true);
    expect(
      Boolean(ready.container.querySelector(".space-y-3.md\\:hidden")),
    ).toBe(true);
  });
});
