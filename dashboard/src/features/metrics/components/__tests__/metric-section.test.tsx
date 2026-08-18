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

// w9/m86: a query that FAILS (a real error, not the recognized 503s) with no
// series must render a DISTINCT error card, not the child chart's "No data in
// range" — the conflation that hid the w5/m71 wrong-identifier bug for months.
describe("MetricSection error state", () => {
  it("renders a distinct error card (not the chart) for a failed query", () => {
    render(
      <MetricSection
        title="Disk"
        result={result({ error: new Error("network down"), series: [] })}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.queryByTestId("chart")).toBeNull();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load this metric",
    );
  });

  it("renders the child (empty 'No data in range') when the window is genuinely empty", () => {
    render(
      <MetricSection title="Disk" result={result({ series: [] })}>
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByTestId("chart")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps the chart when a refetch errors over existing data", () => {
    render(
      <MetricSection
        title="Disk"
        result={result({ error: new Error("blip"), series: [aSeries] })}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByTestId("chart")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("uses the section-specific errorMessage when given (bandwidth)", () => {
    render(
      <MetricSection
        title="Bandwidth"
        result={result({ error: new Error("boom"), series: [] })}
        errorMessage="Couldn't load bandwidth"
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load bandwidth",
    );
  });

  it("still shows the not-configured 503 state, never the error card, when unavailable", () => {
    render(
      <MetricSection title="Disk" result={result({ unavailable: true })}>
        {CHILD}
      </MetricSection>,
    );
    // MetricUnavailable, not MetricError — the two states stay distinct.
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByTestId("chart")).toBeNull();
  });
});
