import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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
    schedule: null,
    command: null,
    runs: [],
    healthCheckPath: null,
    maxShutdownDelaySeconds: 30,
    registryCredentialId: null,
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
  autoscalingState.save.mockClear();
  autoscalingState.disable.mockClear();
  scaleService.mockClear();
  scaleService.mockResolvedValue(true);
  metricsState.clear();
});

describe("ServiceScalingPage (w7/m43)", () => {
  it("shows Autoscaling + Manual Scaling + Recent Metrics when autoscaling is off", async () => {
    renderScaling();

    expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
    expect(screen.getByText("Manual Scaling")).toBeInTheDocument();
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

  it.each(["cron_job", "static_site"] as const)(
    "hides the Manual Scaling card for a %s (no replica concept)",
    async (type) => {
      serverState.service = svc({ type });
      renderScaling();

      expect(await screen.findByText("Autoscaling")).toBeInTheDocument();
      expect(screen.queryByText("Manual Scaling")).not.toBeInTheDocument();
    },
  );

  it("saves a drafted manual instance count through scaleService", async () => {
    renderScaling();
    await screen.findByText("Manual Scaling");

    // Save is disabled until the draft differs from the live count (2).
    const save = screen.getByRole("button", { name: "Save Changes" });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByRole("spinbutton"), {
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

    const input = screen.getByRole("spinbutton");
    fireEvent.change(input, { target: { value: "5" } });
    fireEvent.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => expect(scaleService).toHaveBeenCalledWith("app", 5));
    expect(input).toHaveValue(5);
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeEnabled();
  });

  it("clamps the manual instance input to 1–100", async () => {
    renderScaling();
    await screen.findByText("Manual Scaling");

    const input = screen.getByRole("spinbutton");
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
