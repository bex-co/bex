import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceScalingPage } from "../services.$serviceId.scaling";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";

// The Scaling page composes useServer (type gating + replicas), useAutoscaling
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
  save: vi.fn(async () => true),
  disable: vi.fn(async () => true),
};
vi.mock("@/features/services/hooks/use-autoscaling", () => ({
  useAutoscaling: () => autoscalingState,
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
const metricsState = new Map<string, UseMetricsResult>();
vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: (_resource: string, metric: string) =>
    metricsState.get(metric) ?? EMPTY_METRIC,
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
    buildFilter: null,
    autoDeploy: null,
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
function renderScaling(serviceId = "app") {
  const rootRoute = createRootRoute();
  const scalingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/scaling",
    component: () => <ServiceScalingPage serviceId={serviceId} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([scalingRoute]),
    history: createMemoryHistory({
      initialEntries: [`/services/${serviceId}/scaling`],
    }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = svc();
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
  metricsState.clear();
});

describe("ServiceScalingPage (w7/m43)", () => {
  it.each(["web_service", "private_service"] as const)(
    "explains and disables autoscaling for a free %s before any mutation",
    async (type) => {
      serverState.service = svc({ type, plan: "free", replicas: 1 });
      renderScaling();

      const toggle = await screen.findByRole("switch", {
        name: "Autoscaling off",
      });
      expect(toggle).toBeDisabled();
      expect(
        screen.getByText("Autoscaling is available on paid plans. Upgrade to enable it."),
      ).toBeInTheDocument();
      fireEvent.click(toggle);
      expect(screen.queryByRole("spinbutton", { name: "Maximum instances" })).not.toBeInTheDocument();
      expect(screen.getByRole("spinbutton", { name: "Instances" })).toHaveAttribute("max", "1");
      expect(screen.getByRole("spinbutton", { name: "Instances" })).toBeDisabled();
      expect(screen.queryByRole("slider", { name: "Instances" })).not.toBeInTheDocument();
      expect(autoscalingState.save).not.toHaveBeenCalled();
      expect(autoscalingState.disable).not.toHaveBeenCalled();
    },
  );

  it("keeps a free service's fixed instance count visible with a stored singleton autoscaling config", async () => {
    serverState.service = svc({ plan: "free", replicas: 1 });
    autoscalingState.enabled = true;
    autoscalingState.maxInstances = 1;
    autoscalingState.targetCPUPercent = 60;
    renderScaling();

    expect(await screen.findByRole("switch", { name: "Autoscaling off" })).toBeDisabled();
    expect(screen.getByRole("spinbutton", { name: "Instances" })).toHaveValue(1);
    expect(screen.queryByRole("spinbutton", { name: "Maximum instances" })).not.toBeInTheDocument();
    expect(autoscalingState.save).not.toHaveBeenCalled();
    expect(autoscalingState.disable).not.toHaveBeenCalled();
  });

  it.each(["web_service", "private_service", "background_worker"] as const)(
    "preserves the paid %s editor and its 1–100 instance range",
    async (type) => {
      serverState.service = svc({ type, plan: "starter" });
      renderScaling();

      fireEvent.click(await screen.findByRole("switch", { name: "Autoscaling off" }));
      const maximum = screen.getByRole("spinbutton", { name: "Maximum instances" });
      expect(maximum).toHaveAttribute("max", "100");
      expect(screen.getByRole("slider", { name: "Minimum" })).toHaveAttribute("aria-valuemin", "1");
      expect(screen.getByRole("slider", { name: "Maximum" })).toHaveAttribute("aria-valuemax", "100");
      fireEvent.change(maximum, { target: { value: "100" } });
      fireEvent.click(screen.getByRole("switch", { name: "Target CPU Utilization" }));
      const card = screen.getByText("Autoscaling").closest('[data-slot="card"]') as HTMLElement;
      fireEvent.click(within(card).getByRole("button", { name: "Save Changes" }));
      await waitFor(() => expect(autoscalingState.save).toHaveBeenCalledWith({
        minInstances: 1,
        maxInstances: 100,
        targetCPUPercent: 60,
        targetMemoryPercent: null,
      }));
    },
  );

  it("shows Autoscaling + Manual Scaling + Recent Metrics when autoscaling is off", async () => {
    renderScaling();

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
    renderScaling();

    expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
    expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
    // Recent Metrics shows in both states.
    expect(screen.getByText("Recent Metrics")).toBeInTheDocument();
  });

  it("names CPU and memory controls independently and preserves range thumb names", async () => {
    autoscalingState.enabled = true;
    autoscalingState.targetCPUPercent = 60;
    autoscalingState.targetMemoryPercent = 70;
    renderScaling();
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

  it("does not flash Autoscaling while the service type is still loading", () => {
    serverState.service = null;
    renderScaling();
    expect(screen.queryByText("Autoscaling")).not.toBeInTheDocument();
    expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Scaling isn't available"),
    ).not.toBeInTheDocument();
  });

  it.each(["cron_job", "static_site"] as const)(
    "explains that Scaling is unavailable for a %s (no replica concept)",
    async (type) => {
      serverState.service = svc({ type });
      renderScaling();

      expect(
        await screen.findByText("Scaling isn't available"),
      ).toBeInTheDocument();
      expect(screen.queryByText("Autoscaling")).not.toBeInTheDocument();
      expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
      expect(screen.queryByText("Recent Metrics")).not.toBeInTheDocument();
    },
  );

  it("still offers Autoscaling for a background_worker", async () => {
    serverState.service = svc({ type: "background_worker" });
    renderScaling();
    expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
    expect(screen.getByText("Manual Scaling")).toBeInTheDocument();
  });

  it("saves a drafted manual instance count through scaleService", async () => {
    renderScaling();
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
    renderScaling();
    await screen.findByText("Manual Scaling");

    const input = screen.getByRole("spinbutton", { name: "Instances" });
    fireEvent.change(input, { target: { value: "5" } });
    fireEvent.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => expect(scaleService).toHaveBeenCalledWith("app", 5));
    expect(input).toHaveValue(5);
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeEnabled();
  });

  it("clamps the manual instance input to 1–100", async () => {
    renderScaling();
    await screen.findByText("Manual Scaling");

    const input = screen.getByRole("spinbutton", { name: "Instances" });
    fireEvent.change(input, { target: { value: "250" } });
    expect(input).toHaveValue(100);
    fireEvent.change(input, { target: { value: "0" } });
    expect(input).toHaveValue(1);
  });

  it("confirms before disabling a server-enabled autoscaling config", async () => {
    autoscalingState.enabled = true;
    renderScaling();

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
    renderScaling();

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
    renderScaling();

    await screen.findByText("Average Memory Utilization");
    // Memory has data ⇒ only CPU + Instances show the empty state.
    expect(
      screen.getAllByText("No data captured in the past 48 hours"),
    ).toHaveLength(2);
  });
});
