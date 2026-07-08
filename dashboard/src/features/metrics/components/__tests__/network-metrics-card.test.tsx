import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { NetworkMetricsCard } from "../network-metrics-card";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";
import { useMonthToDateBandwidth } from "@/features/metrics/hooks/use-month-to-date-bandwidth";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-month-to-date-bandwidth", () => ({
  useMonthToDateBandwidth: vi.fn(),
}));

const mockUseMetrics = vi.mocked(useMetrics);
const mockUseMonthToDateBandwidth = vi.mocked(useMonthToDateBandwidth);

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

function renderCard(statusCode = "") {
  return render(
    <NetworkMetricsCard
      resource="beancount-cms"
      window={WINDOW}
      quantile={0.95}
      statusCode={statusCode}
    />,
  );
}

describe("NetworkMetricsCard", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockUseMonthToDateBandwidth.mockReset();
    mockUseMonthToDateBandwidth.mockReturnValue({
      egressBandwidthMB: null,
      loading: false,
      error: undefined,
    });
  });

  it("queries the shared window for all three request metrics, and passes quantile only to latency", () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_requests",
      expect.objectContaining(WINDOW),
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "bandwidth",
      WINDOW,
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      expect.objectContaining({ ...WINDOW, quantile: 0.95 }),
    );
  });

  it("applies the status-code filter to requests and latency but never bandwidth (no code label on the bytes counter)", () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard("5xx");

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_requests",
      expect.objectContaining({ statusCode: "5xx" }),
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      expect.objectContaining({ statusCode: "5xx" }),
    );
    const bandwidthCall = mockUseMetrics.mock.calls.find(
      ([, metric]) => metric === "bandwidth",
    )!;
    expect(bandwidthCall[2]).not.toHaveProperty("statusCode");
  });

  it("renders a populated chart for a metric with data", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "http_requests") {
        return {
          series: [
            {
              unit: "count",
              labels: {},
              points: [{ timestamp: "2026-07-06T09:00:00Z", value: 1 }],
            },
          ],
          loading: false,
          unavailable: false,
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard();

    expect(
      screen.getByRole("img", { name: /Bar chart with 1 data points/ }),
    ).toBeInTheDocument();
  });

  it("stacks grouped request series into shared time buckets", () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "http_requests" && opts && "groupBy" in opts) {
        return {
          series: [
            {
              unit: "count",
              labels: { code: "200" },
              points: [{ timestamp: "2026-07-06T09:00:00Z", value: 3 }],
            },
            {
              unit: "count",
              labels: { code: "500" },
              points: [{ timestamp: "2026-07-06T09:00:00Z", value: 1 }],
            },
          ],
          loading: false,
          unavailable: false,
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard();

    // Group-by is off by default, so series labels aren't shown as a legend…
    expect(screen.queryByText("200")).not.toBeInTheDocument();
    // …but both series stack into the single shared time bucket.
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

    renderCard();

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

    renderCard();

    expect(screen.getByText(/used this month/)).toBeInTheDocument();
  });
});
