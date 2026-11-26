import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
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
import { BuildSourceLayout } from "../build-source-layout";
import { BuildSourceDetailRouteSkeleton } from "../build-source-skeletons";
import { usePipelineMinutesExceeded } from "@/features/usage/hooks/use-pipeline-minutes-exceeded";

const sourceState = vi.hoisted(() => ({
  buildSource: null as BuildSourceView | null,
  loading: false,
  error: undefined as Error | undefined,
  notFound: false,
  refetch: vi.fn(async () => null),
}));

const capabilitiesState = vi.hoisted(() => ({ canCreate: true }));
const notFoundSpy = vi.hoisted(() => vi.fn());

vi.mock("@/features/build-sources/hooks/use-build-source", () => ({
  useBuildSource: () => sourceState,
}));
vi.mock("@/features/build-sources/hooks/use-trigger-build", () => ({
  useTriggerBuild: () => ({ trigger: vi.fn(), busy: false }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () =>
    mockCapabilities({ canCreate: capabilitiesState.canCreate }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: [{ id: "prj-1", name: "Core" }] }),
}));
vi.mock("@/features/workspaces/hooks/use-adopt-resource-workspace", () => ({
  useAdoptResourceWorkspace: () => false,
}));
vi.mock("@/common/hooks/use-not-found-redirect", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("@/common/hooks/use-not-found-redirect")
    >();
  return {
    ...actual,
    useNotFoundRedirect: (...args: unknown[]) => notFoundSpy(...args),
  };
});
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

const GIT_SOURCE: BuildSourceView = {
  id: "bds-git-1",
  name: "shared-backend",
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
  serviceLinks: [{ id: "srv-1", name: "api" }],
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
  serviceLinks: [],
  createdAt: "2026-11-23T00:00:00Z",
  updatedAt: "2026-11-24T00:00:00Z",
  cursor: "bds-img-1",
};

function renderLayout(buildSourceId: string) {
  const rootRoute = createRootRoute();
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/build-sources/$buildSourceId",
    component: () => <BuildSourceLayout buildSourceId={buildSourceId} />,
  });
  const routes = [
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/project/$projectId",
      component: () => null,
    }),
    detailRoute.addChildren([
      createRoute({
        getParentRoute: () => detailRoute,
        path: "/",
        component: () => <div data-testid="outlet-builds">Builds Outlet</div>,
      }),
      createRoute({
        getParentRoute: () => detailRoute,
        path: "/services",
        component: () => null,
      }),
      createRoute({
        getParentRoute: () => detailRoute,
        path: "/environment",
        component: () => null,
      }),
      createRoute({
        getParentRoute: () => detailRoute,
        path: "/settings",
        component: () => null,
      }),
    ]),
  ];
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({
      initialEntries: [`/build-sources/${buildSourceId}`],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  vi.clearAllMocks();
  sourceState.buildSource = null;
  sourceState.loading = false;
  sourceState.error = undefined;
  sourceState.notFound = false;
  capabilitiesState.canCreate = true;
});

describe("BuildSourceLayout (w2/m614/t001)", () => {
  it("renders git source header, all 4 tabs, and enabled Trigger Build CTA", async () => {
    sourceState.buildSource = GIT_SOURCE;
    renderLayout(GIT_SOURCE.id);

    expect(
      await screen.findByRole("heading", { name: "shared-backend" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Git")).toBeInTheDocument();
    expect(screen.getByText("go")).toBeInTheDocument();
    expect(screen.getByText("Frankfurt, Germany")).toBeInTheDocument();
    expect(screen.getByText("Core")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Builds" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Services/ })).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Environment" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Trigger Build" }),
    ).not.toBeDisabled();
    expect(screen.getByTestId("outlet-builds")).toBeInTheDocument();
  });

  it("hides Trigger Build CTA and Environment tab for Docker image sources", async () => {
    sourceState.buildSource = IMAGE_SOURCE;
    renderLayout(IMAGE_SOURCE.id);

    expect(
      await screen.findByRole("heading", { name: "nginx-base" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Docker image")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Trigger Build" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Environment" })).toBeNull();
    expect(screen.getByRole("link", { name: "Builds" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
  });

  it("disables Trigger Build for viewers (canCreate: false)", async () => {
    sourceState.buildSource = GIT_SOURCE;
    capabilitiesState.canCreate = false;
    renderLayout(GIT_SOURCE.id);

    expect(
      await screen.findByRole("button", { name: "Trigger Build" }),
    ).toBeDisabled();
  });

  // Render's `pfe` (w3/108): a build source's Manual Build is disabled while
  // the workspace's pipeline minutes are spent, with that notice ahead of the
  // permission's.
  it("disables Trigger Build with Render's notice while pipeline minutes are spent", async () => {
    sourceState.buildSource = GIT_SOURCE;
    capabilitiesState.canCreate = false;
    vi.mocked(usePipelineMinutesExceeded).mockReturnValue(true);
    const user = userEvent.setup();
    renderLayout(GIT_SOURCE.id);

    const button = await screen.findByRole("button", { name: "Trigger Build" });
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    const notice = await screen.findByRole("tooltip");
    expect(notice).toHaveTextContent(
      "You’ve used up the maximum number of pipeline minutes specified by your spend limit.Learn more.",
    );
    expect(
      within(notice).getByRole("link", { name: "pipeline minutes" }),
    ).toHaveAttribute("href", "/workspace/settings#build-pipeline");
  });

  it("redirects unknown build source IDs back to /build-sources", async () => {
    sourceState.buildSource = null;
    sourceState.notFound = true;
    sourceState.error = new Error("not found");
    renderLayout("bds-missing");

    await screen.findByText((_, el) => el?.getAttribute("data-route-skeleton") === "build-source-detail");
    expect(notFoundSpy).toHaveBeenCalledWith(true, {
      to: "/build-sources",
      replace: true,
    });
  });

  it("renders ResourceLoadError instead of redirecting on network error", async () => {
    sourceState.buildSource = null;
    sourceState.error = new Error("network timeout");
    renderLayout("bds-err");

    expect(await screen.findByRole("button", { name: /Try again/i })).toBeInTheDocument();
    expect(notFoundSpy).toHaveBeenCalledWith(false, {
      to: "/build-sources",
      replace: true,
    });
  });

  it("pending skeleton declares all regions in ROUTE_SKELETON_MANIFEST", () => {
    const { container } = render(<BuildSourceDetailRouteSkeleton />);
    const manifest = ROUTE_SKELETON_MANIFEST["/build-sources/$buildSourceId"];
    expect(manifest.kind).toBe("render");
    if (manifest.kind === "render") {
      expect(
        container.querySelector('[data-skeleton-region="detail-tabs"]'),
      ).not.toBeNull();
      expect(
        container.querySelector('[data-skeleton-region="builds-list"]'),
      ).not.toBeNull();
    }
  });
});
