import { beforeEach, describe, it, expect, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Outlet,
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import {
  ServiceDetailHeader,
  ServiceDetailHeaderSkeleton,
  type ServiceDetailHeaderProps,
} from "@/features/services/components/service-detail-header";
import type { ServiceView } from "@/features/services/types";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";
import type { LatestDeploySummary } from "@/features/deploys/hooks/use-latest-deploy";
import { mockAllowedResourceActions } from "@/test/mocks/resource-actions";
import { useShellGate } from "@/features/services/hooks/use-shell-gate";
import {
  hydrateAcrossBoundary,
  hydrateAcrossEnvironments,
} from "@/test/hydration";

// The instance-type chip and the Manual Deploy button both go through Apollo;
// stub them at the hook boundary (the pattern every other panel's test uses) so
// this render needs no ApolloProvider.
const STARTER: InstanceTypeView = {
  id: "starter",
  name: "Starter",
  cpu: "500m",
  memory: "512Mi",
  monthlyUsd: "4.90",
  available: true,
  unavailableReason: "",
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
// Where the service lives, which the header names below `lg` (the top bar's
// breadcrumbs carry it from `lg`).
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [{ id: "prj-1", name: "storefront", serviceIds: ["app"] }],
  }),
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: (projectId: string | null) => ({
    environments:
      projectId === "prj-1"
        ? [{ id: "env-1", name: "Production", serviceIds: ["app"] }]
        : [],
  }),
}));
const triggerDeploy = vi.fn(async () => {});
const restartServer = vi.fn(async () => null);
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({
    deploying: false,
    trigger: triggerDeploy,
    restart: restartServer,
  }),
}));
const triggerCronRun = vi.fn(async () => {});
let cronTriggering = false;
vi.mock("@/features/services/hooks/use-trigger-cron-run", () => ({
  useTriggerCronRun: () => ({
    triggering: cronTriggering,
    trigger: triggerCronRun,
  }),
}));
vi.mock("@/features/services/hooks/use-auto-deploy", () => ({
  useAutoDeploy: () => ({ setAutoDeploy: vi.fn(), busy: false }),
}));

let resourceActions = mockAllowedResourceActions("app");
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: () => resourceActions.useServerActions(),
  useDeployActions: () => resourceActions.useDeployActions(),
}));
const permissionQuery = vi.fn();
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useApolloClient: () => ({ query: permissionQuery }),
}));

beforeEach(() => {
  resourceActions = mockAllowedResourceActions("app");
  vi.mocked(useShellGate).mockReturnValue({
    gate: { kind: "ready" },
    reason: undefined,
  });
  permissionQuery.mockReset().mockResolvedValue({
    data: {
      deployActions: [
        {
          action: "deploy",
          outcome: "allowed",
          reason: null,
          precondition: null,
        },
      ],
    },
  });
  triggerDeploy.mockClear();
  restartServer.mockClear();
  triggerCronRun.mockClear();
  cronTriggering = false;
  workspacePlan = "hobby";
});

// The current workspace's plan, which titles an owner's suspension (w3/m643).
let workspacePlan = "hobby";
vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  const { useWorkspace } = mockWorkspaceContext();
  return {
    useWorkspace: () => {
      const context = useWorkspace();
      return {
        ...context,
        currentWorkspace: { ...context.currentWorkspace, plan: workspacePlan },
      };
    },
  };
});

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
function headerRouter(
  service: ServiceView,
  props: Partial<{
    pending: null | "restart";
    latestDeploy: LatestDeploySummary | null;
    onRun: ServiceDetailHeaderProps["onRun"];
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
        onRun={props.onRun}
      />
    ),
  });
  return createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
}

function renderHeader(
  service: ServiceView,
  props: Parameters<typeof headerRouter>[1] = {},
) {
  return render(<RouterProvider router={headerRouter(service, props)} />);
}

describe("ServiceDetailHeader", () => {
  // The server owns the saved/running distinction. The notice also applies to a
  // rollback and a restart of that release, so it must not claim a cancellation.
  it("explains how to apply saved settings that are not running", async () => {
    renderHeader(svc({ undeployedChanges: true }));
    expect(
      await screen.findByText(
        "Saved changes aren't live yet. Use a standard deploy to apply them.",
      ),
    ).toBeInTheDocument();
  });

  // w4/m184: while the rolling deploy already carries the saved settings,
  // telling the user to deploy again invites a redundant second deploy.
  it("says the saved changes are on their way while the carrying deploy runs", async () => {
    renderHeader(
      svc({ undeployedChanges: true, undeployedChangesApplying: true }),
    );
    expect(
      await screen.findByText(
        "Saved changes go live when the current deploy finishes.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Use a standard deploy/)).toBeNull();
  });

  it("says nothing about undeployed changes on an ordinary service", async () => {
    renderHeader(svc({ undeployedChanges: false }));
    await screen.findByRole("heading", { name: "app" });
    expect(screen.queryByText(/aren't live yet/)).not.toBeInTheDocument();
  });

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

  it("names the project and environment beside the type below lg", async () => {
    renderHeader(svc());
    const scope = await screen.findByText(/storefront \/ Production/);
    // From `lg` the breadcrumbs say it; the header's copy hides there.
    expect(scope).toHaveClass("lg:hidden");
  });

  // w3/m81: a deleting service's route/certificate are withdrawn, so its URL is
  // dead. The header must read "Deleting" and never render that URL as a live
  // link, even if the wire still carries it (the m81 fixture served a dead link
  // for 2+ hours). By-id reads 404 the instant deletion is accepted, so this is
  // the belt-and-suspenders for any transient render before the redirect.
  it("shows a deleting service as Deleting with no live URL link", async () => {
    renderHeader(svc({ phase: "Deleting", url: "https://app.onbex.co" }));

    expect(await screen.findByText("Deleting")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "https://app.onbex.co" }),
    ).not.toBeInTheDocument();
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

  // w4/726: native SSH and the browser terminal refuse a Developer in a
  // protected environment, so Connect says why instead of offering either.
  it("shows a refused caller the reason instead of the SSH command", async () => {
    vi.mocked(useShellGate).mockReturnValue({
      gate: { kind: "denied", reasonKey: "capabilities.actionAdminOnly" },
      reason: "This action is limited to admins.",
    });
    const user = userEvent.setup();
    renderHeader(svc({ sshAddress: "srv-example@ssh.bex.co" }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(
      screen.getByText("This action is limited to admins."),
    ).toBeInTheDocument();
    expect(vi.mocked(useShellGate)).toHaveBeenCalledWith("app");
    expect(screen.queryByText("ssh srv-example@ssh.bex.co")).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Copy SSH command" }),
    ).toBeNull();
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
    // Render's line: the private network is per region (w4/m712).
    expect(
      screen.getByText(
        "All of your bex services in the same region can communicate internally over their private network.",
      ),
    ).toBeInTheDocument();
  });

  describe("Free web service private-network guidance (w4/252, w8/m130)", () => {
    const note =
      "Free web services can't receive private network traffic. Upgrade to use the internal address.";
    const free = {
      plan: "free",
      phase: "Hibernated",
      internalAddress: "app-x1y2:8080",
    };

    it("qualifies the sleeping hint as a public request", async () => {
      renderHeader(svc(free));
      expect(
        await screen.findByText(
          "Sleeping to save resources — wakes on the next request to its public URL.",
        ),
      ).toBeInTheDocument();
    });

    it("explains next to the internal address that a Free service takes no private traffic", async () => {
      const user = userEvent.setup();
      renderHeader(svc(free));
      await user.click(await screen.findByRole("button", { name: "Connect" }));
      expect(screen.getByText("app-x1y2:8080")).toBeInTheDocument();
      expect(screen.getByText(note)).toBeInTheDocument();
    });

    it.each([
      ["a paid web service", { plan: "starter", phase: "Running" }],
      [
        "a private service",
        { type: "private_service", url: null, phase: "Running" },
      ],
      ["a Free web service with no public URL", { url: null }],
    ])("omits the note for %s", async (_name, overrides) => {
      const user = userEvent.setup();
      renderHeader(svc({ ...free, ...overrides } as Partial<ServiceView>));
      await user.click(await screen.findByRole("button", { name: "Connect" }));
      expect(screen.getAllByText("app-x1y2:8080").length).toBeGreaterThan(0);
      expect(screen.queryByText(note)).not.toBeInTheDocument();
    });
  });

  it("omits the Internal section for a non-addressable service", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ type: "background_worker", internalAddress: null }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(screen.queryByText("Internal")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy internal address" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/same region can communicate internally/),
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

  // w4/143: the reason used to sit in a hover-only title; it is visible text
  // now, with a link to the remedy the user can take.
  it("says a Free service needs a paid compute plan for SSH, and links to the plan", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ sshAddress: null, plan: "free" }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(
      screen.getByText(
        "SSH needs a paid compute plan. Free services don't include shell access.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Change compute plan" }),
    ).toHaveAttribute("href", "/services/app/compute");
  });

  // w4/643: every other action is disabled while suspended, so the header
  // offers Resume, through the same gated verb as the Settings card; since
  // w3/107 it is the notice's Resume service, as on Render.
  it("offers Resume service in a suspended service's notice, and resumes", async () => {
    const user = userEvent.setup();
    const onRun = vi.fn().mockResolvedValue({ status: "success" });
    permissionQuery.mockResolvedValue({
      data: {
        serverActions: [
          {
            action: "resume",
            outcome: "allowed",
            reason: null,
            precondition: null,
          },
        ],
      },
    });
    const service = svc({ suspended: true, plan: "starter" });
    renderHeader(service, { onRun });

    await user.click(
      await screen.findByRole("button", { name: "Resume service" }),
    );

    await waitFor(() => expect(onRun).toHaveBeenCalledWith("resume", service));
  });

  it("offers no Resume on a running service", async () => {
    renderHeader(svc({ plan: "starter" }), { onRun: vi.fn() });

    expect(
      await screen.findByRole("button", { name: "Connect" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Resume service" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  // w4/m183: the service layout keeps this header mounted across its tabs, so
  // a remedy link used to leave the modal menu (and its body pointer lock)
  // open over the page it recommended.
  it.each([[{ plan: "free" }, "Change compute plan", "compute page"]])(
    "closes Connect when its %o remedy navigates",
    async (overrides, linkName, destination) => {
      const user = userEvent.setup();
      const rootRoute = createRootRoute({
        component: () => (
          <>
            <ServiceDetailHeader
              service={svc({ sshAddress: null, ...overrides })}
              pending={null}
            />
            <Outlet />
          </>
        ),
      });
      const page = (path: string, text: string) =>
        createRoute({
          getParentRoute: () => rootRoute,
          path,
          component: () => <p>{text}</p>,
        });
      const router = createRouter({
        routeTree: rootRoute.addChildren([
          page("/", "overview page"),
          page("/services/$serviceId/compute", "compute page"),
        ]),
        history: createMemoryHistory({ initialEntries: ["/"] }),
        context: { client: {} as never, session: null },
      });
      render(<RouterProvider router={router} />);

      await user.click(await screen.findByRole("button", { name: "Connect" }));
      await user.click(screen.getByRole("link", { name: linkName }));

      expect(await screen.findByText(destination)).toBeInTheDocument();
      await waitFor(() =>
        expect(screen.queryByText("SSH isn't available")).toBeNull(),
      );
      expect(document.body.style.pointerEvents).not.toBe("none");
    },
  );

  // w4/512: Connect was a dropdown menu with no menu items, so focus stayed on
  // the menu container and Tab never reached the Copy buttons.
  it("lets the keyboard reach Connect's Copy buttons and Escape return to Connect", async () => {
    const user = userEvent.setup();
    renderHeader(
      svc({
        internalAddress: "web-x1y2:3000",
        sshAddress: "srv-example@ssh.bex.co",
        outboundIps: { type: "shared", ips: ["203.0.113.7"] },
      }),
    );

    const connect = await screen.findByRole("button", { name: "Connect" });
    connect.focus();
    await user.keyboard("{Enter}");
    const panel = await screen.findByRole("dialog", { name: "Connect" });

    // Focus lands inside the panel, which is a dialog rather than a menu, and
    // its controls are ordinary tab stops (a menu without items kept focus on
    // its container and gave them none). jsdom cannot replay the portal's tab
    // order, so assert the stops themselves.
    expect(panel).toContainElement(document.activeElement as HTMLElement);
    expect(screen.queryByRole("menu")).toBeNull();
    for (const name of [
      "Copy internal address",
      "Copy SSH command",
      "Copy outbound IP addresses",
    ]) {
      const copy = within(panel).getByRole("button", { name });
      expect(copy).not.toHaveAttribute("tabindex", "-1");
      copy.focus();
      expect(copy).toHaveFocus();
    }

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(connect).toHaveFocus();
  });

  // w4/m711: Render lists a service's outbound IPs in Connect → Outbound, after
  // SSH, only while the list has an address, and never for a static site.
  describe("Connect → Outbound", () => {
    const ips = ["203.0.113.7", "198.51.100.24"];
    const lead =
      "Network requests from your service to the public internet will come from one of the following IP addresses or ranges.";

    it.each([
      ["web_service", {}],
      ["private_service", { url: null, internalAddress: "app-x1y2:8080" }],
      ["background_worker", { url: null }],
    ])(
      "lists a %s's outbound IPs under Render's title and words",
      async (type, overrides) => {
        const user = userEvent.setup();
        renderHeader(
          svc({
            type,
            sshAddress: "srv-example@ssh.bex.co",
            outboundIps: { type: "shared", ips },
            ...overrides,
          } as Partial<ServiceView>),
        );
        await user.click(
          await screen.findByRole("button", { name: "Connect" }),
        );
        const panel = screen.getByRole("dialog", { name: "Connect" });

        const list = within(panel).getByRole("list", {
          name: "Outbound IP Addresses",
        });
        expect(
          within(list)
            .getAllByRole("listitem")
            .map((item) => item.textContent),
        ).toEqual(ips);
        expect(within(panel).getByText(lead)).toBeInTheDocument();
        // bex's own line on where the addresses come from, under the key
        // w21/m616 rewrites when they become a fixed set.
        expect(
          within(panel).getByText(/tenant pool's current node public IPs/),
        ).toBeInTheDocument();
        // After SSH, as Render orders the tabs.
        const ssh = within(panel).getByText("ssh srv-example@ssh.bex.co");
        expect(
          ssh.compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING,
        ).toBeTruthy();
      },
    );

    it("copies the whole list, one address per line", async () => {
      const user = userEvent.setup();
      renderHeader(svc({ outboundIps: { type: "shared", ips } }));
      await user.click(await screen.findByRole("button", { name: "Connect" }));

      await user.click(
        screen.getByRole("button", { name: "Copy outbound IP addresses" }),
      );

      expect(await navigator.clipboard.readText()).toBe(
        "203.0.113.7\n198.51.100.24",
      );
    });

    it.each([
      ["an empty list", { type: "shared", ips: [] }],
      ["a list the read did not select", null],
    ])("shows no Outbound section for %s", async (_name, outboundIps) => {
      const user = userEvent.setup();
      renderHeader(svc({ sshAddress: "srv-example@ssh.bex.co", outboundIps }));
      await user.click(await screen.findByRole("button", { name: "Connect" }));

      const panel = screen.getByRole("dialog", { name: "Connect" });
      expect(
        within(panel).getByText("ssh srv-example@ssh.bex.co"),
      ).toBeInTheDocument();
      expect(within(panel).queryByText("Outbound IP Addresses")).toBeNull();
      expect(within(panel).queryByText(lead)).toBeNull();
      expect(
        screen.queryByRole("button", { name: "Copy outbound IP addresses" }),
      ).toBeNull();
    });

    it("gives a static site no Connect menu, so no outbound IPs", async () => {
      renderHeader(
        svc({
          type: "static_site",
          plan: null,
          outboundIps: { type: "shared", ips },
        }),
      );
      expect(
        await screen.findByRole("button", { name: "Manual Deploy" }),
      ).toBeVisible();
      expect(screen.queryByRole("button", { name: "Connect" })).toBeNull();
      expect(screen.queryByText("203.0.113.7")).toBeNull();
    });
  });

  it("keeps the general explanation, visibly, when the user holds no remedy", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ sshAddress: null, plan: "starter" }));

    await user.click(await screen.findByRole("button", { name: "Connect" }));

    expect(
      screen.getByText(
        "SSH requires a running paid web, private, or background service and an active gateway.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Resume service" })).toBeNull();
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
      screen.getByRole("link", { name: "github.com · bex-co / hello-go" }),
    ).toHaveAttribute("href", "https://github.com/bex-co/hello-go/tree/main");
    expect(screen.getByText("main")).toBeInTheDocument();

    // instance-type chip deep-links to the plan tab
    expect(screen.getByRole("link", { name: "Starter" })).toHaveAttribute(
      "href",
      "/services/app/compute",
    );

    // the bex-native facts line, incl. the platform-host slug (w4/m20/t002)
    expect(screen.getByText("Slug")).toBeInTheDocument();
    expect(screen.getByText("app-x1y2")).toBeInTheDocument();
    expect(screen.getByText("Instances")).toBeInTheDocument();
    expect(screen.getByText("Revision")).toBeInTheDocument();
    expect(screen.getByText("r1")).toBeInTheDocument();
  });

  // w4/547: the Instances fact counts what runs, as the instances API does:
  // none while suspended or asleep, with the count it returns to, and none
  // when no release has gone live (a failed first build).
  it.each([
    ["running", {}, "2"],
    [
      "suspended",
      { suspended: true, phase: "Hibernated" },
      "0 (2 when resumed)",
    ],
    ["sleeping", { phase: "Hibernated", plan: "free" }, "0 (2 when awake)"],
    // A paid service never sleeps, so no wake is promised.
    ["parked paid", { phase: "Hibernated" }, "0"],
    ["never-live", { phase: "Failed", revision: "" }, "0"],
    [
      "never-live suspended",
      { suspended: true, phase: "Hibernated", revision: "" },
      "0",
    ],
  ] as const)(
    "counts a %s service's running instances",
    async (_, overrides, want) => {
      renderHeader(svc({ replicas: 2, ...overrides }));
      const term = await screen.findByText("Instances");
      expect(term.nextElementSibling).toHaveTextContent(want);
    },
  );

  it("omits the Instances fact for a static site (no runtime instances, w5/m48)", async () => {
    renderHeader(svc({ type: "static_site" }));

    // The rest of the facts line stays; only the meaningless instance count goes.
    expect(await screen.findByText("Slug")).toBeInTheDocument();
    expect(screen.getByText("Revision")).toBeInTheDocument();
    expect(screen.queryByText("Instances")).not.toBeInTheDocument();
  });

  it("omits the Instances fact for a cron job, which runs nothing between runs (w4/606)", async () => {
    renderHeader(svc({ type: "cron_job", replicas: 1, revision: "rev-1" }));

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
    expect(
      screen.queryByRole("link", { name: "Starter" }),
    ).not.toBeInTheDocument();
    // A compute type with the same plan still shows the chip.
    renderHeader(svc({ type: "web_service", plan: "starter" }));
    expect(
      await screen.findByRole("link", { name: "Starter" }),
    ).toHaveAttribute("href", "/services/app/compute");
  });

  // w3/m554: a cron job has no Compute page. Render's plan badge links its
  // Settings' Compute section (`settings#compute`), and bex's does too.
  it("links a cron job's plan chip to its Settings' Compute section", async () => {
    renderHeader(
      svc({ type: "cron_job", plan: "starter", schedule: "*/5 * * * *" }),
    );

    expect(
      await screen.findByRole("link", { name: "Starter" }),
    ).toHaveAttribute("href", "/services/app/settings#compute");
  });

  it("shows a bound image credential by name and links to its settings panel", async () => {
    renderHeader(
      svc({ repo: null, branch: null, registryCredentialId: "rgc-private" }),
    );

    expect(await screen.findByText("Registry credential:")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Private GHCR" })).toHaveAttribute(
      "href",
      "/workspace/settings#registry-credentials",
    );
  });

  it("does not add credential metadata to an unbound service", async () => {
    renderHeader(svc({ registryCredentialId: null }));

    await screen.findByRole("heading", { name: "app" });
    expect(screen.queryByText("Registry credential:")).not.toBeInTheDocument();
  });

  // w3/m555: Render's facts under the source: "Runs <command> <schedule in
  // words>", then the last successful run, and no next run.
  it("says what a cron job runs and when, in Render's words, where the URL was", async () => {
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        plan: null,
        schedule: "0 9 * * *",
        command: "node report.js",
      }),
    );

    const command = await screen.findByText("node report.js");
    expect(command.tagName).toBe("CODE");
    const line = command.closest("p")!;
    expect(line).toHaveTextContent("Runs node report.js at 09:00 AM UTC");
    expect(screen.queryByText("https://app.onbex.co")).not.toBeInTheDocument();
    expect(screen.queryByText(/Schedule/)).not.toBeInTheDocument();
  });

  // w3/m643: every schedule in cronstrue's words, as Render's header has it.
  it.each([
    ["* * * * *", "every minute"],
    ["*/5 * * * *", "every 5 minutes"],
    ["0 * * * *", "every hour"],
    ["30 13 * * 1", "at 01:30 PM UTC, only on Monday"],
    ["0 9 * * 1-5", "at 09:00 AM UTC, Monday through Friday"],
    ["*/40 * * * *", "every 40 minutes"],
  ])("words %s as %s", async (schedule, words) => {
    renderHeader(
      svc({ type: "cron_job", url: null, schedule, command: "./job" }),
    );
    expect(await screen.findByText(words)).toBeInTheDocument();
  });

  // Render's header reads nothing after the command for a schedule its gate
  // refuses, and shows no expression either.
  it("shows nothing after the command for a schedule Render reads nothing for", async () => {
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "*-5 * * * *",
        command: "./job",
      }),
    );
    const command = await screen.findByText("./job");
    expect(command.closest("p")!.textContent!.trim()).toBe("Runs ./job");
    expect(screen.queryByText("*-5 * * * *")).toBeNull();
  });

  it("names a native cron job's start command when nothing overrides it", async () => {
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "* * * * *",
        command: null,
        runtime: "node",
        startCommand: "npm run report",
      }),
    );
    expect(await screen.findByText("npm run report")).toBeInTheDocument();
  });

  // Render's bundle words a Docker or image cron job with no command as the
  // Dockerfile's CMD, the image-backed one included.
  it.each([
    ["an image", { runtime: "image", dockerfilePath: null }, "Dockerfile"],
    [
      "a Dockerfile build",
      { runtime: "docker", dockerfilePath: "jobs/Dockerfile" },
      "jobs/Dockerfile",
    ],
  ])(
    "says %s cron job with no command runs CMD in its Dockerfile",
    async (_, source, dockerfile) => {
      renderHeader(
        svc({
          type: "cron_job",
          url: null,
          schedule: "* * * * *",
          command: null,
          ...source,
        }),
      );
      const cmd = await screen.findByText("CMD");
      expect(cmd.tagName).toBe("CODE");
      expect(screen.getByText(dockerfile).tagName).toBe("CODE");
      expect(cmd.closest("p")).toHaveTextContent(
        `Runs CMD in ${dockerfile} every minute`,
      );
    },
  );

  // w3/m643: Render's `LL [at] LT z` in the viewer's zone. On UTC, the
  // suite's zone, the date has no tooltip and is no focusable term.
  it("shows a cron job's last successful run with its zone, no tooltip on UTC, and no next run", async () => {
    const user = userEvent.setup();
    const lastRun = "2026-11-21T17:00:00Z";
    const { container } = renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "0 9 * * *",
        command: "./job",
        lastSuccessfulRunAt: lastRun,
        nextRunAt: "2026-11-22T09:00:00Z",
      }),
    );

    const label = await screen.findByText(/^Last successful run/);
    const instant = label.querySelector("time")!;
    expect(instant).toHaveAttribute("dateTime", lastRun);
    await waitFor(() =>
      expect(label).toHaveTextContent(
        "Last successful run November 21, 2026 at 5:00 PM UTC",
      ),
    );
    expect(label.querySelector("[tabindex]")).toBeNull();
    await user.hover(instant);
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(screen.queryByText(/Next run/)).not.toBeInTheDocument();
    expect(
      container.querySelector('time[datetime="2026-11-22T09:00:00Z"]'),
    ).toBeNull();
  });

  // Off UTC the date carries the viewer's zone and, hovered or focused,
  // Render's tooltip: the instant in UTC as `MMM D hh:mm:ss A`.
  it("shows a viewer in Los Angeles the date in PST and the UTC instant in a tooltip", async () => {
    const previous = process.env.TZ;
    process.env.TZ = "America/Los_Angeles";
    try {
      const user = userEvent.setup();
      renderHeader(
        svc({
          type: "cron_job",
          url: null,
          schedule: "0 9 * * *",
          command: "./job",
          lastSuccessfulRunAt: "2026-11-21T17:00:00Z",
        }),
      );

      const label = await screen.findByText(/^Last successful run/);
      await waitFor(() =>
        expect(label).toHaveTextContent(
          "Last successful run November 21, 2026 at 9:00 AM PST",
        ),
      );
      const term = label.querySelector("[tabindex='0']")!;
      expect(term).toHaveClass("hover:underline");
      expect(term).toHaveTextContent("November 21, 2026 at 9:00 AM PST");
      await user.hover(term);
      const tooltip = await screen.findByRole("tooltip");
      expect(tooltip).toHaveTextContent(/^UTC: Nov 21 05:00:00 PM$/);
      const time = document.querySelector('[data-slot="tooltip-content"] time');
      expect(time).toHaveAttribute("dateTime", "2026-11-21T17:00:00Z");
      expect(time).toHaveTextContent("UTC: Nov 21 05:00:00 PM");
    } finally {
      if (previous === undefined) delete process.env.TZ;
      else process.env.TZ = previous;
    }
  });

  it("advances a cron job's creation age for two minutes with unchanged service facts", async () => {
    const now = Date.parse("2026-10-02T12:00:00Z");
    const clock = vi.spyOn(Date, "now").mockReturnValue(now);
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
    const createdAt = new Date(now - 4 * 60_000).toISOString();
    const service = svc({
      type: "cron_job",
      url: null,
      plan: null,
      schedule: "*/5 * * * *",
      createdAt,
    });
    const view = renderHeader(service);
    try {
      await screen.findByText("No successful runs yet.");
      const created = view.container.querySelector(
        `time[datetime="${createdAt}"]`,
      );
      expect(created).toHaveTextContent("4m");
      for (const minutes of [1, 2]) {
        clock.mockReturnValue(now + minutes * 60_000);
        act(() => vi.advanceTimersByTime(60_000));
        expect(created).toHaveTextContent(`${4 + minutes}m`);
        expect(created).toHaveAttribute("dateTime", createdAt);
      }
      expect(triggerDeploy).not.toHaveBeenCalled();
      expect(triggerCronRun).not.toHaveBeenCalled();
    } finally {
      view.unmount();
      vi.useRealTimers();
      clock.mockRestore();
    }
  });

  // w3/m555: the date sits in a <p>, and its placeholder was a <div>, which
  // the HTML parser moves out of a paragraph. Every production load of a cron
  // job with a successful run then threw React #418.
  it("hydrates a cron job's last successful run without a mismatch", async () => {
    const router = headerRouter(
      svc({
        type: "cron_job",
        url: null,
        schedule: "0 9 * * *",
        command: "./job",
        lastSuccessfulRunAt: "2026-11-21T09:00:00Z",
      }),
    );
    await router.load();
    const now = Date.parse("2026-11-25T14:30:00Z");
    const probe = hydrateAcrossBoundary(<RouterProvider router={router} />, {
      serverNow: now,
      clientNow: now,
    });
    expect(probe.html).toContain('data-header-item="cron-last-run"');
    expect(probe.recovered).toEqual([]);
  });

  // w3/m643: the zone and the tooltip are the viewer's, so the server (UTC)
  // renders neither and the browser draws both once hydrated.
  it("hydrates a Los Angeles viewer's last run from a UTC server without a mismatch", async () => {
    const service = svc({
      type: "cron_job",
      url: null,
      schedule: "0 9 * * *",
      command: "./job",
      lastSuccessfulRunAt: "2026-11-21T17:00:00Z",
    });
    let hydrated = "";
    let term: Element | null = null;
    const probe = await hydrateAcrossEnvironments(
      async () => {
        const router = headerRouter(service);
        await router.load();
        return <RouterProvider router={router} />;
      },
      {
        server: { timeZone: "UTC", language: "en" },
        client: { timeZone: "America/Los_Angeles", language: "en" },
        inspect: (container) => {
          const line = container.querySelector(
            '[data-header-item="cron-last-run"]',
          )!;
          hydrated = line.textContent ?? "";
          term = line.querySelector("[tabindex='0']");
        },
      },
    );
    const server = document.createElement("div");
    server.innerHTML = probe.html;
    expect(
      server.querySelector('[data-header-item="cron-last-run"]'),
    ).toHaveTextContent(/^Last successful run$/);
    expect(probe.recovered).toEqual([]);
    expect(probe.failed).toEqual([]);
    expect(hydrated).toBe(
      "Last successful run November 21, 2026 at 9:00 AM PST",
    );
    expect(term).not.toBeNull();
  });

  it("says a cron job has no successful runs yet", async () => {
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "*/5 * * * *",
        command: "./job",
        lastSuccessfulRunAt: null,
      }),
    );
    expect(
      await screen.findByText("No successful runs yet."),
    ).toBeInTheDocument();
  });

  // w3/m555: a running cron job's header has one action, Trigger Run: no
  // Connect (a cron job has no SSH) and no Manual Deploy menu (its build is
  // Manual Build on the Deploys page).
  it("gives a running cron job Trigger Run and no other action", async () => {
    const user = userEvent.setup();
    renderHeader(svc({ type: "cron_job", url: null, schedule: "* * * * *" }), {
      onRun: vi.fn(),
    });

    const button = await screen.findByRole("button", { name: "Trigger Run" });
    expect(button).toBeEnabled();
    expect(screen.queryByRole("button", { name: "Connect" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Manual Deploy" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Resume" })).toBeNull();

    await user.click(button);
    expect(triggerCronRun).toHaveBeenCalledOnce();
    // Render's click has no confirm.
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("keeps Trigger Run's place while it runs", async () => {
    cronTriggering = true;
    renderHeader(svc({ type: "cron_job", url: null, schedule: "* * * * *" }));
    expect(
      await screen.findByRole("button", { name: "Trigger Run" }),
    ).toBeDisabled();
  });

  it("disables Trigger Run on a repo-backed cron job whose build never went live, linking Deploys", async () => {
    const user = userEvent.setup();
    renderHeader(
      svc({ type: "cron_job", url: null, schedule: "* * * * *", revision: "" }),
    );

    const button = await screen.findByRole("button", { name: "Trigger Run" });
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    const tip = await screen.findByRole("tooltip");
    expect(tip).toHaveTextContent(
      "No successful build detected. View Deploys.",
    );
    expect(within(tip).getByRole("link", { name: "Deploys" })).toHaveAttribute(
      "href",
      "/services/app/deploys",
    );
    expect(triggerCronRun).not.toHaveBeenCalled();
  });

  it("lets an image-backed cron job run before any release went live", async () => {
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "* * * * *",
        repo: null,
        branch: null,
        imagePath: "docker.io/library/busybox:1.36",
        revision: "",
      }),
    );
    expect(
      await screen.findByRole("button", { name: "Trigger Run" }),
    ).toBeEnabled();
  });

  it.each([
    [
      "a role without the permission",
      { outcome: "denied" as const },
      "Your role can’t perform this action. Ask a workspace admin.",
    ],
    [
      "a precondition the server reports",
      { precondition: "billing_blocked" },
      "Billing needs attention before this action can run. Resolve it in Billing.",
    ],
    // w3/m556: Render's own words for a suspended cron job.
    [
      "a suspended cron job",
      { precondition: "suspended" },
      "Resume this cron job to trigger a run.",
    ],
  ])("disables Trigger Run for %s, with its reason", async (_, gate, text) => {
    resourceActions = mockAllowedResourceActions("app", { cron_run_now: gate });
    const user = userEvent.setup();
    renderHeader(svc({ type: "cron_job", url: null, schedule: "* * * * *" }));

    const button = await screen.findByRole("button", { name: "Trigger Run" });
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(text);
  });

  // w3/107: Render's banner titles a suspension by who made it (`Lr`), and
  // an owner's by the current workspace's plan (w3/m643): "Suspended by you"
  // on Hobby, "Manually suspended" on every other plan.
  it.each([
    ["hobby", { manuallySuspended: true }, "Suspended by you"],
    ["pro", { manuallySuspended: true }, "Manually suspended"],
    ["scale", { manuallySuspended: true }, "Manually suspended"],
    [
      "hobby",
      { crashLoopSuspended: true },
      "This service has been suspended in response to repeated crashes",
    ],
    ["hobby", { billingSuspended: true }, "Suspended by bex"],
    ["hobby", {}, "Suspended"],
  ])(
    "on %s titles the notice of a suspension by %o %j",
    async (plan, by, title) => {
      workspacePlan = plan;
      renderHeader(
        svc({
          type: "cron_job",
          url: null,
          schedule: "* * * * *",
          suspended: true,
          ...by,
        }),
        { onRun: vi.fn() },
      );
      const notice = await screen.findByRole("status");
      expect(notice).toHaveTextContent(
        `${title} You are not billed for suspended services. Resume service`,
      );
    },
  );

  // w3/107: Render's Server page passes its header actions only while the
  // service runs (`state === "Running"`), the same rule as a cron job's, and
  // puts the same banner under the facts: no Connect, Resume or Manual Deploy
  // on any type while suspended.
  it.each([
    ["web_service", {}],
    ["private_service", { url: null, internalAddress: "app-x1y2:10000" }],
    ["background_worker", { url: null }],
    ["static_site", { plan: null }],
    ["cron_job", { url: null, schedule: "* * * * *" }],
  ])(
    "gives a suspended %s no header action and the notice",
    async (type, overrides) => {
      const user = userEvent.setup();
      const onRun = vi.fn().mockResolvedValue({ status: "success" });
      permissionQuery.mockResolvedValue({
        data: {
          serverActions: [
            {
              action: "resume",
              outcome: "allowed",
              reason: null,
              precondition: null,
            },
          ],
        },
      });
      const service = svc({
        type,
        ...overrides,
        suspended: true,
        manuallySuspended: true,
      });
      renderHeader(service, { onRun });

      const notice = await screen.findByRole("status");
      expect(notice).toHaveTextContent(
        "Suspended by you You are not billed for suspended services. Resume service",
      );
      // The action slot is empty, so `empty:hidden` drops it from the row.
      expect(
        document.querySelector(".md\\:max-lg\\:col-start-2.empty\\:hidden"),
      ).toBeEmptyDOMElement();
      for (const name of [
        "Connect",
        "Manual Deploy",
        "Trigger Run",
        "Resume",
      ]) {
        expect(screen.queryByRole("button", { name })).toBeNull();
      }

      await user.click(
        within(notice).getByRole("button", { name: "Resume service" }),
      );
      await waitFor(() =>
        expect(onRun).toHaveBeenCalledWith("resume", service),
      );
    },
  );

  // While it runs, each type keeps its actions and has no notice.
  it.each([
    ["web_service", {}, ["Connect", "Manual Deploy"]],
    ["static_site", { plan: null }, ["Manual Deploy"]],
    ["cron_job", { url: null, schedule: "* * * * *" }, ["Trigger Run"]],
  ])("keeps a running %s's actions", async (type, overrides, actions) => {
    renderHeader(svc({ type, ...overrides }), { onRun: vi.fn() });
    for (const name of actions) {
      expect(await screen.findByRole("button", { name })).toBeVisible();
    }
    expect(screen.queryByRole("status")).toBeNull();
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

    await waitFor(() => expect(triggerDeploy).toHaveBeenCalledWith("app"));
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

  it("restarts through the Manual Deploy dropdown via restartServer, after the shared confirmation (w1/m148)", async () => {
    triggerDeploy.mockClear();
    restartServer.mockClear();
    const user = userEvent.setup();
    renderHeader(svc());

    await user.click(
      await screen.findByRole("button", { name: "Manual Deploy" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Restart service" }),
    );

    // Restart keeps the running commit; a bare triggerDeploy would build the
    // branch head. It asks the same question as the services-list row.
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "restarts on the commit or image it is running now",
    );
    await user.click(screen.getByRole("button", { name: "Restart" }));
    await waitFor(() => expect(restartServer).toHaveBeenCalledWith("app"));
    expect(triggerDeploy).not.toHaveBeenCalled();
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
      screen.queryByRole("button", { name: /^Actions for / }),
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

// w7/m79: a web service with no public address used to render nothing at all in
// this slot, so "will never be reachable" looked exactly like "still starting".
// The platform now says which it is, and what to do about it.
describe("public routing notice", () => {
  const notice =
    "this service has no public address: the platform subdomain is not available on this " +
    "installation, and no custom domain is attached. Add a custom domain to serve it publicly.";

  it("explains a missing public address instead of showing nothing", async () => {
    renderHeader(svc({ url: null, publicRoutingNotice: notice }));

    expect(await screen.findByText(/no public address/i)).toBeInTheDocument();
    // The remedy has to be in reach, or the notice is just a nicer silence.
    expect(screen.getByText(/custom domain/i)).toBeInTheDocument();
  });

  it("prefers the real url when the service is routed", async () => {
    renderHeader(
      svc({ url: "https://app.onbex.co", publicRoutingNotice: null }),
    );

    expect(
      await screen.findByRole("link", { name: "https://app.onbex.co" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/no public address/i)).not.toBeInTheDocument();
  });

  it("stays silent when there is no url and nothing to explain", async () => {
    // A worker, a cron job, or an owner who switched their own subdomain off:
    // the backend sends no notice, and the header must not invent one.
    renderHeader(svc({ url: null, publicRoutingNotice: null }));

    expect(
      await screen.findByRole("heading", { name: "app" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/no public address/i)).not.toBeInTheDocument();
  });
});

it("shows the immutable Blueprint name separately from a renamed label and suffixed slug", async () => {
  const user = userEvent.setup();
  renderHeader(
    svc({
      name: "Friendly label",
      immutableName: "original-api",
      slug: "original-api-x123",
    }),
  );
  expect(await screen.findByText("Blueprint name")).toBeInTheDocument();
  expect(screen.getByText("original-api-x123")).toBeInTheDocument();
  const immutable = screen.getByText("original-api");
  await user.hover(immutable);
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "Use this immutable name in services[].name in your Blueprint manifest.",
  );
});

it("omits the extra Blueprint name when it is already the display label", async () => {
  renderHeader(
    svc({
      name: "original-api",
      immutableName: "original-api",
      slug: "original-api-x123",
    }),
  );
  expect(await screen.findByText("original-api-x123")).toBeInTheDocument();
  expect(screen.queryByText("Blueprint name")).not.toBeInTheDocument();
});

// w6/m148: an image-port private service publishes every TCP port its image
// declares. The header lists them beside the Service Address with the primary
// (the `<slug>:<port>` suffix) marked.
describe("open ports beside the Service Address (w6/m148)", () => {
  const tcp = (...ports: number[]) =>
    ports.map((port) => ({ port, protocol: "TCP" }));
  const clickhouse = {
    type: "private_service",
    url: null,
    internalAddress: "ch-x1y2:8123",
    port: 8123,
    openPorts: tcp(8123, 9000, 9009),
  } satisfies Partial<ServiceView>;

  function expectPorts(list: HTMLElement) {
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent("8123");
    expect(within(items[0]).getByText("primary")).toBeInTheDocument();
    for (const [item, port] of [
      [items[1], "9000"],
      [items[2], "9009"],
    ] as const) {
      expect(item).toHaveTextContent(port);
      expect(within(item).queryByText("primary")).not.toBeInTheDocument();
    }
  }

  it("lists every open port beside the address, marking the primary", async () => {
    renderHeader(svc(clickhouse));

    expectPorts(await screen.findByRole("list", { name: "Open ports" }));
    expect(screen.getByText("Service Address")).toBeInTheDocument();
    expect(screen.getByText("ch-x1y2:8123")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy internal address" }),
    ).toBeInTheDocument();
  });

  it("repeats the open ports under Connect → Internal", async () => {
    const user = userEvent.setup();
    renderHeader(svc(clickhouse));

    await user.click(await screen.findByRole("button", { name: "Connect" }));
    const menu = screen.getByRole("dialog", { name: "Connect" });

    expect(within(menu).getByText("ch-x1y2:8123")).toBeInTheDocument();
    expectPorts(within(menu).getByRole("list", { name: "Open ports" }));
  });

  // w9/104: before its image is read, bex-api reads the service on Render's
  // default port, as Render's create does, so the address is copyable at once.
  it("shows an unread image's default address with nothing beside it", async () => {
    const user = userEvent.setup();
    renderHeader(
      svc({
        type: "private_service",
        url: null,
        internalAddress: "p-x1y2:10000",
        port: 10000,
        openPorts: tcp(10000),
      }),
    );

    expect(await screen.findByText("p-x1y2:10000")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy internal address" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Open ports")).not.toBeInTheDocument();

    await user.click(await screen.findByRole("button", { name: "Connect" }));
    const menu = screen.getByRole("dialog", { name: "Connect" });
    expect(within(menu).getByText("p-x1y2:10000")).toBeInTheDocument();
  });

  it("keeps a single-port private service's Service Address as it was", async () => {
    renderHeader(
      svc({
        type: "private_service",
        url: null,
        internalAddress: "app-x1y2:8080",
        port: 8080,
        openPorts: tcp(8080),
      }),
    );

    expect(await screen.findByText("app-x1y2:8080")).toBeInTheDocument();
    expect(screen.queryByText("Open ports")).not.toBeInTheDocument();
    expect(screen.queryByText("primary")).not.toBeInTheDocument();
  });

  it.each([
    [
      "a background worker",
      {
        type: "background_worker",
        url: null,
        internalAddress: null,
        port: null,
        openPorts: null,
      },
    ],
    [
      "a web service",
      { internalAddress: "app-x1y2:3000", port: 3000, openPorts: tcp(3000) },
    ],
  ] satisfies [string, Partial<ServiceView>][])(
    "adds no port facts for %s",
    async (_name, overrides) => {
      const user = userEvent.setup();
      renderHeader(svc(overrides));

      await screen.findByRole("heading", { name: "app" });
      expect(screen.queryByText("Service Address")).not.toBeInTheDocument();

      await user.click(await screen.findByRole("button", { name: "Connect" }));
      expect(screen.queryByText("Open ports")).not.toBeInTheDocument();
    },
  );
});

// w3/085: the pending header drew its Service ID and URL rows 28px tall
// (`h-7` rows, `size-7` copy squares) where each ready row is as tall as its
// 32px copy button, so at 1440 the header grew 4px when the service read
// landed and every tab under it moved down. Below `lg` (or on a coarse
// pointer) the touch rule grows that button to 44px, so the square has to
// grow with it. The skeleton is read against the buttons the ready header
// renders: a resized copy button fails here until its slot follows.
describe("ServiceDetailHeaderSkeleton second row", () => {
  const SIZE = /^(?:size|h|min-h|max-h)-/;
  const sizes = (el: Element) =>
    el
      .getAttribute("class")
      ?.split(/\s+/)
      .filter((c) => SIZE.test(c)) ?? [];

  it.each([
    ["service-id", "Copy service ID"],
    ["service-url", "Copy service URL"],
  ])(
    "draws the %s row at its ready copy button's height",
    async (slot, label) => {
      renderHeader(svc());
      const button = await screen.findByRole("button", { name: label });
      // The ready row sets no height: its copy button is its tallest item.
      expect(sizes(button.parentElement!)).toEqual([]);
      expect(sizes(button)).toEqual(["size-8"]);

      const { container } = render(<ServiceDetailHeaderSkeleton />);
      const row = container.querySelector(`[data-skeleton-item="${slot}"]`);
      expect(row).not.toBeNull();
      // No fixed height of its own, which would hold the row at a size the
      // square no longer has.
      expect(sizes(row!)).toEqual([]);
      const square = row!.lastElementChild!;
      expect(sizes(square)).toEqual(sizes(button));
      // The ready button is a `button.size-8` in the page's touch region, so
      // it grows to 44px below `lg`; the square must take the same rule.
      expect(square).toHaveAttribute("data-touch-spacer");
      // Nothing else in the row is taller than the square: its words, or a
      // 16px bar where the frame does not know them.
      for (const child of Array.from(row!.children).slice(0, -1)) {
        expect(sizes(child).filter((size) => size !== "h-4")).toEqual([]);
      }
    },
  );
});

// w3/094: the second row wraps, so a slot the ready header lacks moves the
// band's bottom edge: at 1024 an image-backed service's Service ID and URL
// fit on one line, where the skeleton's repository slot pushed the URL to a
// second. Each source the cache can report draws the items its ready header
// renders, and nothing it does not.
describe("ServiceDetailHeaderSkeleton source items", () => {
  const drawn = (container: Element) =>
    Array.from(
      container.querySelectorAll(
        '[data-skeleton-item="repository"], [data-skeleton-item="registry-credential"]',
      ),
      (item) => item.getAttribute("data-skeleton-item"),
    );

  it.each([
    [
      "a Git service",
      svc(),
      {
        repository: true,
        registryCredential: false,
        cron: false,
        suspendedNotice: null,
      },
      ["repository"],
    ],
    [
      "an image-backed service",
      svc({ repo: "", branch: null }),
      {
        repository: false,
        registryCredential: false,
        cron: false,
        suspendedNotice: null,
      },
      [],
    ],
    [
      "an image pulled with a registry credential",
      svc({ repo: null, branch: null, registryCredentialId: "rgc-private" }),
      {
        repository: false,
        registryCredential: true,
        cron: false,
        suspendedNotice: null,
      },
      ["registry-credential"],
    ],
    [
      "a Git service whose Dockerfile pulls with a registry credential",
      svc({ registryCredentialId: "rgc-private" }),
      {
        repository: true,
        registryCredential: true,
        cron: false,
        suspendedNotice: null,
      },
      ["repository", "registry-credential"],
    ],
  ])("draws %s's source items", async (_, service, source, items) => {
    renderHeader(service);
    await screen.findByRole("heading", { name: "app" });
    const ready = [
      screen.queryByRole("link", { name: "github.com · bex-co / hello-go" })
        ? "repository"
        : null,
      screen.queryByText("Registry credential:") ? "registry-credential" : null,
    ].filter(Boolean);
    expect(ready).toEqual(items);

    const { container } = render(
      <ServiceDetailHeaderSkeleton source={source} />,
    );
    expect(drawn(container)).toEqual(items);
  });

  // On a phone the ready row is a grid: every item in the first column and
  // "Details" beside the first of them, 44px tall under the touch rule. An
  // auto-placed skeleton put the URL beside the registry credential instead,
  // and its band ended 28px short of the ready one at 390.
  it("places Details beside the first item on a phone, as the ready row does", async () => {
    const placement = (el: Element) =>
      el
        .getAttribute("class")
        ?.split(/\s+/)
        .filter((c) => /(?:^|:)(?:\[&>\*\]:)?(?:row|col)-start-/.test(c))
        .sort() ?? [];
    renderHeader(
      svc({ repo: null, branch: null, registryCredentialId: "rgc-private" }),
    );
    const ready = await screen.findByRole("button", { name: "Details" });

    const { container } = render(
      <ServiceDetailHeaderSkeleton
        source={{
          repository: false,
          registryCredential: true,
          cron: false,
          suspendedNotice: null,
        }}
      />,
    );
    const slot = container.querySelector(
      '[data-skeleton-item="details-toggle"]',
    )!;
    expect(placement(ready)).toEqual(["max-sm:col-start-2!", "row-start-1"]);
    expect(placement(slot)).toEqual(placement(ready));
    expect(placement(slot.parentElement!)).toEqual(
      placement(ready.parentElement!),
    );
    expect(slot).toHaveAttribute("data-touch-spacer");
  });

  it("draws a Git service's items while nothing cached has said", () => {
    const { container } = render(<ServiceDetailHeaderSkeleton source={null} />);
    expect(drawn(container)).toEqual(["repository"]);
  });
});

// w3/112: the second row wraps on its items' widths. At 640 the frame's fixed
// bars fit the URL beside the repository where the ready URL wraps under it,
// with "Details" beside it, and the frame ended 22px below the ready band. The
// frame now lays each item out in the ready one's words (the labels always;
// the id, repository, branch and URL from the cache) at the ready item's
// type and flex sizing, and each action as the ready button's own box.
describe("ServiceDetailHeaderSkeleton words (w3/112)", () => {
  const SOURCE = {
    repository: true,
    registryCredential: false,
    cron: false,
    suspendedNotice: null,
    id: "app",
    repo: "https://github.com/bex-co/hello-go.git",
    branch: "main",
    url: "https://app.onbex.co",
  };
  const slot = (container: Element, item: string) =>
    container.querySelector(`[data-skeleton-item="${item}"]`)!;
  // What an item reads, without its controls' accessible labels.
  const words = (el: Element) => {
    const copy = el.cloneNode(true) as Element;
    copy.querySelectorAll("button, svg").forEach((node) => node.remove());
    return copy.textContent!.replace(/\s+/g, " ").trim();
  };
  const invisibleWords = (el: Element) =>
    Array.from(el.querySelectorAll(".invisible"), (node) => node.textContent)
      .join("")
      .replace(/\s+/g, " ")
      .trim();
  // The classes that size an item: its flex behaviour and its type, not its
  // colour.
  const FLEX =
    /^(?:min-w-|max-w-|basis-|grow|shrink|truncate|gap-|text-(?:xs|sm)$|font-(?:mono|medium)$)/;
  const flex = (el: Element) =>
    [...el.classList].filter((c) => FLEX.test(c)).sort();

  it("lays the row's items out in the ready ones' words, at their sizes", async () => {
    renderHeader(svc());
    const details = await screen.findByRole("button", { name: "Details" });
    const repoLink = screen.getByRole("link", {
      name: "github.com · bex-co / hello-go",
    });
    const ready = {
      "service-id": screen.getByRole("button", { name: "Copy service ID" })
        .parentElement!,
      repository: repoLink.parentElement!,
      "service-url": screen.getByRole("button", { name: "Copy service URL" })
        .parentElement!,
      "details-toggle": details,
    };

    const { container } = render(
      <ServiceDetailHeaderSkeleton source={SOURCE} />,
    );
    for (const [item, el] of Object.entries(ready)) {
      const frame = slot(container, item);
      expect(words(frame)).toBe(words(el));
      expect(words(frame)).not.toBe("");
      // Laid out, never shown.
      expect(invisibleWords(frame)).toBe(words(frame));
    }
    // The repository and URL items shrink and truncate as the ready ones do,
    // so a long one keeps its line instead of pushing the next one down.
    for (const item of ["repository", "service-url"] as const) {
      expect(flex(slot(container, item))).toEqual(flex(ready[item]));
    }
    expect(slot(container, "repository").children[1]).toHaveClass(
      "min-w-0",
      "truncate",
    );
    expect(slot(container, "service-url").firstElementChild).toHaveClass(
      "min-w-0",
      "truncate",
    );
    // The branch is the ready badge's box, in its monospace.
    const branch = within(ready.repository).getByText("main");
    expect(branch).toHaveClass("font-mono");
    const reserved = within(slot(container, "repository") as HTMLElement)
      .getByText("main")
      .closest('[data-slot="badge"]')!;
    expect(reserved).toHaveClass("invisible");
    expect(flex(reserved).filter((c) => c !== "font-mono")).toEqual(
      flex(branch).filter((c) => c !== "font-mono"),
    );
    // The row's own type size and wrapping are the ready row's.
    const row = slot(container, "details-toggle").parentElement!;
    for (const c of ["text-sm", "sm:flex", "sm:flex-wrap", "sm:gap-x-6"]) {
      expect(details.parentElement).toHaveClass(c);
      expect(row).toHaveClass(c);
    }
  });

  it("keeps a bar where the cache has not said an item's words", () => {
    const { container } = render(<ServiceDetailHeaderSkeleton />);
    for (const item of ["repository", "service-url"]) {
      expect(words(slot(container, item))).toBe("");
    }
    expect(words(slot(container, "service-id"))).toBe("Service ID:");
    expect(words(slot(container, "details-toggle"))).toBe("Details");
  });

  it("reserves each action as the ready button's own box", async () => {
    renderHeader(svc());
    const connect = await screen.findByRole("button", { name: "Connect" });
    const deploy = screen.getByRole("button", { name: "Manual Deploy" });

    const { container } = render(
      <ServiceDetailHeaderSkeleton source={SOURCE} />,
    );
    for (const [item, ready] of [
      ["connect", connect],
      ["manual-deploy", deploy],
    ] as const) {
      const frame = slot(container, item);
      expect(frame).toHaveTextContent(ready.textContent!);
      // The ready button's classes, its padding beside a chevron included.
      expect(frame.firstElementChild).toHaveClass(...ready.classList);
      expect(frame.firstElementChild!.querySelector("svg")).toHaveClass(
        "size-3.5",
      );
      // It grows under the touch rule, as the ready button does.
      expect(frame).toHaveAttribute("data-touch-spacer");
    }
  });

  it("reserves a cron job's Trigger Run as its own box", async () => {
    renderHeader(svc({ type: "cron_job", url: null, schedule: "* * * * *" }));
    const ready = await screen.findByRole("button", { name: "Trigger Run" });
    const { container } = render(
      <ServiceDetailHeaderSkeleton source={{ ...SOURCE, cron: true }} />,
    );
    const frame = slot(container, "trigger-run");
    expect(frame).toHaveTextContent("Trigger Run");
    expect(frame.firstElementChild).toHaveClass(...ready.classList);
  });
});

// w3/m555: a cron job's frame is its own. Running: one action slot, Trigger
// Run's, and the two "Runs …" lines in the URL's place. Suspended: no action
// and the notice, its words laid out as the ready one's are.
describe("ServiceDetailHeaderSkeleton for a cron job", () => {
  const CRON = { repository: true, registryCredential: false, cron: true };
  const actionSlots = (container: Element) =>
    container.querySelectorAll(
      ".md\\:max-lg\\:col-start-2 > [data-touch-spacer]",
    ).length;

  it("reserves Trigger Run alone and the two facts lines while it runs", async () => {
    // The frame lays its last-run line out with this date (w3/m643).
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "* * * * *",
        lastSuccessfulRunAt: "2026-11-21T17:00:00Z",
      }),
    );
    const ready = await screen.findByRole("button", { name: "Trigger Run" });
    expect(ready.closest(".md\\:max-lg\\:col-start-2")!.children).toHaveLength(
      1,
    );
    const items = ["cron-runs", "cron-last-run"];

    const { container } = render(
      <ServiceDetailHeaderSkeleton
        source={{ ...CRON, suspendedNotice: null }}
      />,
    );
    expect(actionSlots(container)).toBe(1);
    expect(
      container.querySelector('[data-skeleton-item="trigger-run"]'),
    ).toHaveAttribute("data-touch-spacer");
    for (const item of items) {
      const ready = document.querySelector(`[data-header-item="${item}"]`)!;
      const slot = container.querySelector(`[data-skeleton-item="${item}"]`)!;
      // On a phone the last run spans both columns, beside nothing; from
      // `lg` each is its own line.
      const placement = (el: Element) =>
        [...el.classList].filter((c) => /col-span|basis/.test(c)).sort();
      expect(placement(slot)).toEqual(placement(ready));
    }
    // The schedule's words are one text-sm line: 20px.
    expect(
      container.querySelector('[data-skeleton-item="cron-runs"]'),
    ).toHaveClass("h-5");
    // The last run's line, its zone included, wraps at 390 in most
    // languages: the frame lays the ready line's words out invisibly, at its
    // size, so it wraps where the ready one does.
    const lastRun = container.querySelector(
      '[data-skeleton-item="cron-last-run"]',
    )!;
    await waitFor(() =>
      expect(lastRun).toHaveTextContent(
        document.querySelector('[data-header-item="cron-last-run"]')!
          .textContent!,
      ),
    );
    expect(lastRun).toHaveTextContent(
      "Last successful run November 21, 2026 at 5:00 PM UTC",
    );
    expect(lastRun).toHaveClass("text-sm");
    expect(lastRun.querySelector(".invisible")).not.toBeNull();
    expect(
      container.querySelector('[data-skeleton-item="service-url"]'),
    ).toBeNull();
    expect(
      container.querySelector('[data-skeleton-item="suspended-notice"]'),
    ).toBeNull();
  });

  it("keeps every other type's two action slots", () => {
    const { container } = render(<ServiceDetailHeaderSkeleton />);
    expect(actionSlots(container)).toBe(2);
  });
});

// w3/107: a suspended service of any type has no header action and Render's
// notice under its facts; its frame reserves no action slot and the notice in
// the ready one's words, the title for who suspended it included, so it wraps
// where the ready notice does.
describe("ServiceDetailHeaderSkeleton for a suspended service", () => {
  const actionSlots = (container: Element) =>
    container.querySelectorAll(
      ".md\\:max-lg\\:col-start-2 > [data-touch-spacer]",
    ).length;

  it.each([
    ["web_service", false, {}],
    ["private_service", false, { url: null, internalAddress: "app-x1y2:1" }],
    ["background_worker", false, { url: null }],
    ["static_site", false, { plan: null }],
    ["cron_job", true, { url: null, schedule: "* * * * *" }],
  ] as const)(
    "reserves a suspended %s's notice and no action",
    async (type, cron, overrides) => {
      renderHeader(
        svc({ type, ...overrides, suspended: true, crashLoopSuspended: true }),
        { onRun: vi.fn() },
      );
      const ready = await screen.findByRole("status");

      const { container } = render(
        <ServiceDetailHeaderSkeleton
          source={{
            repository: true,
            registryCredential: false,
            cron,
            suspendedNotice: "services.suspendedNoticeCrashLoop",
          }}
        />,
      );
      expect(actionSlots(container)).toBe(0);
      const notice = container.querySelector(
        '[data-skeleton-item="suspended-notice"]',
      )!;
      expect(notice).toHaveTextContent(ready.textContent!);
      expect(notice).toHaveClass(...ready.classList);
      // The resume link's slot grows with the touch rule, as the ready one does.
      expect(notice.lastElementChild).toHaveAttribute("data-touch-spacer");
    },
  );
});

// w3/m555 production walk: at 390 a service's ready name row ("Service", its
// status, "Latest deploy <status>", its plan) wraps onto a second line, and a
// frame that drew no latest deploy fit on one, 30px short of the ready band.
// The frame reserves the ready row's labels in their own words.
describe("ServiceDetailHeaderSkeleton name row", () => {
  it("reserves the ready row's labels, the latest deploy's included", async () => {
    renderHeader(svc(), {
      latestDeploy: { id: "dep-1", status: "live" } as LatestDeploySummary,
    });
    const ready = await screen.findByRole("link", {
      name: "Latest deploy: Live",
    });
    expect(ready).toHaveTextContent("Latest deploy");

    const { container } = render(<ServiceDetailHeaderSkeleton />);
    const slot = container.querySelector(
      '[data-skeleton-item="latest-deploy"]',
    )!;
    expect(slot).toHaveTextContent("Latest deploy");
    // Laid out as the ready link is: the label, then a badge-sized bar.
    expect(slot).toHaveClass("flex", "items-center", "gap-1.5");
    expect(slot.parentElement).toHaveTextContent(/^Service\s*Latest deploy$/);
  });

  it("lays the cached name out at the ready heading's size", async () => {
    renderHeader(svc({ name: "nightly-report" }));
    const heading = await screen.findByRole("heading", {
      name: "nightly-report",
    });
    const { container } = render(
      <ServiceDetailHeaderSkeleton name="nightly-report" />,
    );
    const reserved = within(container as HTMLElement).getByText(
      "nightly-report",
    );
    expect(reserved).toHaveClass("invisible");
    const slot = reserved.parentElement!;
    for (const size of ["text-xl", "font-semibold", "truncate"]) {
      expect(heading).toHaveClass(size);
      expect(slot).toHaveClass(size);
    }
  });
});

// w3/m555 production walk: at 390 "Runs <command> at 09:00 AM UTC" broke "AM
// UTC" onto a second line beside Details while the command still had room
// to truncate, 20px taller than its one-line frame. The command now gives up
// its width first; the words never shrink, and take a line of their own only
// when even four characters of the command leave them no room.
describe("cron facts line", () => {
  it("truncates the command before the schedule's words wrap", async () => {
    renderHeader(
      svc({
        type: "cron_job",
        url: null,
        schedule: "0 9 * * *",
        command: "node -e \"console.log('m555 run ok')\"",
      }),
    );
    const words = await screen.findByText("at 09:00 AM UTC");
    const line = words.closest('[data-header-item="cron-runs"]')!;
    expect(line).toHaveClass("flex", "flex-wrap");
    expect(words).toHaveClass("shrink-0", "max-w-full");
    const command = within(line as HTMLElement).getByText(/console\.log/);
    expect(command).toHaveClass("basis-0", "grow", "max-w-max", "truncate");
  });
});
