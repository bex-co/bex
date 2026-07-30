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
import type { LatestDeploySummary } from "@/features/deploys/hooks/use-latest-deploy";

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

  it("shows the internal address in the Connect menu's Internal section (ADR041 D4)", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ internalAddress: "app-x1y2:8080" }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(screen.getByText("Internal")).toBeInTheDocument();
    expect(screen.getByText("app-x1y2:8080")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy internal address" }),
    ).toBeInTheDocument();
  });

  it("omits the Internal section for a non-addressable service", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ type: "background_worker", internalAddress: null }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(screen.queryByText("Internal")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy internal address" }),
    ).not.toBeInTheDocument();
  });

  it("shows a private service's Service Address as copyable text, never a link", async () => {
    renderHeader(
      svc({
        type: "private_service",
        url: "http://app-x1y2:8080", // the wire still carries it; the header must not link it
        internalAddress: "app-x1y2:8080",
      }),
    );

    expect(await screen.findByText("Service Address")).toBeInTheDocument();
    expect(screen.getByText("app-x1y2:8080")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy internal address" }),
    ).toBeInTheDocument();
    // the old dead cluster-internal link is gone
    expect(
      screen.queryByRole("link", { name: /app-x1y2/ }),
    ).not.toBeInTheDocument();
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

  it("omits the Instances fact for a static site (no runtime instances, w5/m48)", async () => {
    renderHeader(svc({ type: "static_site" }));

    // The rest of the facts line stays; only the meaningless instance count goes.
    expect(await screen.findByText("Slug")).toBeInTheDocument();
    expect(screen.getByText("Revision")).toBeInTheDocument();
    expect(screen.queryByText("Instances")).not.toBeInTheDocument();
  });

  it("omits the instance-type/plan chip for a static site (no /static plan route → would 404, w5/m57)", async () => {
    // A static_site is canonical under /static, which has no plan tab; the chip
    // linked to /services/<id>/plan and canonicalized to a nonexistent
    // /static/<id>/plan (404). Render's static header has no plan concept.
    renderHeader(svc({ type: "static_site", plan: "starter" }));

    await screen.findByRole("heading", { name: "app" });
    expect(screen.queryByRole("link", { name: "Starter" })).not.toBeInTheDocument();
    // A compute type with the same plan still shows the chip.
    renderHeader(svc({ type: "web_service", plan: "starter" }));
    expect(
      await screen.findByRole("link", { name: "Starter" }),
    ).toHaveAttribute("href", "/services/app/plan");
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

  it("deploys the latest commit directly from the Manual Deploy dropdown", async () => {
    const user = userEvent.setup();
    renderHeader(svc());

    await user.click(
      await screen.findByRole("button", { name: "Manual Deploy" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Deploy latest commit" }),
    );

    expect(triggerDeploy).toHaveBeenCalledWith("app");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
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

  it("restarts through the Manual Deploy dropdown via triggerDeploy (w2/m30)", async () => {
    triggerDeploy.mockClear();
    const user = userEvent.setup();
    renderHeader(svc());

    await user.click(
      await screen.findByRole("button", { name: "Manual Deploy" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Restart service" }),
    );

    // Restart routes through the same triggerDeploy mutation as Deploy,
    // so every restart opens a deploy-history row (not a separate onRun path).
    expect(triggerDeploy).toHaveBeenCalledWith("app");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it('has no "•••" actions menu — only Connect + Manual Deploy; restart in Manual Deploy, suspend/resume on Settings', async () => {
    renderHeader(svc());

    // The header's only action controls are Connect + Manual Deploy; the old
    // single-item "•••" dropdown was removed outright (e31388b2) rather than
    // left as a hidden-action menu — Restart is owned by Manual Deploy and
    // Suspend/Resume moved to the Settings page.
    expect(
      await screen.findByRole("button", { name: "Connect" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Manual Deploy" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Open actions menu" }),
    ).not.toBeInTheDocument();
  });

  it("disables Manual Deploy while an action is in flight (poll-to-converge)", async () => {
    renderHeader(svc(), { pending: "restart" });
    expect(
      await screen.findByRole("button", { name: "Manual Deploy" }),
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
