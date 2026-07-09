import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { ApplicationMetricsCard } from "../application-metrics-card";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));

const mockUseMetrics = vi.mocked(useMetrics);

// The page-level resolved live window, passed down by the route.
const WINDOW = {
  startTime: "2026-07-06T09:00:00Z",
  endTime: "2026-07-06T10:00:00Z",
  resolutionSeconds: 30,
  pollIntervalMs: 0,
};

function emptyResult() {
  return { series: [], loading: false, unavailable: false, error: undefined };
}

/** A stepped series with real timestamps (the chart maps x by time). */
function seriesResult(unit: string, values: number[], instance?: string) {
  return {
    series: [
      {
        unit,
        labels: instance ? { instance } : {},
        points: values.map((value, i) => ({
          timestamp: new Date(1_751_800_000_000 + i * 60_000).toISOString(),
          value,
        })),
      },
    ],
    loading: false,
    unavailable: false,
    error: undefined,
  };
}

describe("ApplicationMetricsCard", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
  });

  function renderCard(percentage: boolean, resource = "app") {
    return render(
      <ApplicationMetricsCard
        resource={resource}
        percentage={percentage}
        window={WINDOW}
      />,
    );
  }

  it("fetches cpu/memory/instances over the shared window; the _limit queries ride the same window aggregated", () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard(true, "beancount-cms");

    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu", WINDOW);
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory",
      WINDOW,
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "instance_count",
      WINDOW,
    );
    // Limits share the window's cadence (no second Apollo poll timer).
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu_limit", {
      ...WINDOW,
      aggregateMax: true,
    });
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory_limit",
      { ...WINDOW, aggregateMax: true },
    );
  });

  it("draws a multi-point history chart per metric on the Total tab, with the latest value in the header", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") return seriesResult("bytes", [100, 200]);
      if (metric === "instance_count") return seriesResult("count", [3, 3]);
      return emptyResult();
    });

    renderCard(false);

    // Memory renders a 2-point line chart, not a lone stat…
    expect(
      screen.getAllByRole("img", { name: /2 data points/ }).length,
    ).toBeGreaterThanOrEqual(2); // memory + instances
    // …and its header shows the LAST point (200 bytes), not the first —
    // getAllBy: the y-axis max tick legitimately shows the same value.
    expect(screen.getAllByText("200 B").length).toBeGreaterThan(0);
    expect(screen.getAllByText("3").length).toBeGreaterThan(0);
  });

  it("scales every percentage point by the _limit series and shows the limit label", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") return seriesResult("bytes", [50, 100]);
      if (metric === "memory_limit") return seriesResult("bytes", [200]);
      return emptyResult();
    });

    renderCard(true);

    // Latest 100/200 => 50.0%; the limit label renders alongside.
    expect(screen.getByText("50.0%")).toBeInTheDocument();
    expect(screen.getByText("Limit 200 B")).toBeInTheDocument();
  });

  it("shows the honest no-limit state for percentage when no _limit series exists", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "cpu") return seriesResult("cpu", [0.5]);
      return emptyResult();
    });

    renderCard(true);

    expect(screen.getAllByText(/No limit configured/).length).toBeGreaterThan(
      0,
    );
  });

  it("draws one line per instance and a legend naming each", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") {
        return {
          series: [
            seriesResult("bytes", [1, 2], "web-a").series[0],
            seriesResult("bytes", [3, 4], "web-b").series[0],
          ],
          loading: false,
          unavailable: false,
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard(false);

    expect(screen.getByText("web-a")).toBeInTheDocument();
    expect(screen.getByText("web-b")).toBeInTheDocument();
  });

  it("renders the unavailable state instead of a chart when a metric has no source", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "cpu") {
        return {
          series: [],
          loading: false,
          unavailable: true,
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard(false);

    expect(
      screen.getByText("Metrics source not configured"),
    ).toBeInTheDocument();
    // Memory and Total Instances still render their (empty) charts, not unavailable.
    expect(screen.getAllByText("No data in range")).toHaveLength(2);
  });
});
