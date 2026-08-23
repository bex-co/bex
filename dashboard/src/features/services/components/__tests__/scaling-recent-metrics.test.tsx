import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ScalingRecentMetrics } from "../scaling-recent-metrics";
import {
  useMetrics,
  type UseMetricsResult,
} from "@/features/metrics/hooks/use-metrics";
import type { MetricId } from "@/features/metrics/types";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));

vi.mock("@/features/metrics/hooks/use-live-range", () => ({
  useLiveRange: () => ({
    startTime: "2026-08-21T00:00:00.000Z",
    endTime: "2026-08-23T00:00:00.000Z",
    resolutionSeconds: 24 * 60,
  }),
}));

const mockUseMetrics = vi.mocked(useMetrics);

function emptyResult(): UseMetricsResult {
  return {
    series: [],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    error: undefined,
    degradedSources: [],
  };
}

function errorResult(): UseMetricsResult {
  return { ...emptyResult(), error: new Error("boom") };
}

function renderPanel(impl: (metric: MetricId) => UseMetricsResult) {
  mockUseMetrics.mockImplementation(
    (_resource: string, metric: MetricId) => impl(metric),
  );
  const rootRoute = createRootRoute();
  const scalingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <ScalingRecentMetrics serviceId="app" />,
  });
  const metricsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/metrics",
    component: () => <div>metrics page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([scalingRoute, metricsRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  mockUseMetrics.mockReset();
});

// w9/049 (m86 residual): the Scaling page's Recent Metrics sections route their
// result through the shared MetricSection, so a FAILED query must render the
// distinct error card — never the "No data captured…" empty state (the
// error-vs-empty conflation that hid the w5/m71 wrong-identifier bug).
describe("ScalingRecentMetrics error-vs-empty", () => {
  it("renders the error card, not the empty state, when every query fails", async () => {
    renderPanel(() => errorResult());

    // Memory, CPU, and Total Instances sections each render MetricError.
    await screen.findAllByRole("alert");
    expect(screen.getAllByRole("alert")).toHaveLength(3);
    expect(
      screen.getAllByText("Couldn't load this metric").length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByText("No data captured in the past 48 hours"),
    ).not.toBeInTheDocument();
  });

  it("renders the error card only for the failed section, keeping the others' empty state", async () => {
    renderPanel((metric) =>
      metric === "instance_count" ? errorResult() : emptyResult(),
    );

    await screen.findByRole("alert");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    // The healthy memory/cpu sections still read as genuinely empty.
    expect(
      screen.getAllByText("No data captured in the past 48 hours").length,
    ).toBeGreaterThan(0);
  });

  it("keeps the honest empty state (no error card) when the window is genuinely empty", async () => {
    renderPanel(() => emptyResult());

    await screen.findAllByText("No data captured in the past 48 hours");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(
      screen.getAllByText("No data captured in the past 48 hours").length,
    ).toBeGreaterThan(0);
  });
});
