import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { ApplicationMetricsCard } from "../application-metrics-card";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));

const mockUseMetrics = vi.mocked(useMetrics);

function emptyResult() {
  return { series: [], loading: false, unavailable: false, error: undefined };
}

function seriesResult(unit: string, values: number[]) {
  return {
    series: [
      {
        unit,
        labels: {},
        points: values.map((value, i) => ({ timestamp: `t${i}`, value })),
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

  it("fetches cpu/memory raw and their _limit counterparts (aggregateMax), plus instance_count", () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    render(<ApplicationMetricsCard resource="beancount-cms" percentage />);

    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu");
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "memory");
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu_limit", {
      aggregateMax: true,
    });
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory_limit",
      { aggregateMax: true },
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "instance_count",
    );
  });

  it("renders the latest absolute value from each series as a stat when percentage is off", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") return seriesResult("bytes", [100, 200]);
      if (metric === "instance_count") return seriesResult("count", [3]);
      return emptyResult();
    });

    render(<ApplicationMetricsCard resource="app" percentage={false} />);

    // Memory takes the LAST point (200 bytes), not the first.
    expect(screen.getByText("200 B")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("computes a client-side percentage from the raw metric and its _limit series", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") return seriesResult("bytes", [50]);
      if (metric === "memory_limit") return seriesResult("bytes", [200]);
      return emptyResult();
    });

    render(<ApplicationMetricsCard resource="app" percentage />);

    expect(screen.getByText("25.0%")).toBeInTheDocument();
  });

  it("renders — for percentage when the limit series is empty (no pod limit configured)", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "cpu") return seriesResult("cpu", [0.5]);
      return emptyResult();
    });

    render(<ApplicationMetricsCard resource="app" percentage />);

    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  it("renders the unavailable state instead of a stat when a metric has no source", () => {
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

    render(<ApplicationMetricsCard resource="app" percentage={false} />);

    expect(
      screen.getByText("Metrics source not configured"),
    ).toBeInTheDocument();
    // Memory and Total Instances still render as ordinary (empty) stats, not unavailable.
    expect(screen.getAllByText("—")).toHaveLength(2);
  });
});
