import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceComputePage } from "@/features/services/components/service-compute-page";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  billingCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// The Compute page (w6/m483) composes useServer (type gating, plan, replicas),
// useInstanceTypes + useUpdatePlan (the Compute section), useAutoscaling
// (card + mutual exclusion), useScaleService (manual save), and useMetrics
// (recent metrics) — mock the data layer, mirroring the settings page test.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

const autoscalingState = {
  enabled: false,
  loading: false,
  saving: false,
  minInstances: 1,
  maxInstances: 3,
  targetCPUPercent: null as number | null,
  targetMemoryPercent: null as number | null,
  metricsUnavailable: false,
  save: vi.fn(async () => true),
  disable: vi.fn(async () => true),
};
const autoscalingCalls: Array<{ skip?: boolean } | undefined> = [];
vi.mock("@/features/services/hooks/use-autoscaling", () => ({
  useAutoscaling: (_id: string, options?: { skip?: boolean }) => {
    autoscalingCalls.push(options);
    return autoscalingState;
  },
}));

const STARTER: InstanceTypeView = {
  id: "starter",
  planId: "0.5c-512mb",
  name: "Starter",
  cpu: "500m",
  memory: "512Mi",
  monthlyUsd: "5.25",
  available: true,
  unavailableReason: "",
};
const STANDARD: InstanceTypeView = {
  id: "standard",
  planId: "1c-2g",
  name: "Standard",
  cpu: "1",
  memory: "2Gi",
  monthlyUsd: "17.50",
  available: true,
  unavailableReason: "",
};
vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    instanceTypes: [STARTER, STANDARD],
    loading: false,
    error: undefined,
    byID: (id: string | null | undefined) =>
      [STARTER, STANDARD].find((it) => it.id === id),
  }),
}));

// The list cache's answer while the server read is still in flight.
const cachedType: { value: string | null } = { value: null };
vi.mock("@/features/services/hooks/use-cached-service-type", () => ({
  useCachedServiceType: () => cachedType.value,
}));

const updatePlan = vi.fn(
  async (): Promise<{ deployId: string | null } | null> => ({ deployId: null }),
);
vi.mock("@/features/services/hooks/use-update-plan", () => ({
  useUpdatePlan: () => ({ updatePlan, busy: false }),
}));

const scaleService = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-scale-service", () => ({
  useScaleService: () => ({ scaleService, busy: false }),
}));

// Recent Metrics reads five metrics; the mock serves per-metric fixtures.
const EMPTY_METRIC: UseMetricsResult = {
  series: [],
  loading: false,
  unavailable: false,
  storeUnavailable: false,
  throttled: false,
  degradedSources: [],
  error: undefined,
};
// Keyed by metric id, with a "%" suffix for the percentage read.
const metricsState = new Map<string, UseMetricsResult>();
const metricReads = vi.fn();
vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: (
    _resource: string,
    metric: string,
    opts?: { percentage?: boolean },
  ) => {
    metricReads(metric);
    return (
      metricsState.get(opts?.percentage ? `${metric}%` : metric) ?? EMPTY_METRIC
    );
  },
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 2,
    revision: "r1",
    plan: null,
    idleTTLSeconds: 0,
    maintenanceMode: { enabled: false, uri: "" },
    outboundIps: null,
    schedule: null,
    command: null,
    runs: [],
    healthCheckPath: null,
    maxShutdownDelaySeconds: 30,
    registryCredentialId: null,
    slug: null,
    internalAddress: null,
    sshAddress: null,
    repo: null,
    branch: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    dockerContext: null,
    buildFilter: null,
    autoDeploy: null,
    autoDeployTrigger: null,
    notifyOnFail: null,
    notificationsToSend: null,
    preDeployCommand: null,
    renderSubdomainPolicy: null,
    publishPath: null,
    routes: [],
    headers: [],
    ipAllowList: null,
    ipAllowListEntries: null,
    ...overrides,
  };
}

// ScalingRecentMetrics renders a Link, so the page needs a real router context.
function renderCompute(serviceId = "app") {
  const rootRoute = createRootRoute();
  const computeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/compute",
    component: () => <ServiceComputePage serviceId={serviceId} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([computeRoute]),
    history: createMemoryHistory({
      initialEntries: [`/services/${serviceId}/compute`],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = svc();
  serverState.loading = false;
  cachedType.value = null;
  autoscalingState.enabled = false;
  autoscalingState.loading = false;
  autoscalingState.minInstances = 1;
  autoscalingState.maxInstances = 3;
  autoscalingState.targetCPUPercent = null;
  autoscalingState.targetMemoryPercent = null;
  autoscalingState.save.mockClear();
  autoscalingState.disable.mockClear();
  scaleService.mockClear();
  scaleService.mockResolvedValue(true);
  updatePlan.mockClear();
  updatePlan.mockResolvedValue({ deployId: null });
  metricsState.clear();
  metricReads.mockClear();
});

// w9/m621: Recent Metrics are service metrics, which Render's Billing role
// does not view; the plan and scaling sections stay. A Viewer sees them all.
describe("ServiceComputePage Recent Metrics per role (w9/m621)", () => {
  const sections = () =>
    screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent);

  it("leaves Recent Metrics out for a Billing member, with no metric read", async () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    await screen.findByText("Manual Scaling");
    expect(sections()).toEqual(["Compute", "Autoscaling", "Manual Scaling"]);
    expect(metricReads).not.toHaveBeenCalled();
  });

  it("shows Recent Metrics to a Viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    await screen.findByText("Manual Scaling");
    expect(sections()).toEqual([
      "Compute",
      "Autoscaling",
      "Manual Scaling",
      "Recent Metrics",
    ]);
    expect(metricReads).toHaveBeenCalledWith("cpu");
  });
});

describe("ServiceComputePage scaling sections (w7/m43, w6/m483)", () => {
  it.each(["web_service", "private_service"] as const)(
    "explains and disables autoscaling for a free %s before any mutation",
    async (type) => {
      serverState.service = svc({ type, plan: "free", replicas: 1 });
      renderCompute();

      const toggle = await screen.findByRole("switch", {
        name: "Autoscaling off",
      });
      expect(toggle).toBeDisabled();
      expect(
        screen.getByText(
          "Autoscaling is available on paid plans. Upgrade to enable it.",
        ),
      ).toBeInTheDocument();
      fireEvent.click(toggle);
      expect(
        screen.queryByRole("spinbutton", { name: "Maximum instances" }),
      ).not.toBeInTheDocument();
      expect(
        screen.getByRole("spinbutton", { name: "Instances" }),
      ).toHaveAttribute("max", "1");
      expect(
        screen.getByRole("spinbutton", { name: "Instances" }),
      ).toBeDisabled();
      expect(
        screen.queryByRole("slider", { name: "Instances" }),
      ).not.toBeInTheDocument();
      expect(autoscalingState.save).not.toHaveBeenCalled();
      expect(autoscalingState.disable).not.toHaveBeenCalled();
    },
  );

  it("keeps a free service's fixed instance count visible with a stored singleton autoscaling config", async () => {
    serverState.service = svc({ plan: "free", replicas: 1 });
    autoscalingState.enabled = true;
    autoscalingState.maxInstances = 1;
    autoscalingState.targetCPUPercent = 60;
    renderCompute();

    // w4/365: the stored config reads as saved-but-inactive (not a plain
    // "Autoscaling off" hiding it), and its turn-off is reachable…
    const toggle = await screen.findByRole("switch", {
      name: "Autoscaling on",
    });
    expect(toggle).not.toBeDisabled();
    expect(
      screen.getByText(/saved but inactive on the Free plan/),
    ).toBeInTheDocument();
    // …while the fixed count that actually runs stays visible beside it.
    expect(screen.getByRole("spinbutton", { name: "Instances" })).toHaveValue(
      1,
    );
    expect(
      screen.queryByRole("spinbutton", { name: "Maximum instances" }),
    ).not.toBeInTheDocument();
    expect(autoscalingState.save).not.toHaveBeenCalled();

    fireEvent.click(toggle);
    fireEvent.click(await screen.findByRole("button", { name: "Disable" }));
    expect(autoscalingState.disable).toHaveBeenCalledOnce();
  });

  it.each(["web_service", "private_service", "background_worker"] as const)(
    "preserves the paid %s editor and its 1–100 instance range",
    async (type) => {
      serverState.service = svc({ type, plan: "starter" });
      renderCompute();

      fireEvent.click(
        await screen.findByRole("switch", { name: "Autoscaling off" }),
      );
      const maximum = screen.getByRole("spinbutton", {
        name: "Maximum instances",
      });
      expect(maximum).toHaveAttribute("max", "100");
      expect(screen.getByRole("slider", { name: "Minimum" })).toHaveAttribute(
        "aria-valuemin",
        "1",
      );
      expect(screen.getByRole("slider", { name: "Maximum" })).toHaveAttribute(
        "aria-valuemax",
        "100",
      );
      fireEvent.change(maximum, { target: { value: "100" } });
      fireEvent.click(
        screen.getByRole("switch", { name: "Target CPU Utilization" }),
      );
      const card = screen
        .getByText("Autoscaling")
        .closest('[data-slot="card"]') as HTMLElement;
      fireEvent.click(
        within(card).getByRole("button", { name: "Save Changes" }),
      );
      await waitFor(() =>
        expect(autoscalingState.save).toHaveBeenCalledWith({
          minInstances: 1,
          maxInstances: 100,
          targetCPUPercent: 60,
          targetMemoryPercent: null,
        }),
      );
    },
  );

  it("shows Autoscaling + Manual Scaling + Recent Metrics when autoscaling is off", async () => {
    renderCompute();

    expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
    expect(screen.getByText("Manual Scaling")).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Instances" })).toHaveAttribute(
      "aria-valuenow",
      "2",
    );
    expect(screen.getByText("Recent Metrics")).toBeInTheDocument();
    expect(screen.getByText("View all metrics.")).toBeInTheDocument();
  });

  it("hides the Manual Scaling card while autoscaling is on (Render's mutual exclusion)", async () => {
    autoscalingState.enabled = true;
    renderCompute();

    expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
    expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
    // Recent Metrics shows in both states.
    expect(screen.getByText("Recent Metrics")).toBeInTheDocument();
  });

  it("names CPU and memory controls independently and preserves range thumb names", async () => {
    autoscalingState.enabled = true;
    autoscalingState.targetCPUPercent = 60;
    autoscalingState.targetMemoryPercent = 70;
    renderCompute();
    await screen.findByText("Autoscaling");
    expect(
      screen.getByRole("switch", { name: "Autoscaling on" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Target CPU Utilization" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Target Memory Utilization" }),
    ).toBeInTheDocument();
    for (const name of [
      "Target CPU Utilization",
      "Target Memory Utilization",
    ]) {
      expect(screen.getByRole("slider", { name })).toBeInTheDocument();
      expect(screen.getByRole("spinbutton", { name })).toBeInTheDocument();
    }
    expect(screen.getByRole("slider", { name: "Minimum" })).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Maximum" })).toBeInTheDocument();
  });

  it.each([
    [
      "web_service",
      ["compute-plan", "autoscaling", "manual-scaling", "recent-metrics"],
    ],
    ["cron_job", ["compute-plan"]],
    [null, ["compute-plan", "autoscaling", "manual-scaling", "recent-metrics"]],
  ] as const)(
    "holds a %s-shaped frame while the service read is in flight",
    async (type, regions) => {
      serverState.service = null;
      serverState.loading = true;
      cachedType.value = type;
      const { container } = renderCompute();
      const frame = await waitFor(() => {
        const el = container.querySelector(
          '[data-route-skeleton="service-compute"]',
        );
        expect(el).not.toBeNull();
        return el!;
      });
      expect(
        [...frame.querySelectorAll("[data-skeleton-region]")].map((r) =>
          r.getAttribute("data-skeleton-region"),
        ),
      ).toEqual(regions);
      // No control is offered before the service is known.
      expect(screen.queryByRole("button")).toBeNull();
      expect(screen.queryByRole("switch")).toBeNull();
      expect(screen.queryByRole("spinbutton")).toBeNull();
    },
  );

  // A cron job cannot scale, so its page never reads an autoscaling config;
  // the read starts with the service read while the type is unknown.
  it.each([
    ["cron_job", true],
    ["web_service", false],
  ] as const)("skips the autoscaling read for a %s: %s", async (type, skip) => {
    serverState.service = svc({ type, plan: "starter" });
    autoscalingCalls.length = 0;
    renderCompute();
    await screen.findByRole("heading", { name: "Compute" });
    expect(autoscalingCalls.at(-1)).toEqual({ skip });
  });

  it("draws only the Compute section when the service read returned nothing", async () => {
    serverState.service = null;
    renderCompute();
    expect(
      await screen.findByRole("heading", { name: "Compute" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Autoscaling")).not.toBeInTheDocument();
    expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
  });

  // w6/m483: a cron job has a compute plan but no scaling, so its Compute
  // page is the Compute section alone (Render: "multiple instances of a web
  // service, private service, or background worker").
  it.each(["cron_job", "static_site"] as const)(
    "shows a %s only its compute plan, no scaling sections",
    async (type) => {
      serverState.service = svc({ type, plan: "starter" });
      renderCompute();

      expect(
        await screen.findByRole("heading", { name: "Compute" }),
      ).toBeInTheDocument();
      expect(screen.queryByText("Autoscaling")).not.toBeInTheDocument();
      expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
      expect(screen.queryByText("Recent Metrics")).not.toBeInTheDocument();
    },
  );

  it("still offers Autoscaling for a background_worker", async () => {
    serverState.service = svc({ type: "background_worker" });
    renderCompute();
    expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
    expect(screen.getByText("Manual Scaling")).toBeInTheDocument();
  });

  it("saves a drafted manual instance count through scaleService", async () => {
    renderCompute();
    await screen.findByText("Manual Scaling");

    // Save is disabled until the draft differs from the live count (2).
    const save = screen.getByRole("button", { name: "Save Changes" });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByRole("spinbutton", { name: "Instances" }), {
      target: { value: "5" },
    });
    expect(save).toBeEnabled();

    fireEvent.click(save);
    await waitFor(() => expect(scaleService).toHaveBeenCalledWith("app", 5));
  });

  it("preserves a rejected manual count so it can be retried", async () => {
    scaleService.mockResolvedValueOnce(false);
    renderCompute();
    await screen.findByText("Manual Scaling");

    const input = screen.getByRole("spinbutton", { name: "Instances" });
    fireEvent.change(input, { target: { value: "5" } });
    fireEvent.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => expect(scaleService).toHaveBeenCalledWith("app", 5));
    expect(input).toHaveValue(5);
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeEnabled();
  });

  it("clamps the manual instance input to 1–100", async () => {
    renderCompute();
    await screen.findByText("Manual Scaling");

    const input = screen.getByRole("spinbutton", { name: "Instances" });
    fireEvent.change(input, { target: { value: "250" } });
    expect(input).toHaveValue(100);
    fireEvent.change(input, { target: { value: "0" } });
    expect(input).toHaveValue(1);
  });

  it("confirms before disabling a server-enabled autoscaling config", async () => {
    autoscalingState.enabled = true;
    renderCompute();

    fireEvent.click(
      await screen.findByRole("switch", { name: "Autoscaling on" }),
    );
    // Render's disable dialog: the fixed Manual Scaling count takes over.
    expect(await screen.findByText("Disable autoscaling?")).toBeInTheDocument();
    expect(autoscalingState.disable).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(autoscalingState.disable).toHaveBeenCalled());
  });

  it("shows Render's empty state per block when no data was captured", async () => {
    renderCompute();

    const empties = await screen.findAllByText(
      "No data captured in the past 48 hours",
    );
    // Memory, CPU, and Total Instances blocks.
    expect(empties).toHaveLength(3);
    expect(screen.getAllByText("Across all instances")).toHaveLength(2);
  });

  it("charts averaged utilization as % of the limit when data exists", async () => {
    metricsState.set("memory", {
      ...EMPTY_METRIC,
      series: [
        {
          unit: "bytes",
          labels: { instance: "a" },
          points: [{ timestamp: "2026-07-16T00:00:00Z", value: 40 }],
        },
        {
          unit: "bytes",
          labels: { instance: "b" },
          points: [{ timestamp: "2026-07-16T00:00:00Z", value: 80 }],
        },
      ],
    });
    metricsState.set("memory%", {
      ...EMPTY_METRIC,
      series: [
        {
          unit: "percentage",
          labels: { instance: "a" },
          points: [{ timestamp: "2026-07-16T00:00:00Z", value: 40 }],
        },
        {
          unit: "percentage",
          labels: { instance: "b" },
          points: [{ timestamp: "2026-07-16T00:00:00Z", value: 80 }],
        },
      ],
    });
    metricsState.set("memory_limit", {
      ...EMPTY_METRIC,
      series: [
        {
          unit: "bytes",
          labels: {},
          points: [{ timestamp: "2026-07-16T00:00:00Z", value: 100 }],
        },
      ],
    });
    renderCompute();

    await screen.findByText("Average Memory Utilization");
    // Memory draws its averaged line; CPU and Instances show the empty state.
    expect(screen.getAllByRole("img")).toHaveLength(1);
    expect(
      screen.getAllByText("No data captured in the past 48 hours"),
    ).toHaveLength(2);
  });

  // w5/m125: a sleeping service has no pod, so its current-limit reads are
  // empty. With no surviving percentage either, that is not "no limit".
  it("never calls a sleeping service limitless", async () => {
    serverState.service = svc({ plan: "free", phase: "Hibernated" });
    for (const metric of ["memory", "cpu"])
      metricsState.set(metric, {
        ...EMPTY_METRIC,
        series: [
          {
            unit: metric === "memory" ? "bytes" : "cpu",
            labels: { instance: "a" },
            points: [{ timestamp: "2026-07-16T00:00:00Z", value: 40 }],
          },
        ],
      });
    renderCompute();

    expect(
      await screen.findAllByText(
        "Percentages unavailable — no trustworthy limit for this window",
      ),
    ).toHaveLength(2);
    expect(
      screen.queryByText("No limit configured — percentage is undefined"),
    ).toBeNull();
  });

  // w4/688: while only the autoscaling read is pending, the page draws the
  // autoscaling-off frame — the same cards, headers, copy and row heights the
  // ready page lands on — so neither card nor Recent Metrics moves.
  describe("pending autoscaling read keeps the ready frame (w4/688)", () => {
    // jsdom has no layout, so compare what sets the geometry: each card's
    // regions and copy, the header's control slot, and the height class of
    // every row in the manual card's body.
    // A row is as tall as its tallest fixed-height box (the slider track is
    // h-1.5 inside the h-8 input's row).
    function heightOf(el: Element): string | undefined {
      const heights = [el, ...el.querySelectorAll("*")].flatMap((n) =>
        [...n.classList].filter((c) => /^h-\d/.test(c)),
      );
      return heights.sort(
        (a, b) => parseFloat(b.slice(2)) - parseFloat(a.slice(2)),
      )[0];
    }
    function frame(container: HTMLElement) {
      return [...container.querySelectorAll('[data-slot="card"]')].map(
        (card) => {
          const header = card.querySelector('[data-slot="card-header"]')!;
          const control = header.querySelector(".shrink-0");
          const body = card.querySelector(
            '[data-slot="card-content"] .space-y-3',
          );
          return {
            regions: [...card.children].map((c) => c.getAttribute("data-slot")),
            title: card.querySelector('[data-slot="card-title"]')?.textContent,
            description: card.querySelector('[data-slot="card-description"]')
              ?.textContent,
            copy: [...card.querySelectorAll('[data-slot="card-content"] p')]
              .map((el) => el.textContent)
              .filter(Boolean),
            control: control ? control.children.length : null,
            rows: body ? [...body.children].map(heightOf) : null,
          };
        },
      );
    }

    it.each([
      ["free", "web_service"],
      ["starter", "web_service"],
      ["free", "private_service"],
    ] as const)("on a %s %s", async (plan, type) => {
      serverState.service = svc({ type, plan, replicas: 1 });
      autoscalingState.loading = true;
      const pending = renderCompute();
      expect(
        await screen.findByTestId("autoscaling-loading"),
      ).toBeInTheDocument();
      expect(screen.getByTestId("manual-scaling-loading")).toBeInTheDocument();
      // No control is offered before the state behind it is known.
      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      expect(screen.queryByRole("spinbutton")).not.toBeInTheDocument();
      const pendingFrame = frame(pending.container);
      pending.unmount();

      autoscalingState.loading = false;
      const ready = renderCompute();
      expect(
        await screen.findByRole("switch", { name: "Autoscaling off" }),
      ).toBeInTheDocument();
      // The Compute, autoscaling and manual cards; Recent Metrics renders the
      // same component in both states and follows them.
      expect(pendingFrame.slice(0, 3)).toEqual(
        frame(ready.container).slice(0, 3),
      );
      expect(pendingFrame.map((card) => card.title).slice(0, 3)).toEqual([
        "Compute",
        "Autoscaling",
        "Manual Scaling",
      ]);
    });
  });
});

// w6/m483: Render's Compute page — "Under Compute, click Edit … Select a new
// compute plan and click Save" (render.com/docs/compute-plans), then the
// scaling sections (docs/render-artifacts/compute-page.md).
describe("ServiceComputePage Compute section (w6/m483)", () => {
  it("leads with the Compute section, then Autoscaling, Manual Scaling and Recent Metrics", async () => {
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    await screen.findByText("Manual Scaling");
    expect(
      screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent),
    ).toEqual(["Compute", "Autoscaling", "Manual Scaling", "Recent Metrics"]);
  });

  it("shows the current plan by Render's plan ID beside its legacy name", async () => {
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    const section = (
      await screen.findByRole("heading", { name: "Compute" })
    ).closest('[data-slot="card"]') as HTMLElement;
    expect(within(section).getByText("0.5c-512mb")).toBeInTheDocument();
    expect(within(section).getByText("Starter")).toBeInTheDocument();
    expect(section).toHaveTextContent("0.5 CPU");
    expect(section).toHaveTextContent("512 MB");
    // Collapsed: no plan cards until Edit.
    expect(within(section).queryByRole("radiogroup")).toBeNull();
  });

  it("opens the plan cards on Edit and closes them on Cancel without saving", async () => {
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const cards = screen.getByRole("radiogroup", { name: "Compute plan" });
    expect(
      within(cards).getByRole("radio", { name: /0\.5c-512mb Starter/ }),
    ).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(updatePlan).not.toHaveBeenCalled();
  });

  it("saves a picked plan after confirming, then closes the cards", async () => {
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("radio", { name: /1c-2g Standard/ }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText("Change compute plan to 1c-2g (Standard)?"),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updatePlan).toHaveBeenCalledWith(
        "app",
        "standard",
        "1c-2g (Standard)",
      ),
    );
    await waitFor(() => expect(screen.queryByRole("radiogroup")).toBeNull());
  });

  it("keeps the cards open when the change is refused", async () => {
    updatePlan.mockResolvedValueOnce(null);
    serverState.service = svc({ plan: "starter" });
    renderCompute();

    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("radio", { name: /1c-2g Standard/ }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() => expect(updatePlan).toHaveBeenCalled());
    expect(screen.getByRole("radiogroup")).toBeInTheDocument();
  });

  it("anchors the Manual Scaling section where the old Scaling URL lands", async () => {
    renderCompute();

    const manual = (await screen.findByText("Manual Scaling")).closest(
      "#manual-scaling",
    );
    expect(manual).not.toBeNull();
    // While autoscaling hides the manual card, no empty anchor is left behind.
    autoscalingState.enabled = true;
    const { container } = renderCompute();
    await within(container).findByText("Autoscaling");
    expect(container.querySelector("#manual-scaling")).toBeNull();
  });
});
