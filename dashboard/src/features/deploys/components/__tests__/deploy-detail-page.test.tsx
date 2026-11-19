import { beforeEach, describe, expect, it, vi } from "vitest";
import { ServerError } from "@apollo/client/errors";
import { fireEvent, render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type { ReactNode } from "react";
import { DeployDetailPage } from "../deploy-detail-page";
import type { DeployView } from "../../hooks/use-deploy";

const deployState: {
  deploy: DeployView;
  loading: boolean;
  error: Error | undefined;
  notFound: boolean;
  refetch: () => void;
} = {
  deploy: {
    id: "dep-1",
    status: "live",
    trigger: "api",
    image: "registry.example.com/web:1",
    imageSha: "",
    rollbackOf: "",
    buildAvailable: null,
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-07-14T00:00:00Z",
    updatedAt: "2026-07-14T00:01:00Z",
    startedAt: null,
    finishedAt: "2026-07-14T00:01:00Z",
    preDeployStatus: "",
    initialDeployHookStatus: "",
    initialDeployHookFinishedAt: null,
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    failureReasonTemplate: null,
    stallReasonTemplate: null,
    cancelReasonTemplate: null,
  },
  loading: false,
  error: undefined as Error | undefined,
  notFound: false,
  refetch: vi.fn(),
};

// The page's back link is a router Link; most of these tests render the page
// with no router around it, so it stands in as a plain anchor at its path.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: (await import("@/test/mocks/router-link")).PlainLink,
}));

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({ service: undefined, loading: false, error: undefined }),
}));
vi.mock("../../hooks/use-deploy", () => ({
  useDeploy: () => deployState,
}));
// The canceled-step window has its own real-Apollo suite
// (deploy-detail-canceled-step.test.tsx); these deploys never need it.
vi.mock("../../hooks/use-canceled-step-end", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("../../hooks/use-canceled-step-end")
  >()),
  useCanceledStepEnd: () => ({ status: "none" }),
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
let panelMounts = 0;
vi.mock("../deploy-log-panel", async () => {
  const { useState } = await import("react");
  return {
    DeployLogPanel: ({
      followBuild,
      endTime,
      hasPreDeploy,
    }: {
      followBuild: boolean;
      endTime: string | undefined;
      hasPreDeploy: boolean;
    }) => {
      const [mount] = useState(() => ++panelMounts);
      return (
        <div
          data-testid="build-log"
          data-follow-build={String(followBuild)}
          data-end-time={endTime ?? "open"}
          data-has-pre-deploy={String(hasPreDeploy)}
          data-mount={mount}
        />
      );
    },
  };
});

beforeEach(() => {
  deployState.deploy = {
    ...deployState.deploy,
    id: "dep-1",
    status: "live",
    finishedAt: "2026-07-14T00:01:00Z",
    initialDeployHookStatus: "",
    initialDeployHookFinishedAt: null,
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

  it("leads back to the service's Deploys list above the header", () => {
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);
    expect(screen.getByRole("link", { name: "Deploys" })).toHaveAttribute(
      "href",
      "/services/web/deploys",
    );
  });

  // w4/m192: the build tail keeps streamed rows through a pause, so another
  // deploy of the same service must get a fresh panel, not the old build's rows.
  it("starts a fresh log panel when navigating to another deploy", () => {
    const { rerender } = render(
      <DeployDetailPage serviceId="web" deployId="dep-1" />,
    );
    const first = screen.getByTestId("build-log").dataset.mount;
    rerender(<DeployDetailPage serviceId="web" deployId="dep-1" />);
    expect(screen.getByTestId("build-log").dataset.mount).toBe(first);
    rerender(<DeployDetailPage serviceId="web" deployId="dep-2" />);
    expect(screen.getByTestId("build-log").dataset.mount).not.toBe(first);
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

  // w33/m435: the Blueprint's one-time initialDeployHook runs after the
  // deploy is live, so its output is past the deploy's own end.
  it("closes a live deploy's log window at its own end when no hook ran", () => {
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    const panel = screen.getByTestId("build-log");
    expect(panel).toHaveAttribute("data-end-time", "2026-07-14T00:01:00Z");
    expect(panel).toHaveAttribute("data-has-pre-deploy", "false");
  });

  it("keeps the log window open, and the live pod read on, while the hook runs", () => {
    deployState.deploy.initialDeployHookStatus = "running";

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    const panel = screen.getByTestId("build-log");
    expect(panel).toHaveAttribute("data-end-time", "open");
    expect(panel).toHaveAttribute("data-has-pre-deploy", "true");
    // The deploy is live, not building: only the window follows the hook.
    expect(panel).toHaveAttribute("data-follow-build", "false");
  });

  it.each(["succeeded", "failed"])(
    "ends the log window where a %s hook ended",
    (status) => {
      deployState.deploy.initialDeployHookStatus = status;
      deployState.deploy.initialDeployHookFinishedAt = "2026-07-14T00:01:08Z";

      render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

      expect(screen.getByTestId("build-log")).toHaveAttribute(
        "data-end-time",
        "2026-07-14T00:01:09.000Z",
      );
    },
  );

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

  it("offers Sign in, not Try again, when the session has expired", () => {
    deployState.deploy = null as never;
    deployState.error = new ServerError("status 401", {
      response: new Response(null, { status: 401 }),
      bodyText: "",
    });

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByText("Your session has expired")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  });

  it("shows a request error instead of leaving a permanent skeleton", () => {
    deployState.deploy = null as never;
    deployState.error = new Error("deploy query unavailable");

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    // A read that never got an answer shows translated copy, not its raw
    // transport text (w5/167).
    expect(
      screen.getByText(/This deploy couldn't be read just now/),
    ).toBeInTheDocument();
    expect(screen.queryByText("deploy query unavailable")).toBeNull();
    // It names what failed — the deploy, not its logs — and offers a retry.
    expect(screen.getByText("Couldn't load this deploy")).toBeInTheDocument();
    // The way back stays when the deploy itself cannot be read.
    expect(screen.getByRole("link", { name: "Deploys" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(deployState.refetch).toHaveBeenCalled();
  });
});
