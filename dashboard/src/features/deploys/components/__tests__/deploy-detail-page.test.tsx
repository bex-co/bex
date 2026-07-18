import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type { ReactNode } from "react";
import { DeployDetailPage } from "../deploy-detail-page";

const deployState = {
  deploy: {
    id: "dep-1",
    status: "live",
    trigger: "api",
    image: "registry.example.com/web:1",
    rollbackOf: "",
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-07-14T00:00:00Z",
    updatedAt: "2026-07-14T00:01:00Z",
    startedAt: null,
    finishedAt: "2026-07-14T00:01:00Z",
    preDeployStatus: "",
  },
  loading: false,
  error: undefined as Error | undefined,
  notFound: false,
};

vi.mock("../../hooks/use-deploy", () => ({
  useDeploy: () => deployState,
}));
vi.mock("../deploy-header", () => ({
  DeployHeader: ({ actions }: { actions?: ReactNode }) => (
    <div data-testid="deploy-header">{actions}</div>
  ),
}));
vi.mock("../deploy-actions", () => ({
  DeployActions: () => <div data-testid="deploy-actions" />,
}));
vi.mock("../deploy-timeline", () => ({
  DeployTimeline: () => <div data-testid="deploy-timeline" />,
}));
vi.mock("../deploy-log-panel", () => ({
  DeployLogPanel: ({ followBuild }: { followBuild: boolean }) => (
    <div data-testid="build-log" data-follow-build={String(followBuild)} />
  ),
}));

beforeEach(() => {
  deployState.deploy = {
    ...deployState.deploy,
    id: "dep-1",
    status: "live",
    finishedAt: "2026-07-14T00:01:00Z",
  };
  deployState.loading = false;
  deployState.error = undefined;
  deployState.notFound = false;
});

describe("DeployDetailPage", () => {
  it("renders the shared actions, timeline, and deploy log viewer", () => {
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByTestId("deploy-header")).toBeInTheDocument();
    expect(screen.getByTestId("deploy-actions")).toBeInTheDocument();
    expect(screen.getByTestId("deploy-timeline")).toBeInTheDocument();
    expect(screen.getByTestId("build-log")).toBeInTheDocument();
    expect(screen.getByTestId("build-log")).toHaveAttribute(
      "data-follow-build",
      "false",
    );
  });

  it("switches the build pane to SSE only during build_in_progress", () => {
    deployState.deploy.status = "build_in_progress";
    deployState.deploy.finishedAt = null;

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByTestId("build-log")).toHaveAttribute(
      "data-follow-build",
      "true",
    );
  });

  it("redirects a dead deploy id to the service's Deploys tab (w9/m55)", async () => {
    deployState.deploy = null as never;
    deployState.notFound = true;

    const rootRoute = createRootRoute();
    const deploysRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId/deploys",
      component: () => <div>deploys tab</div>,
    });
    const detailRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId/deploys/$deployId",
      component: () => (
        <DeployDetailPage serviceId="web" deployId="dep-missing" />
      ),
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([deploysRoute, detailRoute]),
      history: createMemoryHistory({
        initialEntries: ["/services/web/deploys/dep-missing"],
      }),
      context: { client: {} as never, session: null },
    });
    render(<RouterProvider router={router} />);

    // to the nearest live parent — the service exists, only the deploy is dead
    expect(await screen.findByText("deploys tab")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/services/web/deploys");
  });

  it("shows a request error instead of leaving a permanent skeleton", () => {
    deployState.deploy = null as never;
    deployState.error = new Error("deploy query unavailable");

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByText("deploy query unavailable")).toBeInTheDocument();
  });
});
