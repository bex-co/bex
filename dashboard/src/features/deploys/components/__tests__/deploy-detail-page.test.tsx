import { beforeEach, describe, expect, it, vi } from "vitest";
import { ServerError } from "@apollo/client/errors";
import { codedGraphQLError } from "@/test/mocks/apollo";
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
import { useDeploy, type DeployView } from "../../hooks/use-deploy";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  ACTIVITY_REASON,
  LOGS_REASON,
  billingCapabilities,
  contributorCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

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

const serverState: {
  service:
    | {
        id: string;
        name?: string;
        type?: string;
        repo?: string | null;
        buildSourceId?: string | null;
        autoDeploy?: boolean | null;
      }
    | undefined;
} = { service: undefined };
const buildSourceState: {
  source: {
    id: string;
    name: string;
    kind: "git" | "image";
    source: string;
    nativeRuntime: null;
  } | null;
} = { source: null };

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: serverState.service,
    loading: false,
    error: undefined,
    ownerId: serverState.service ? "tea-1" : null,
  }),
}));
vi.mock("@/features/services/hooks/use-build-sources", () => ({
  useBuildSource: () => ({ source: buildSourceState.source, loading: false }),
}));
vi.mock("../../hooks/use-deploy", () => ({
  useDeploy: vi.fn(() => deployState),
}));
// The Scaling fact's reads have their own real-Apollo suite
// (use-deploy-scaling.test.tsx).
vi.mock("../../hooks/use-deploy-scaling", () => ({
  useDeployScaling: () => null,
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
  DeployHeader: ({
    actions,
    repo,
    buildSourceId,
    buildSourceName,
  }: {
    actions?: ReactNode;
    repo?: string | null;
    buildSourceId?: string | null;
    buildSourceName?: string | null;
  }) => (
    <div
      data-testid="deploy-header"
      data-repo={repo ?? ""}
      data-build-source-id={buildSourceId ?? ""}
      data-build-source-name={buildSourceName ?? ""}
    >
      {actions}
    </div>
  ),
}));
// The Notices have their own suite (deploy-notices.test.tsx); the page
// hands them the deploy, its service and the service's workspace.
vi.mock("../deploy-notices", () => ({
  DeployNotices: ({
    deploy,
    ownerId,
  }: {
    deploy?: { id: string } | null;
    ownerId?: string | null;
  }) => (
    <div
      data-testid="deploy-notices"
      data-deploy={deploy?.id ?? ""}
      data-owner={ownerId ?? ""}
    />
  ),
}));
vi.mock("../deploy-actions", () => ({
  DeployActions: ({ offerRollback }: { offerRollback?: boolean }) => (
    <div
      data-testid="deploy-actions"
      data-offer-rollback={String(offerRollback)}
    />
  ),
}));
vi.mock("../deploy-rollback", () => ({
  DeployRollback: ({
    status,
    autoDeployOn,
  }: {
    status?: string;
    autoDeployOn?: boolean;
  }) => (
    <div
      data-testid="deploy-rollback"
      data-status={status ?? ""}
      data-auto-deploy={String(autoDeployOn)}
    />
  ),
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
      startedAt,
      hasPreDeploy,
    }: {
      followBuild: boolean;
      endTime: string | undefined;
      startedAt: string | undefined;
      hasPreDeploy: boolean;
    }) => {
      const [mount] = useState(() => ++panelMounts);
      return (
        <div
          data-testid="build-log"
          data-follow-build={String(followBuild)}
          data-end-time={endTime ?? "open"}
          data-started-at={startedAt ?? "none"}
          data-has-pre-deploy={String(hasPreDeploy)}
          data-mount={mount}
        />
      );
    },
  };
});

beforeEach(() => {
  serverState.service = undefined;
  buildSourceState.source = null;
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
  vi.mocked(useDeploy).mockClear();
});

// w9/m621: Render's role table gives the Billing role no "View service
// events", which is where Render's deploys live; bex-api refuses the deploy
// read to it (can_view_activity). A Viewer reads the page as an Admin does.
describe("DeployDetailPage per role (w9/m621, w9/145)", () => {
  it("a Billing member sees why, and the deploy is never read", () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByText(ACTIVITY_REASON)).toBeInTheDocument();
    for (const part of ["deploy-header", "deploy-timeline", "build-log"]) {
      expect(screen.queryByTestId(part)).not.toBeInTheDocument();
    }
    expect(useDeploy).not.toHaveBeenCalled();
  });

  // w9/145: nor does a Viewer "View service logs", so the deploy's log panel
  // (and its reads and build tail) gives way to the role's reason.
  it("a Viewer reads the deploy, with the role's reason in place of its logs", () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByTestId("deploy-header")).toBeInTheDocument();
    expect(screen.getByTestId("deploy-timeline")).toBeInTheDocument();
    expect(screen.queryByText(ACTIVITY_REASON)).not.toBeInTheDocument();
    expect(useDeploy).toHaveBeenCalledWith("web", "dep-1");
    expect(screen.queryByTestId("build-log")).not.toBeInTheDocument();
    expect(screen.getByText(LOGS_REASON)).toBeInTheDocument();
  });

  it("a Contributor reads the deploy and its logs", () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByTestId("build-log")).toBeInTheDocument();
    expect(screen.queryByText(LOGS_REASON)).not.toBeInTheDocument();
  });
});

describe("DeployDetailPage", () => {
  // w1/m646: the deploy's /rollback route is Render's Confirm Rollback page,
  // the deploy's header over the rollback section in place of its log.
  it("puts the Confirm Rollback section in place of the log on its route", () => {
    serverState.service = { autoDeploy: true } as never;
    deployState.deploy = { ...deployState.deploy, status: "deactivated" };
    render(<DeployDetailPage serviceId="web" deployId="dep-1" rollback />);

    expect(screen.getByTestId("deploy-header")).toBeInTheDocument();
    expect(screen.queryByTestId("deploy-actions")).not.toBeInTheDocument();
    const section = screen.getByTestId("deploy-rollback");
    expect(section).toHaveAttribute("data-status", "deactivated");
    expect(section).toHaveAttribute("data-auto-deploy", "true");
    for (const part of ["deploy-timeline", "build-log"]) {
      expect(screen.queryByTestId(part)).not.toBeInTheDocument();
    }
  });

  // w1/m659: Render's Notices sit between the deploy header and the log, and
  // above the Confirm Rollback section on its route.
  it("puts the Notices between the header and the log", () => {
    serverState.service = { id: "web" };
    for (const rollback of [false, true]) {
      const { unmount } = render(
        <DeployDetailPage
          serviceId="web"
          deployId="dep-1"
          rollback={rollback}
        />,
      );
      const notices = screen.getByTestId("deploy-notices");
      expect(notices).toHaveAttribute("data-deploy", "dep-1");
      expect(notices).toHaveAttribute("data-owner", "tea-1");
      const header = screen.getByTestId("deploy-header");
      const body = screen.getByTestId(
        rollback ? "deploy-rollback" : "build-log",
      );
      expect(
        header.compareDocumentPosition(notices) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
      expect(
        notices.compareDocumentPosition(body) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
      unmount();
    }
  });

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

  // w1/157: Render's cron deploy page has no Rollback; Cancel stays.
  it("offers no Rollback on a cron job's deploy page, as Render's has none", () => {
    for (const [type, offered] of [
      ["cron_job", "false"],
      ["web_service", "true"],
      ["background_worker", "true"],
    ]) {
      serverState.service = { id: "web", type };
      const { unmount } = render(
        <DeployDetailPage serviceId="web" deployId="dep-1" />,
      );
      expect(screen.getByTestId("deploy-actions")).toHaveAttribute(
        "data-offer-rollback",
        offered,
      );
      unmount();
    }
  });

  // Render's deploy page passes the deploy's creation as its log's start
  // whatever the range: each line's From start (w4/716).
  it("gives the log the deploy's creation as its start, under a picked range too", () => {
    const { rerender } = render(
      <DeployDetailPage serviceId="web" deployId="dep-1" />,
    );
    expect(screen.getByTestId("build-log")).toHaveAttribute(
      "data-started-at",
      "2026-07-14T00:00:00Z",
    );
    rerender(<DeployDetailPage serviceId="web" deployId="dep-1" range="1h" />);
    expect(screen.getByTestId("build-log")).toHaveAttribute(
      "data-started-at",
      "2026-07-14T00:00:00Z",
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
  // deploy is live, so its output is past the deploy's own end. With no hook
  // the window closes just past the primary URL block bex-api writes after
  // the live line (w32/m567).
  it("closes a live deploy's log window past its primary URL block when no hook ran", () => {
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    const panel = screen.getByTestId("build-log");
    expect(panel).toHaveAttribute("data-end-time", "2026-07-14T00:01:00.007Z");
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

  // w3/m706: Render's DeployDetailsHeader keeps the URL and draws "Deploy
  // not found" for the service, with View deploys then Return home.
  it("draws Deploy not found for the service in place of a dead deploy id", async () => {
    deployState.deploy = null as never;
    deployState.notFound = true;
    serverState.service = { id: "web", name: "orders-api" };

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

    expect(
      await screen.findByRole("heading", { name: "Deploy not found" }),
    ).toBeInTheDocument();
    expect(screen.getByText("dep-missing")).toBeInTheDocument();
    expect(screen.getByText("orders-api")).toBeInTheDocument();
    expect(screen.getByText(/The link may be incorrect\./)).toBeInTheDocument();
    const links = screen.getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual([
      "View deploys",
      "Return home",
    ]);
    expect(links[0]).toHaveAttribute("href", "/services/web/deploys");
    expect(router.state.location.pathname).toBe(
      "/services/web/deploys/dep-missing",
    );
    expect(screen.queryByText("deploys tab")).not.toBeInTheDocument();
  });

  it("draws Access denied for a refused deploy read", () => {
    deployState.deploy = null as never;
    deployState.error = codedGraphQLError("FORBIDDEN", {}, "forbidden");

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(
      screen.getByRole("heading", { name: "Access denied" }),
    ).toBeInTheDocument();
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

  it("forwards linked Build Source provenance and git repo to DeployHeader (w2/m612)", () => {
    serverState.service = {
      id: "srv-1",
      repo: "",
      buildSourceId: "bds-mono001",
      autoDeploy: true,
    };
    buildSourceState.source = {
      id: "bds-mono001",
      name: "Monorepo",
      kind: "git",
      source: "https://github.com/acme/mono",
      nativeRuntime: null,
    };
    render(<DeployDetailPage serviceId="srv-1" deployId="dep-1" />);
    const header = screen.getByTestId("deploy-header");
    expect(header).toHaveAttribute("data-build-source-id", "bds-mono001");
    expect(header).toHaveAttribute("data-build-source-name", "Monorepo");
    expect(header).toHaveAttribute("data-repo", "https://github.com/acme/mono");
  });
});
