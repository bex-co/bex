import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MetricSection } from "../metric-section";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";
import type { ChartSeries } from "@/features/metrics/types";

function result(over: Partial<UseMetricsResult> = {}): UseMetricsResult {
  return {
    series: [],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    error: undefined,
    degradedSources: [],
    ...over,
  };
}

const CHILD = <div data-testid="chart">chart</div>;
const aSeries = { label: "cpu", points: [{ t: 0, v: 1 }] } as unknown as ChartSeries;

// w9/m63 t002: while a metric's first fetch is in flight (no series yet), the
// section must show a loading shimmer — NOT its child chart, which would render
// the "No data in range" empty state and read as broken data.
describe("MetricSection loading state", () => {
  it("shows a shimmer (not the chart) while loading with no series yet", () => {
    render(
      <MetricSection title="CPU" result={result({ loading: true, series: [] })}>
        {CHILD}
      </MetricSection>,
    );
    expect(screen.queryByTestId("chart")).toBeNull();
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("renders the chart once resolved (even if empty)", () => {
    render(
      <MetricSection title="CPU" result={result({ loading: false, series: [] })}>
        {CHILD}
      </MetricSection>,
    );
    // Resolved-empty is the child's job (EmptyChart), not the section's shimmer.
    expect(screen.getByTestId("chart")).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("keeps the chart during a poll refetch over existing data", () => {
    render(
      <MetricSection
        title="CPU"
        result={result({ loading: true, series: [aSeries] })}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByTestId("chart")).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });
});
