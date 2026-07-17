import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceDetailHeader } from "@/features/services/components/service-detail-header";
import type { ServiceView } from "@/features/services/types";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";
import type { RunServiceAction } from "@/features/services/hooks/use-service-lifecycle";
import type { LatestDeploySummary } from "@/features/deploys/hooks/use-latest-deploy";

// ServiceDetailHeader renders ServiceRowActions, which renders a "Move to
// project" submenu via useMoveToProject — needs an ApolloProvider + workspace
// context neither this render nor the component under test cares about; stub
// it to the empty-projects shape (the submenu renders nothing with no projects).
vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

// The instance-type chip and the Manual Deploy button both go through Apollo;
// stub them at the hook boundary (the pattern every other panel's test uses) so
// this render needs no ApolloProvider.
const STARTER: InstanceTypeView = {
  id: "starter",
  name: "Starter",
  cpu: "500m",
  memory: "512Mi",
};
vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    instanceTypes: [STARTER],
    loading: false,
    error: undefined,
    byID: (id: string | null | undefined) =>
      id === "starter" ? STARTER : undefined,
  }),
}));
const triggerDeploy = vi.fn(async () => {});
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ deploying: false, trigger: triggerDeploy }),
}));

vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credentials",
  () => ({
    useRegistryCredentials: () => ({
      credentials: [
        {
          id: "rgc-private",
          name: "Private GHCR",
          host: "ghcr.io",
          username: "robot",
          expiresAt: null,
          status: "active",
          createdAt: null,
        },
      ],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }),
  }),
);

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "r1",
    slug: "app-x1y2",
    plan: "starter",
    repo: "https://github.com/bex-co/hello-go.git",
    branch: "main",
    registryCredentialId: null,
    schedule: null,
    ...overrides,
  } as ServiceView;
}

// The header links to the plan tab, so it needs a router around it.
function renderHeader(
  service: ServiceView,
  props: Partial<{
    pending: null | "restart";
    onRun: RunServiceAction;
    latestDeploy: LatestDeploySummary | null;
  }> = {},
) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => (
      <ServiceDetailHeader
        service={service}
        latestDeploy={props.latestDeploy}
        pending={props.pending ?? null}
        onRun={
          props.onRun ?? vi.fn(async () => ({ status: "success" as const }))
        }
      />
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

describe("ServiceDetailHeader", () => {
  it("renders the service identity, status badge, and live URL", async () => {
    renderHeader(svc());

    expect(
      await screen.findByRole("heading", { name: "app" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Web Service")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "https://app.onbex.co" }),
    ).toHaveAttribute("href", "https://app.onbex.co");
  });

  it("names service phase and latest deploy status separately and shows runtime", async () => {
    renderHeader(svc({ phase: "Building", runtime: "node" }), {
      latestDeploy: { id: "dep-failed", status: "build_failed" },
    });

    expect(await screen.findByText("Service")).toBeInTheDocument();
    expect(screen.getByText("Building")).toBeInTheDocument();
    expect(screen.getByText("Latest deploy")).toBeInTheDocument();
    expect(screen.getByText("Build Failed")).toBeInTheDocument();
    expect(screen.getByText("Runtime")).toBeInTheDocument();
    expect(screen.getByText("node")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Latest deploy: Build Failed" }),
    ).toHaveAttribute("href", "/services/app/deploys/dep-failed");
  });

  it("omits runtime and latest-deploy facts when the API does not supply them", async () => {
    renderHeader(svc({ runtime: null }), { latestDeploy: null });

    await screen.findByRole("heading", { name: "app" });
    expect(screen.queryByText("Runtime")).not.toBeInTheDocument();
    expect(screen.queryByText("Latest deploy")).not.toBeInTheDocument();
  });

  it("renders Docker as an API-supplied runtime without inventing a language", async () => {
    renderHeader(svc({ runtime: "docker" }));

    expect(await screen.findByText("Runtime")).toBeInTheDocument();
    expect(screen.getByText("docker")).toBeInTheDocument();
  });

  it("shows a copy-ready SSH command in the Connect menu", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ sshAddress: "srv-example@ssh.bex.co" }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(screen.getByText("SSH")).toBeInTheDocument();
    expect(screen.getByText("ssh srv-example@ssh.bex.co")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy SSH command" }),
    ).toBeInTheDocument();
  });

  it("explains unavailable SSH in the Connect menu without inventing a command", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ sshAddress: null }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(screen.getByText("SSH isn't available")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy SSH command" }),
    ).not.toBeInTheDocument();
  });

  it("carries the facts the retired Overview tab showed: id, source, instance type, revision", async () => {
    renderHeader(svc());

    // service id + its copy affordance (Render's header metadata stack)
    expect(await screen.findByText("Service ID:")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy service ID" }),
    ).toBeInTheDocument();

    // source repo links to the branch tree, with the branch beside it
    expect(
      screen.getByRole("link", { name: "bex-co / hello-go" }),
    ).toHaveAttribute("href", "https://github.com/bex-co/hello-go/tree/main");
    expect(screen.getByText("main")).toBeInTheDocument();

    // instance-type chip deep-links to the plan tab
    expect(screen.getByRole("link", { name: "Starter" })).toHaveAttribute(
      "href",
      "/services/app/plan",
    );

    // the bex-native facts line, incl. the platform-host slug (w4/m20/t002)
    expect(screen.getByText("Slug")).toBeInTheDocument();
    expect(screen.getByText("app-x1y2")).toBeInTheDocument();
    expect(screen.getByText("Instances")).toBeInTheDocument();
    expect(screen.getByText("Revision")).toBeInTheDocument();
    expect(screen.getByText("r1")).toBeInTheDocument();
  });

  it("shows a bound image credential by name and links to its settings panel", async () => {
    renderHeader(
      svc({ repo: null, branch: null, registryCredentialId: "rgc-private" }),
    );

    expect(await screen.findByText("Registry credential:")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Private GHCR" })).toHaveAttribute(
      "href",
      "/settings#registry-credentials",
    );
  });

  it("does not add credential metadata to an unbound service", async () => {
    renderHeader(svc({ registryCredentialId: null }));

    await screen.findByRole("heading", { name: "app" });
    expect(screen.queryByText("Registry credential:")).not.toBeInTheDocument();
  });

  it("shows a cron job's schedule in place of the URL row", async () => {
    renderHeader(
      svc({ type: "cron_job", url: null, schedule: "*/5 * * * *", plan: null }),
    );

    expect(await screen.findByText("Cron Job")).toBeInTheDocument();
    expect(screen.getByText("*/5 * * * *")).toBeInTheDocument();
    expect(screen.queryByText("https://app.onbex.co")).not.toBeInTheDocument();
  });

  it("deploys the latest commit from the Manual Deploy dropdown, behind a confirm naming the branch", async () => {
    const user = userEvent.setup();
    renderHeader(svc());

    await user.click(
      await screen.findByRole("button", { name: "Manual Deploy" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Deploy latest commit" }),
    );
    expect(
      screen.getByText("Deploy the latest commit on main?"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Proceed" }));

    expect(triggerDeploy).toHaveBeenCalledWith("app");
  });

  it("labels the deploy item for an image-backed service (no repo to rebuild from)", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ repo: null, branch: null }));

    await user.click(
      await screen.findByRole("button", { name: "Manual Deploy" }),
    );

    expect(
      screen.getByRole("menuitem", { name: "Deploy latest image" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Deploy latest commit" }),
    ).not.toBeInTheDocument();
  });

  it('restarts through the Manual Deploy dropdown via triggerDeploy (w2/m30), not a duplicate control in the "•••" menu', async () => {
    triggerDeploy.mockClear();
    const user = userEvent.setup();
    renderHeader(svc());

    await user.click(
      await screen.findByRole("button", { name: "Manual Deploy" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Restart service" }),
    );
    await user.click(screen.getByRole("button", { name: "Proceed" }));

    // Restart now routes through the same triggerDeploy mutation as Deploy,
    // so every restart opens a deploy-history row (not a separate onRun path).
    expect(triggerDeploy).toHaveBeenCalledWith("app");

    // the "•••" menu keeps Suspend but no longer offers Restart — it's owned
    // by Manual Deploy on this page (Render's own grouping)
    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    expect(
      screen.getByRole("menuitem", { name: "Suspend" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Restart" }),
    ).not.toBeInTheDocument();
  });

  it("fires a lifecycle action through the reused row-actions control", async () => {
    const onRun = vi.fn(async () => ({ status: "success" as const }));
    const suspended = svc({ suspended: true, phase: "Hibernated", url: null });
    const user = userEvent.setup();

    renderHeader(suspended, { onRun });

    // a suspended service exposes only Resume, which runs without a confirm
    await user.click(
      await screen.findByRole("button", { name: "Open actions menu" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Resume" }));

    expect(onRun).toHaveBeenCalledWith("resume", suspended);
  });

  it("disables the actions control while an action is in flight (poll-to-converge)", async () => {
    renderHeader(svc(), { pending: "restart" });
    expect(
      await screen.findByRole("button", { name: "Open actions menu" }),
    ).toBeDisabled();
  });

  it("shows the maintenance-mode banner when enabled", async () => {
    renderHeader(svc({ maintenanceMode: { enabled: true, uri: "" } }));

    expect(
      await screen.findByText("Maintenance mode is on"),
    ).toBeInTheDocument();
  });

  it("shows no banner when maintenance mode is disabled or unselected", async () => {
    renderHeader(svc({ maintenanceMode: { enabled: false, uri: "" } }));
    expect(
      await screen.findByRole("heading", { name: "app" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Maintenance mode is on"),
    ).not.toBeInTheDocument();

    renderHeader(svc({ maintenanceMode: null }));
    expect(
      screen.queryByText("Maintenance mode is on"),
    ).not.toBeInTheDocument();
  });
});
