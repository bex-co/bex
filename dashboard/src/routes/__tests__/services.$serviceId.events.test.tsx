import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  type AnyRouter,
} from "@tanstack/react-router";
import { ServiceEventsPage } from "../services.$serviceId.events";
import { formatDateTime } from "@/common/lib/format";
import i18n from "@/i18n/init";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import { DeployBuildAvailabilityDocument } from "@/graphql/definitions";

// The Events tab reads server(id) for the cron-runs section; a null service
// (the common non-cron case) keeps that section out of the way.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

// w4/561: a move event names its placements from these lists.
const placementState: {
  projects: { id: string; name: string }[];
  environments: Record<string, { id: string; name: string }[]>;
} = { projects: [], environments: {} };
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({ projects: placementState.projects }),
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: (projectId: string | null) => ({
    environments: (projectId && placementState.environments[projectId]) || [],
  }),
}));

const instanceTypeNames: Record<string, string> = {
  starter: "Starter",
  standard: "Standard",
};
vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    byID: (id: string | null | undefined) =>
      id && instanceTypeNames[id] ? { name: instanceTypeNames[id] } : undefined,
  }),
}));

vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));

const mockUseQuery = vi.fn();
const rollbackService = vi.fn();
const cancelDeploy = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: vi.fn(),
  useApolloClient: () => ({
    query: vi.fn().mockResolvedValue({
      data: {
        deployActions: [
          {
            action: "cancel_deploy",
            outcome: "allowed",
            reason: null,
            precondition: null,
          },
          {
            action: "rollback",
            outcome: "allowed",
            reason: null,
            precondition: null,
          },
        ],
      },
    }),
  }),
}));

vi.mock("@/features/capabilities/hooks/use-resource-actions", async () => {
  const { mockAllowedResourceActions } =
    await import("@/test/mocks/resource-actions");
  return mockAllowedResourceActions("app");
});

vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext();
});

function deployEvent(over: Record<string, unknown> = {}) {
  return {
    id: "evt-live-001",
    type: "deploy_ended",
    timestamp: "2026-07-14T14:30:00Z",
    cursor: "cursor-live-001",
    details: {
      deployId: "dep-live-001",
      deployStatus: "succeeded",
      preDeployStatus: "",
      actor: "dev@localhost",
      triggeredByUser: "dev@localhost",
      trigger: { manual: true },
    },
    ...over,
  };
}

// A deploy the events feed offers Rollback on: a previous deploy that a newer
// live deploy has since deactivated — never the current live one (w4/051).
function deactivatedDeployEvent() {
  return deployEvent({
    details: {
      deployId: "dep-live-001",
      deployStatus: "deactivated",
      preDeployStatus: "",
      actor: "dev@localhost",
      triggeredByUser: "dev@localhost",
      trigger: { manual: true },
    },
  });
}

// row is one feed event of type at the given hour of a fixed day.
function row(
  id: string,
  type: string,
  hour: number,
  details: Record<string, unknown>,
) {
  return {
    id,
    type,
    timestamp: `2026-08-28T${String(hour).padStart(2, "0")}:00:00Z`,
    cursor: `c-${id}`,
    details,
  };
}

// The route's component reads serviceId via Route.useParams(), so it needs a
// real router context — rebuild it rooted at the events path (mirrors
// services.$serviceId.settings.test.tsx's pattern). The deploys/$deployId
// destination route isn't mounted: TanStack's <Link> renders a real <a href>
// synchronously, so asserting on href proves the row points at the right
// page without the destination page needing to exist in this test; for the
// rollback case we assert the router's own post-navigate location instead
// (useNavigate is real here, not mocked).
function renderEvents(serviceId = "app") {
  const rootRoute = createRootRoute();
  const eventsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/events",
    // The page now takes serviceId as a prop (shared by the /services and
    // /static route trees, w5/m57); pass the route's own param.
    component: () => <ServiceEventsPage serviceId={serviceId} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([eventsRoute]),
    history: createMemoryHistory({
      initialEntries: [`/services/${serviceId}/events`],
    }),
    context: { client: {} as never, session: null },
  });
  const result = render(<RouterProvider router={router} />);
  return { router: router as AnyRouter, ...result };
}

beforeEach(async () => {
  mockUseQuery.mockReset();
  rollbackService.mockReset();
  cancelDeploy.mockReset();
  serverState.service = null;
  const { useMutation } = await import("@apollo/client/react");
  vi.mocked(useMutation).mockImplementation((doc: unknown) => {
    // Match by the mutation's own operation name (definitions[0].name.value)
    // rather than call order — robust to the two useMutation calls in
    // ServiceEventsPage being reordered.
    type OpDoc = { definitions?: Array<{ name?: { value?: string } }> };
    const opName = (doc as OpDoc)?.definitions?.[0]?.name?.value;
    if (opName === "RollbackService") {
      return [rollbackService, { loading: false }] as never;
    }
    return [cancelDeploy, { loading: false }] as never;
  });
});

describe("ServiceEventsPage — deploy rows link to the deploy page (w9/m1/t004)", () => {
  it("links each stored deploy row to its exact deploy page", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    const link = await screen.findByRole("link");
    expect(link).toHaveAttribute("href", "/services/app/deploys/dep-live-001");
  });

  it("presents deploy_started as in progress even though its status is empty", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent({
            type: "deploy_started",
            details: {
              deployId: "dep-live-001",
              deployStatus: "",
              preDeployStatus: "",
              trigger: { firstBuild: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("In Progress")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^Cancel deploy / }),
    ).toBeInTheDocument();
    // The row's <time> hover carries the exact stamp via the shared formatter
    // ("July 14, 2026 at …") — computed through the helper so this holds in
    // any runner timezone.
    expect(
      screen.getByTitle(formatDateTime("2026-07-14T14:30:00Z")!),
    ).toBeInTheDocument();
  });

  it("does not offer cancel on a started event whose deploy already finished", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent(),
          deployEvent({
            id: "evt-started-001",
            type: "deploy_started",
            details: {
              deployId: "dep-live-001",
              deployStatus: "",
              preDeployStatus: "",
              trigger: { firstBuild: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Deploy started")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Cancel deploy / }),
    ).not.toBeInTheDocument();
  });

  it("names non-deploy activity instead of presenting it as an unknown deploy", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent({
            id: "evt-scale-001",
            type: "instance_count_changed",
            details: {
              deployId: "",
              deployStatus: "",
              preDeployStatus: "",
              trigger: null,
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText("Instance count changed"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Unknown")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  // w8/m158: a running service's process that crashed or was killed for
  // memory is an Instance Failed event with that cause and the instance, not
  // a failed health check and not a cron run's wording.
  it("words a service's crash and memory kill as the instance's", async () => {
    const failed = (id: string, details: Record<string, unknown>) => ({
      ...deployEvent({ details: {} }),
      id,
      type: "server_failed",
      details: { instanceId: "web-7d9f5c-abcde", ...details },
    });
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          failed("evt-crash", { reasonCode: "non_zero_exit", exitCode: 3 }),
          failed("evt-oom", { reasonCode: "oom_killed", memoryLimit: "512Mi" }),
          failed("evt-exit0", { reasonCode: "early_exit" }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });
    renderEvents("app");

    expect(
      await screen.findByText(
        /The instance's process exited with status 3 and was restarted\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /The instance ran out of memory \(512 MB limit\) and was restarted\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/The instance's process exited and was restarted\./),
    ).toBeInTheDocument();
    expect(screen.queryByText(/The run exited/)).not.toBeInTheDocument();
    expect(screen.getAllByText(/Instance web-7d9f5c-abcde/)).toHaveLength(3);
  });

  // w8/m156: the registry keeps a bounded history of builds. A previous
  // deploy whose build is gone cannot be rolled back to, so its row offers no
  // Rollback; one whose build is stored keeps it.
  it("offers no Rollback on a previous deploy whose build is no longer stored", async () => {
    const events = {
      data: { serviceEvents: [deactivatedDeployEvent()] },
      loading: false,
      refetch: vi.fn(),
    };
    const availability = (buildAvailable: boolean) => (doc: unknown) =>
      doc === DeployBuildAvailabilityDocument
        ? {
            data: { deploys: [{ id: "dep-live-001", buildAvailable }] },
            loading: false,
          }
        : events;

    mockUseQuery.mockImplementation(availability(false));
    const gone = renderEvents("app");
    expect(await screen.findByText(/dep-live-001/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Rollback to / }),
    ).not.toBeInTheDocument();
    gone.unmount();

    mockUseQuery.mockImplementation(availability(true));
    renderEvents("app");
    expect(
      await screen.findByRole("button", { name: /^Rollback to / }),
    ).toBeInTheDocument();
    mockUseQuery.mockReset();
  });

  it("navigates to the rollback deploy's own page once the mutation resolves its id", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deactivatedDeployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });
    rollbackService.mockResolvedValue({
      data: { rollbackService: { id: "dep-rollback-1", status: "live" } },
    });

    const user = userEvent.setup();
    const { router } = renderEvents("app");

    await user.click(
      await screen.findByRole("button", { name: /^Rollback to / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(rollbackService).toHaveBeenCalledWith({
      variables: { serviceId: "app", deployId: "dep-live-001" },
    });
    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/app/deploys/dep-rollback-1",
      );
    });
  });

  it("does not navigate when the rollback mutation rejects", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deactivatedDeployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });
    rollbackService.mockRejectedValue(new Error("conflict"));

    const user = userEvent.setup();
    const { router } = renderEvents("app");

    await user.click(
      await screen.findByRole("button", { name: /^Rollback to / }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(rollbackService).toHaveBeenCalled();
    expect(router.state.location.pathname).toBe("/services/app/events");
  });
});

// w4/568: an instance-count, autoscaling or plan change says what changed;
// a legacy row without the values keeps its bare label.
describe("ServiceEventsPage — configuration change values (w4/568)", () => {
  it("shows each change's before and after", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          {
            id: "evt-count",
            type: "instance_count_changed",
            timestamp: "2026-08-28T09:00:00Z",
            cursor: "c-count",
            details: { instanceCountFrom: 1, instanceCountTo: 2 },
          },
          {
            id: "evt-auto-on",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T08:00:00Z",
            cursor: "c-auto-on",
            details: { autoscalingMinTo: 1, autoscalingMaxTo: 2 },
          },
          {
            id: "evt-auto-off",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T07:00:00Z",
            cursor: "c-auto-off",
            details: { autoscalingMinFrom: 1, autoscalingMaxFrom: 2 },
          },
          {
            id: "evt-plan",
            type: "plan_changed",
            timestamp: "2026-08-28T06:00:00Z",
            cursor: "c-plan",
            details: { planFrom: "starter", planTo: "standard" },
          },
          {
            id: "evt-plan-first",
            type: "plan_changed",
            timestamp: "2026-08-28T05:00:00Z",
            cursor: "c-plan-first",
            details: { planFrom: "", planTo: "starter" },
          },
          {
            id: "evt-auto-legacy",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T04:00:00Z",
            cursor: "c-auto-legacy",
            details: {},
          },
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Instances 1 → 2")).toBeInTheDocument();
    expect(
      screen.getByText("Autoscaling Off → 1–2 instances"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Autoscaling 1–2 instances → Off"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Instance type Starter → Standard"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Instance type none → Starter"),
    ).toBeInTheDocument();
    // The legacy row adds no third "Autoscaling …" line, Off → Off included.
    expect(screen.getAllByText(/^Autoscaling .+ → /)).toHaveLength(2);
  });

  // w4/m335: a change to a target alone no longer reads "1–3 → 1–3".
  it("names each side's CPU and memory targets, in en and zh", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          {
            id: "evt-retarget",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T08:00:00Z",
            cursor: "c-retarget",
            details: {
              autoscalingMinFrom: 1,
              autoscalingMaxFrom: 3,
              autoscalingCPUFrom: 70,
              autoscalingMinTo: 1,
              autoscalingMaxTo: 3,
              autoscalingCPUTo: 60,
              autoscalingMemoryTo: 80,
            },
          },
          {
            id: "evt-memory-off",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T07:00:00Z",
            cursor: "c-memory-off",
            details: {
              autoscalingMinFrom: 2,
              autoscalingMaxFrom: 4,
              autoscalingMemoryFrom: 75,
            },
          },
          {
            id: "evt-before-targets",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T06:00:00Z",
            cursor: "c-before-targets",
            details: { autoscalingMinTo: 1, autoscalingMaxTo: 2 },
          },
          {
            // w4/597: enabling the settings a disable kept.
            id: "evt-kept",
            type: "autoscaling_config_changed",
            timestamp: "2026-08-28T05:00:00Z",
            cursor: "c-kept",
            details: {
              autoscalingMinFrom: 2,
              autoscalingMaxFrom: 3,
              autoscalingCPUFrom: 50,
              autoscalingEnabledFrom: false,
              autoscalingMinTo: 2,
              autoscalingMaxTo: 3,
              autoscalingCPUTo: 50,
            },
          },
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText(
        "Autoscaling 1–3 instances, CPU 70% → 1–3 instances, CPU 60%, memory 80%",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Autoscaling 2–4 instances, memory 75% → Off"),
    ).toBeInTheDocument();
    // A row recorded before the targets were shows the range alone.
    expect(
      screen.getByText("Autoscaling Off → 1–2 instances"),
    ).toBeInTheDocument();
    // Settings a disable kept read as Off until they run again.
    expect(
      screen.getByText("Autoscaling Off → 2–3 instances, CPU 50%"),
    ).toBeInTheDocument();

    await act(() => i18n.changeLanguage("zh"));
    try {
      expect(
        await screen.findByText(
          "自动伸缩 1–3 个实例，CPU 70% → 1–3 个实例，CPU 60%，内存 80%",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByText("自动伸缩 2–4 个实例，内存 75% → 关闭"),
      ).toBeInTheDocument();
    } finally {
      await act(() => i18n.changeLanguage("en"));
    }
  });
});

// w4/563: an env-group link or unlink names its group; a row recorded
// before groups were named keeps the bare type label.
describe("ServiceEventsPage — env-group link titles (w4/563)", () => {
  it("names the group a link added or an unlink removed", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          {
            id: "evt-link-1",
            type: "env_group_linked",
            timestamp: "2026-08-28T09:00:00Z",
            cursor: "cursor-link-1",
            details: { envGroupId: "evg-shared", envGroupName: "shared" },
          },
          {
            id: "evt-unlink-1",
            type: "env_group_unlinked",
            timestamp: "2026-08-28T08:00:00Z",
            cursor: "cursor-unlink-1",
            details: { envGroupId: "evg-old", envGroupName: "legacy" },
          },
          {
            id: "evt-link-0",
            type: "env_group_linked",
            timestamp: "2026-08-28T07:00:00Z",
            cursor: "cursor-link-0",
            details: {},
          },
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText("Environment group shared linked"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Environment group legacy unlinked"),
    ).toBeInTheDocument();
    expect(screen.getByText("Environment group linked")).toBeInTheDocument();
  });
});

// w4/m337: a rename names both names, and a custom-domain row its hostname;
// rows recorded before either keep their bare titles.
describe("ServiceEventsPage — rename and custom-domain details (w4/m337)", () => {
  it("names the service's names and the domain, in en and zh", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          row("evt-rename", "display_name_changed", 9, {
            displayNameFrom: "api",
            displayNameTo: "Checkout API",
          }),
          row("evt-added", "custom_domain_added", 8, {
            domain: "app.example.com",
          }),
          row("evt-verified", "custom_domain_verified", 7, {
            domain: "app.example.com",
          }),
          row("evt-removed", "custom_domain_removed", 6, {
            domain: "app.example.com",
          }),
          row("evt-rename-legacy", "display_name_changed", 5, {}),
          row("evt-added-legacy", "custom_domain_added", 4, {}),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText("Name api → Checkout API"),
    ).toBeInTheDocument();
    for (const title of [
      "Custom domain app.example.com added",
      "Custom domain app.example.com verified",
      "Custom domain app.example.com removed",
    ]) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
    expect(screen.getAllByText("Display name changed")).toHaveLength(2);
    expect(screen.getByText("Custom domain added")).toBeInTheDocument();
    expect(screen.getAllByText(/^Name /)).toHaveLength(1);

    await act(() => i18n.changeLanguage("zh"));
    try {
      expect(
        await screen.findByText("名称 api → Checkout API"),
      ).toBeInTheDocument();
      for (const title of [
        "已添加自定义域名 app.example.com",
        "自定义域名 app.example.com 已验证",
        "已移除自定义域名 app.example.com",
      ]) {
        expect(screen.getByText(title)).toBeInTheDocument();
      }
    } finally {
      await act(() => i18n.changeLanguage("en"));
    }
  });
});

describe("ServiceEventsPage — edge cache and pipeline minutes (w9/m345)", () => {
  it("names each edge-cache change and the refused build, with who did it, in en and zh", async () => {
    const cache = {
      edgeCacheTrigger: { user: { id: "usr-cache", email: "" } },
    };
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          row("evt-on", "edge_cache_enabled", 9, cache),
          row("evt-purge", "edge_cache_purged", 8, cache),
          row("evt-off", "edge_cache_disabled", 7, cache),
          row("evt-minutes", "pipeline_minutes_exhausted", 6, {
            deployId: "dep-refused",
            trigger: { manual: true, user: { id: "usr-dev", email: "" } },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    for (const title of [
      "Edge cache enabled",
      "Edge cache purged",
      "Edge cache disabled",
      "Pipeline minutes exhausted",
    ]) {
      expect(await screen.findByText(title)).toBeInTheDocument();
    }
    expect(screen.getAllByText("by usr-cache")).toHaveLength(3);
    expect(screen.getByText("by usr-dev")).toBeInTheDocument();
    // The refused build links to its deploy, as the deploy rows do.
    const refused = screen
      .getByText("Pipeline minutes exhausted")
      .closest("a") as HTMLAnchorElement;
    expect(refused.getAttribute("href")).toBe(
      "/services/app/deploys/dep-refused",
    );

    await act(() => i18n.changeLanguage("zh"));
    try {
      for (const title of [
        "边缘缓存已启用",
        "边缘缓存已清除",
        "边缘缓存已停用",
        "流水线分钟数已用尽",
      ]) {
        expect(await screen.findByText(title)).toBeInTheDocument();
      }
    } finally {
      await act(() => i18n.changeLanguage("en"));
    }
  });
});

describe("ServiceEventsPage — service_moved placement details (w6/m134)", () => {
  afterEach(() => {
    placementState.projects = [];
    placementState.environments = {};
  });

  function movedEvent(details: Record<string, unknown>) {
    return {
      id: "evt-move-1",
      type: "service_moved",
      timestamp: "2026-08-28T09:00:00Z",
      cursor: "cursor-move-1",
      details,
    };
  }

  it("renders both from→to dimensions of a full move", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          movedEvent({
            projectFrom: "prj-old",
            projectTo: "prj-new",
            environmentFrom: "env-old",
            environmentTo: "env-new",
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Service moved")).toBeInTheDocument();
    expect(screen.getByText("Project prj-old → prj-new")).toBeInTheDocument();
    expect(
      screen.getByText("Environment env-old → env-new"),
    ).toBeInTheDocument();
  });

  // w4/561: the detail names each placement as the project page does; a side
  // that no longer exists reads as its id, so the row never goes blank.
  it("names the placements, and keeps the id of one that is gone", async () => {
    placementState.projects = [
      { id: "prj-old", name: "storefront" },
      { id: "prj-new", name: "checkout" },
    ];
    placementState.environments = {
      "prj-old": [{ id: "env-old", name: "qa-alpha" }],
      "prj-new": [{ id: "env-new", name: "qa-beta" }],
    };
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          movedEvent({
            projectFrom: "prj-old",
            projectTo: "prj-new",
            environmentFrom: "env-old",
            environmentTo: "env-new",
          }),
          {
            ...movedEvent({
              projectFrom: "prj-new",
              projectTo: "prj-new",
              environmentFrom: "env-new",
              environmentTo: "env-deleted",
            }),
            id: "evt-move-2",
            cursor: "cursor-move-2",
          },
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText("Project storefront → checkout"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Environment qa-alpha → qa-beta"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Environment qa-beta → env-deleted"),
    ).toBeInTheDocument();
  });

  it("reads an absent placement side as none, and skips an unchanged dimension", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          // An environment-only move: the project pair is identical and must
          // not render a "changed" line for it.
          movedEvent({
            projectFrom: "prj-1",
            projectTo: "prj-1",
            environmentFrom: null,
            environmentTo: "env-new",
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Service moved")).toBeInTheDocument();
    expect(screen.getByText("Environment none → env-new")).toBeInTheDocument();
    expect(screen.queryByText(/^Project /)).not.toBeInTheDocument();
  });
});

describe("ServiceEventsPage — the feed is fail-open (w6/m122)", () => {
  // The 2026-08-27 live capture: custom_domain_verified sat between two
  // custom_domain_added rows that rendered, and it did not — the tab filtered
  // `events ∩ catalog`, and the filter's own option list came from that same
  // catalog, so no user action could reveal it.
  function configEvent(type: string, id: string, timestamp: string) {
    return {
      id,
      type,
      timestamp,
      cursor: `cursor-${id}`,
      details: { actor: "dev@localhost" },
    };
  }

  it("renders the event type that used to be filtered away", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          configEvent("custom_domain_added", "evt-1", "2026-08-27T11:54:08Z"),
          configEvent(
            "custom_domain_verified",
            "evt-2",
            "2026-08-27T11:54:07Z",
          ),
          configEvent("custom_domain_added", "evt-3", "2026-08-27T11:52:55Z"),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText("Custom domain verified"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Custom domain added")).toHaveLength(2);
  });

  it("renders a type absent from the catalog under the generic label", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          configEvent("a_future_backend_type", "evt-9", "2026-08-27T12:00:00Z"),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    // The bullet that separates a real fix from one that merely clears today's
    // backlog: an uncatalogued type still reaches the reader.
    expect(
      await screen.findByText("Service settings changed"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("No events match this filter"),
    ).not.toBeInTheDocument();
  });

  it("filters one build setting by its own name while the feed title stays generic (w4/223)", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          configEvent(
            "max_shutdown_delay_changed",
            "evt-1",
            "2026-08-27T11:54:08Z",
          ),
          configEvent(
            "root_directory_changed",
            "evt-2",
            "2026-08-27T11:54:07Z",
          ),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");
    expect(
      await screen.findAllByText("Build and deploy settings changed"),
    ).toHaveLength(2);

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Filter events" }));
    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    await user.click(
      screen.getByRole("checkbox", { name: "Max shutdown delay changed" }),
    );
    await user.keyboard("{Escape}");

    // One row left, still under the feed's shared title.
    expect(
      screen.getAllByText("Build and deploy settings changed"),
    ).toHaveLength(1);
    expect(
      screen.queryByText("No events match this filter"),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Filter events/ }));
    await user.click(
      screen.getByRole("checkbox", { name: "Max shutdown delay changed" }),
    );
    await user.click(
      screen.getByRole("checkbox", { name: "Root directory changed" }),
    );
    await user.keyboard("{Escape}");
    expect(
      screen.getAllByText("Build and deploy settings changed"),
    ).toHaveLength(1);
  });

  it("counts the window's own feed, not the post-filter subset", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          configEvent("custom_domain_added", "evt-1", "2026-08-27T11:54:08Z"),
          configEvent(
            "custom_domain_verified",
            "evt-2",
            "2026-08-27T11:54:07Z",
          ),
          configEvent("a_future_backend_type", "evt-3", "2026-08-27T11:52:55Z"),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("3")).toBeInTheDocument();

    // Hiding a group narrows the list but not the badge: the badge answers
    // "how much happened", the list answers "what am I looking at".
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Filter events" }));
    await user.click(screen.getByRole("checkbox", { name: "Configuration" }));

    // Close the popover first: its own option labels would otherwise satisfy a
    // document-wide text query and hide whether the FEED changed.
    await user.keyboard("{Escape}");

    expect(screen.getByText("3")).toBeInTheDocument();
    expect(
      screen.queryByText("Custom domain verified"),
    ).not.toBeInTheDocument();
    // The uncatalogued row survives a catalog-group deselection — it is not a
    // member of any catalog group, so nothing in the catalog can hide it.
    expect(screen.getByText("Service settings changed")).toBeInTheDocument();
  });
});

describe("ServiceEventsPage — failed deploys badge their own status + reason (w1/m138)", () => {
  function failedDeployEvent(over: Record<string, unknown> = {}) {
    return deployEvent({
      id: "evt-build-fail-001",
      details: {
        deployId: "dep-build-fail-001",
        deployStatus: "failed",
        fullDeployStatus: "build_failed",
        failureReason: "image pull failed: not found",
        preDeployStatus: "",
        trigger: { manual: true },
      },
      ...over,
    });
  }

  it("badges a build failure as Build Failed, not Failed, and shows its reason", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [failedDeployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Build Failed")).toBeInTheDocument();
    expect(
      screen.getByText("image pull failed: not found"),
    ).toBeInTheDocument();
    // The lossy update_failed mapping must not fire when the backend named
    // the kind: no bare "Failed" badge.
    expect(screen.queryByText("Failed")).not.toBeInTheDocument();
  });

  it("badges a pre-deploy failure as Pre-Deploy Failed with its reason", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          failedDeployEvent({
            id: "evt-predeploy-fail-001",
            details: {
              deployId: "dep-predeploy-fail-001",
              deployStatus: "failed",
              fullDeployStatus: "pre_deploy_failed",
              failureReason: "migration exited 1",
              preDeployStatus: "failed",
              trigger: { manual: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Pre-Deploy Failed")).toBeInTheDocument();
    expect(screen.getByText("migration exited 1")).toBeInTheDocument();
  });

  it("keeps the old mapping for legacy failed events without the new field", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent({
            id: "evt-legacy-fail-001",
            details: {
              deployId: "dep-legacy-fail-001",
              deployStatus: "failed",
              preDeployStatus: "",
              trigger: { manual: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Failed")).toBeInTheDocument();
  });

  it("shows no reason on a succeeded deploy", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });

    const { container } = renderEvents("app");

    expect(await screen.findByText("Live")).toBeInTheDocument();
    // The reason line is the only text-destructive <p> the row renders.
    expect(container.querySelector("p.text-destructive")).toBeNull();
  });
});

// The events feed and the deploys list describe one deploy; naming its trigger
// two different ways is a contradiction a reader has to resolve, and both of the
// old events labels were also factually wrong (w4/100, w4/104 — both observed
// live on 2026-09-19 against the same deploys the Deploys tab labelled
// correctly).
describe("ServiceEventsPage — a deploy's trigger is named the way the Deploys tab names it", () => {
  const cases: Array<{
    name: string;
    trigger: Record<string, boolean>;
    want: string;
    notWant?: string;
  }> = [
    {
      // A Settings edit — a start command, a schedule — rides Render's
      // envUpdated flag because that is the nearest flag its vocabulary has.
      // "Environment updated" then told the reader an environment value had
      // changed when none had.
      name: "a config change is a Config Change, not an environment update",
      trigger: { envUpdated: true },
      want: "Config Change",
      notWant: "Environment updated",
    },
    {
      // A CI system POSTing a secret URL. Nobody clicked anything.
      name: "a deploy hook is a Deploy Hook, not a manual deploy",
      trigger: { deployHook: true },
      want: "Deploy Hook",
      notWant: "Manual deploy",
    },
    {
      // The regression target: an API/dashboard deploy really is manual.
      name: "an API deploy is still a Manual deploy",
      trigger: { manual: true },
      want: "Manual deploy",
    },
    {
      // w4/106: the flag is called firstBuild because that is Render's field
      // name, but an `image`-runtime service has no build stage at all, so the
      // old "First build" label asserted a step that never ran — and the
      // Deploys tab called the same deploy "First Deploy".
      name: "a first deploy is a First Deploy, not a First build",
      trigger: { firstBuild: true },
      want: "First Deploy",
      notWant: "First build",
    },
  ];

  for (const tc of cases) {
    it(tc.name, async () => {
      mockUseQuery.mockReturnValue({
        data: {
          serviceEvents: [
            deployEvent({
              type: "deploy_started",
              details: {
                deployId: "dep-live-001",
                deployStatus: "",
                preDeployStatus: "",
                trigger: tc.trigger,
              },
            }),
          ],
        },
        loading: false,
        refetch: vi.fn(),
      });

      renderEvents("app");

      expect(await screen.findByText(tc.want)).toBeInTheDocument();
      if (tc.notWant) {
        expect(screen.queryByText(tc.notWant)).not.toBeInTheDocument();
      }
    });
  }

  // A server that has not shipped the new flag yet still sets `manual` for a
  // hook deploy. Checking deployHook first means the correct label wins as soon
  // as either side is deployed, in whichever order that happens.
  it("prefers the hook label when a server sends both flags", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent({
            type: "deploy_started",
            details: {
              deployId: "dep-live-001",
              deployStatus: "",
              preDeployStatus: "",
              trigger: { manual: true, deployHook: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Deploy Hook")).toBeInTheDocument();
    expect(screen.queryByText("Manual deploy")).not.toBeInTheDocument();
  });
});
