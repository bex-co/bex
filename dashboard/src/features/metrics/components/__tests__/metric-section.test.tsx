import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MetricSection } from "../metric-section";
import { EmptyChart, SvgLineChart } from "../svg-line-chart";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";
import type { ChartSeries } from "@/features/metrics/types";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

function result(over: Partial<UseMetricsResult> = {}): UseMetricsResult {
  return {
    series: [],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    throttled: false,
    error: undefined,
    degradedSources: [],
    ...over,
  };
}

const CHILD = <div data-testid="chart">chart</div>;
const aSeries = {
  label: "cpu",
  points: [{ t: 0, v: 1 }],
} as unknown as ChartSeries;

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
      <MetricSection
        title="CPU"
        result={result({ loading: false, series: [] })}
      >
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

// w4/m100 t003: a throttled read must never look like empty data or an
// unwired metrics backend — cold loads get a throttle card; warm caches keep
// the series with an explicit staleness note.
describe("MetricSection throttled state", () => {
  it("renders a throttle-specific card on a cold shed (no cached series)", () => {
    render(
      <MetricSection
        title="CPU"
        result={result({ throttled: true, series: [] })}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.queryByTestId("chart")).toBeNull();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Catching up — traffic was briefly limited",
    );
  });

  it("keeps cached series and names the staleness on a warm shed", () => {
    render(
      <MetricSection
        title="CPU"
        result={result({ throttled: true, series: [aSeries] })}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByTestId("chart")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Showing cached data — refreshing after a brief limit",
    );
  });
});

// The body's overflow-wrap:anywhere lets a flex item shrink below its longest
// word, so a wide headerExtra squeezed "Disk" to one letter per line at 390px.
describe("MetricSection title", () => {
  it("keeps the title's words whole beside a header extra", () => {
    render(
      <MetricSection
        title="Disk"
        result={result()}
        headerExtra={<div>Disk autoscaling</div>}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByText("Disk")).toHaveClass("[overflow-wrap:normal]");
  });
});

// w8/m138: a plan refusal of the section's feature is a plan notice, not an
// error card; another feature's refusal keeps the generic error.
describe("MetricSection plan refusal", () => {
  const refusal = (feature: string) =>
    new CombinedGraphQLErrors({
      errors: [
        {
          message: "bad request: response latency metrics require the Pro plan",
          extensions: { code: "PLAN_FEATURE_REQUIRED", plan: "hobby", feature },
        },
      ],
    } as never);
  const plan = { feature: "latencyMetrics", message: "Needs the Pro plan." };

  it("shows the plan message in place of the chart", () => {
    render(
      <MetricSection
        title="Response Times"
        result={result({ error: refusal("latencyMetrics") })}
        planFeature={plan}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.getByText("Needs the Pro plan.")).toBeInTheDocument();
    expect(screen.queryByTestId("chart")).toBeNull();
  });

  it("keeps the error card for another feature's refusal", () => {
    render(
      <MetricSection
        title="Response Times"
        result={result({ error: refusal("auditLogs") })}
        planFeature={plan}
      >
        {CHILD}
      </MetricSection>,
    );
    expect(screen.queryByText("Needs the Pro plan.")).toBeNull();
  });
});

// w4/446: a service chart's 24px event lane sits above the plot whether the
// section is loading, failed, empty or drawn, so no state change moves it.
describe("MetricSection event lane", () => {
  const lanes = (container: HTMLElement) =>
    container.querySelectorAll(":scope > div > .h-6").length +
    container.querySelectorAll(":scope > div > div.relative > .h-6").length;

  it.each([
    ["loading", result({ loading: true }), null],
    ["failed", result({ error: new Error("boom") }), null],
    ["empty", result(), <EmptyChart key="e" />],
    [
      "drawn without markers",
      result({ series: [aSeries] }),
      <SvgLineChart
        key="c"
        unit="count"
        series={[
          {
            label: "n",
            color: "red",
            points: [
              { timestamp: "2026-11-01T00:00:00Z", value: 1 },
              { timestamp: "2026-11-01T00:01:00Z", value: 2 },
            ],
          },
        ]}
        markersServiceId="srv-1"
      />,
    ],
  ])("reserves exactly one lane while %s", (_state, res, child) => {
    const { container } = render(
      <MetricSection title="CPU" result={res} eventLane>
        {child ?? CHILD}
      </MetricSection>,
    );
    expect(lanes(container)).toBe(1);
  });

  it("reserves no lane on a section without one", () => {
    const { container } = render(
      <MetricSection title="CPU" result={result({ loading: true })}>
        {CHILD}
      </MetricSection>,
    );
    expect(lanes(container)).toBe(0);
  });
});
