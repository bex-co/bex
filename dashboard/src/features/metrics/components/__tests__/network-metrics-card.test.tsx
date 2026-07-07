import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { NetworkMetricsCard } from "../network-metrics-card";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";
import { useMonthToDateBandwidth } from "@/features/metrics/hooks/use-month-to-date-bandwidth";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { RANGE_PRESETS } from "@/features/metrics/lib/range";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-month-to-date-bandwidth", () => ({
  useMonthToDateBandwidth: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-live-range", () => ({
  useLiveRange: vi.fn(),
}));

const mockUseMetrics = vi.mocked(useMetrics);
const mockUseMonthToDateBandwidth = vi.mocked(useMonthToDateBandwidth);
const mockUseLiveRange = vi.mocked(useLiveRange);

function emptyResult() {
  return { series: [], loading: false, unavailable: false, error: undefined };
}

describe("NetworkMetricsCard", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockUseLiveRange.mockReset();
    mockUseLiveRange.mockReturnValue({
      startTime: "2026-07-06T09:00:00Z",
      endTime: "2026-07-06T10:00:00Z",
      resolutionSeconds: 30,
    });
    mockUseMonthToDateBandwidth.mockReset();
    mockUseMonthToDateBandwidth.mockReturnValue({
      egressBandwidthMB: null,
      loading: false,
      error: undefined,
    });
  });

  it("queries the live range's resolved window for all three request metrics, and passes quantile only to latency", () => {
    mockUseMetrics.mockReturnValue(emptyResult());
    const range = RANGE_PRESETS.find((p) => p.id === "1h")!;

    render(
      <NetworkMetricsCard
        resource="beancount-cms"
        range={range}
        quantile={0.95}
      />,
    );

    // pollIntervalMs: 0 disables Apollo's own poll timer — useLiveRange's tick
    // (asserted separately) already forces a refetch every cycle, so a second,
    // out-of-phase timer would only add redundant requests.
    const expectedWindow = {
      startTime: "2026-07-06T09:00:00Z",
      endTime: "2026-07-06T10:00:00Z",
      resolutionSeconds: 30,
      pollIntervalMs: 0,
    };
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_requests",
      expectedWindow,
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "bandwidth",
      expectedWindow,
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      {
        ...expectedWindow,
        quantile: 0.95,
      },
    );
  });

  it("renders a populated chart for a metric with data", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "http_requests") {
        return {
          series: [
            {
              unit: "count",
              labels: {},
              points: [{ timestamp: "t", value: 1 }],
            },
          ],
          loading: false,
          unavailable: false,
          error: undefined,
        };
      }
      return emptyResult();
    });

    render(
      <NetworkMetricsCard
        resource="app"
        range={RANGE_PRESETS.find((p) => p.id === "1h")!}
        quantile={0.95}
      />,
    );

    expect(
      screen.getByRole("img", { name: /Bar chart with 1 data points/ }),
    ).toBeInTheDocument();
  });

  it("renders the unavailable state for a metric with no backend, without affecting the others", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "http_requests") {
        return {
          series: [],
          loading: false,
          unavailable: true,
          error: undefined,
        };
      }
      return emptyResult();
    });

    render(
      <NetworkMetricsCard
        resource="app"
        range={RANGE_PRESETS.find((p) => p.id === "1h")!}
        quantile={0.95}
      />,
    );

    expect(
      screen.getByText("Metrics source not configured"),
    ).toBeInTheDocument();
    // bandwidth/latency still render their (empty) chart, not the unavailable state.
    expect(screen.getAllByText("No data in range")).toHaveLength(2);
  });

  it("shows the month-to-date bandwidth footer once monthToDateBandwidth resolves", () => {
    mockUseMetrics.mockReturnValue(emptyResult());
    mockUseMonthToDateBandwidth.mockReturnValue({
      egressBandwidthMB: 512,
      loading: false,
      error: undefined,
    });

    render(
      <NetworkMetricsCard
        resource="app"
        range={RANGE_PRESETS.find((p) => p.id === "1h")!}
        quantile={0.95}
      />,
    );

    expect(screen.getByText(/used this month/)).toBeInTheDocument();
  });
});
