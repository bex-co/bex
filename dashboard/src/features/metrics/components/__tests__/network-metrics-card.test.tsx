import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NetworkMetricsCard } from "../network-metrics-card";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";
import { useMonthToDateBandwidth } from "@/features/metrics/hooks/use-month-to-date-bandwidth";

// Radix Select uses the Pointer Capture API jsdom doesn't implement — same
// polyfill the log-filter-bar and create-wizard select tests carry.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-month-to-date-bandwidth", () => ({
  useMonthToDateBandwidth: vi.fn(),
}));
// Status-code discovery goes through Apollo; stub it at the hook boundary so
// the render needs no ApolloProvider (the repo's panel-test pattern).
vi.mock("@/features/metrics/hooks/use-metrics-filter-values", () => ({
  useMetricsFilterValues: () => ["404"],
}));
// Host discovery reuses the logs label-values read (also Apollo) — stub it the
// same way so the Host dropdown has options without an ApolloProvider (w5/m58).
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelValues: () => ["web.onbex.co", "www.example.com"],
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
  return {
    series: [],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    error: undefined,
    degradedSources: [],
  };
}

// The Status Code + Percentile controls live on the card itself now (w5/m42),
// so the render takes no filter props — tests drive the card's own dropdowns.
function renderCard() {
  return render(
    <NetworkMetricsCard resource="beancount-cms" window={WINDOW} />,
  );
}

describe("NetworkMetricsCard", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockUseMonthToDateBandwidth.mockReset();
    mockUseMonthToDateBandwidth.mockReturnValue({
      egressBandwidthMB: null,
      degradedSources: [],
      loading: false,
      error: undefined,
    });
  });

  it("queries the shared window for all three request metrics, and passes the default p90 quantile only to latency", () => {
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
      expect.objectContaining({ ...WINDOW, quantile: 0.9 }),
    );
  });

  it("re-queries latency with the quantile picked in the Response Times Percentile dropdown (w5/m42)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.click(screen.getByLabelText("Percentile"));
    await user.click(screen.getByRole("option", { name: "p99" }));

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      expect.objectContaining({ quantile: 0.99 }),
    );
  });

  it("reads all quantiles at once when the Percentile 'All' option is picked (w5/m56)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.click(screen.getByLabelText("Percentile"));
    await user.click(screen.getByRole("option", { name: "All" }));

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      expect.objectContaining({ quantiles: [0.5, 0.9, 0.99] }),
    );
    // In All mode the single-quantile arg is not sent (the overlay path).
    const latest = mockUseMetrics.mock.calls
      .filter(([, metric]) => metric === "http_latency")
      .at(-1);
    expect(latest?.[2]).not.toHaveProperty("quantile");
  });

  it("labels each overlaid latency line p50/p90/p99 in 'All' mode (w5/m56)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "http_latency") {
        return {
          series: [0.5, 0.9, 0.99].map((q) => ({
            unit: "seconds",
            labels: { quantile: String(q) },
            points: [{ timestamp: "2026-07-06T09:00:00Z", value: q }],
          })),
          loading: false,
          unavailable: false,
          degradedSources: [],
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard();
    await user.click(screen.getByLabelText("Percentile"));
    await user.click(screen.getByRole("option", { name: "All" }));

    for (const label of ["p50", "p90", "p99"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("applies its header Status Code filter to requests and latency but never bandwidth (no code label on the bytes counter)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.click(screen.getByLabelText("Status Code"));
    await user.click(screen.getByRole("option", { name: "5xx" }));

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
    const bandwidthCalls = mockUseMetrics.mock.calls.filter(
      ([, metric]) => metric === "bandwidth",
    );
    for (const call of bandwidthCalls) {
      expect(call[2]).not.toHaveProperty("statusCode");
    }
  });

  it("offers the discovered status codes alongside the class presets", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.click(screen.getByLabelText("Status Code"));

    for (const code of ["All", "2xx", "4xx", "5xx", "404"]) {
      expect(screen.getByRole("option", { name: code })).toBeInTheDocument();
    }
  });

  it("populates the Host dropdown from the App's discovered request-log hosts (w5/m58)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.click(screen.getByLabelText("Host"));

    for (const host of ["All", "web.onbex.co", "www.example.com"]) {
      expect(screen.getByRole("option", { name: host })).toBeInTheDocument();
    }
  });

  it("applies its Host filter to requests and latency but never bandwidth (w5/m58)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.click(screen.getByLabelText("Host"));
    await user.click(screen.getByRole("option", { name: "web.onbex.co" }));

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_requests",
      expect.objectContaining({ host: "web.onbex.co" }),
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      expect.objectContaining({ host: "web.onbex.co" }),
    );
    const bandwidthCalls = mockUseMetrics.mock.calls.filter(
      ([, metric]) => metric === "bandwidth",
    );
    for (const call of bandwidthCalls) {
      expect(call[2]).not.toHaveProperty("host");
    }
  });

  it("applies the free-text Path filter to requests and latency on Enter, and clears it (w5/m58)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();
    await user.type(screen.getByLabelText("Path"), "/api{Enter}");

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_requests",
      expect.objectContaining({ path: "/api" }),
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "http_latency",
      expect.objectContaining({ path: "/api" }),
    );

    // The × button clears the active filter (path back to undefined).
    mockUseMetrics.mockClear();
    await user.click(screen.getByLabelText("Clear path filter"));
    const requestsCall = mockUseMetrics.mock.calls.find(
      ([, metric]) => metric === "http_requests",
    );
    expect(requestsCall?.[2].path).toBeUndefined();
  });

  it("shows the store-unavailable state on the request sections when a host/path filter hits no log store (w5/m58)", () => {
    // A host/path-filtered read against a deployment with no Loki returns
    // storeUnavailable; the requests + latency sections must show the explicit
    // "needs the log store" copy, never a silently-unfiltered chart. Bandwidth
    // (never host/path-filtered) is unaffected.
    mockUseMetrics.mockImplementation((_resource, metric) =>
      metric === "bandwidth"
        ? emptyResult()
        : { ...emptyResult(), storeUnavailable: true },
    );

    renderCard();

    expect(
      screen.getAllByText("Host and Path filters need the log store").length,
    ).toBeGreaterThanOrEqual(2);
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
          degradedSources: [],
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

  it("titles Total Requests with the window's aggregate count (w5/m42)", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "http_requests") {
        return {
          series: [
            {
              unit: "count",
              labels: {},
              points: [
                { timestamp: "2026-07-06T09:00:00Z", value: 7000 },
                { timestamp: "2026-07-06T09:30:00Z", value: 266 },
              ],
            },
          ],
          loading: false,
          unavailable: false,
          degradedSources: [],
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard();

    expect(screen.getByText("7,266 requests")).toBeInTheDocument();
  });

  it("omits the aggregate count when the window has no request data", () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();

    // (the Group-by trigger's "All requests" label is not a count)
    expect(screen.queryByText(/\d requests/)).not.toBeInTheDocument();
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
          degradedSources: [],
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
      degradedSources: [],
      loading: false,
      error: undefined,
    });

    renderCard();

    expect(screen.getByText(/used this month/)).toBeInTheDocument();
  });

  // The w1/m50 three-state contract: data with a degradation annotation,
  // a real query error, and (elsewhere above) the healthy empty window —
  // never a gate failure masquerading as "No data in range".
  it("annotates the bandwidth chart when a source is degraded, still rendering data", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "bandwidth") {
        return {
          series: [
            {
              unit: "bytes",
              labels: { degraded_sources: "direct" },
              points: [{ timestamp: "2026-07-18T00:00:00Z", value: 42 }],
            },
          ],
          loading: false,
          unavailable: false,
          degradedSources: ["direct"],
          error: undefined,
        };
      }
      return emptyResult();
    });

    renderCard();

    const badge = screen.getByText("Partial data");
    expect(badge).toBeInTheDocument();
    expect(badge.closest("[title]")?.getAttribute("title")).toContain("direct");
    // The chart renders (no bandwidth error state, no unavailable state).
    expect(
      screen.queryByText(/Couldn't load bandwidth/),
    ).not.toBeInTheDocument();
  });

  it("renders a distinct error state for a failed bandwidth query — not 'No data in range'", () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "bandwidth") {
        return {
          ...emptyResult(),
          error: new Error("boom"),
        };
      }
      return emptyResult();
    });

    renderCard();

    expect(screen.getByText(/Couldn't load bandwidth/)).toBeInTheDocument();
    // Only the two healthy empty charts (requests handles empties itself) say so.
    expect(screen.queryAllByText("No data in range").length).toBeLessThan(3);
  });

  it("marks the month-to-date figure when its window was degraded", () => {
    mockUseMetrics.mockReturnValue(emptyResult());
    mockUseMonthToDateBandwidth.mockReturnValue({
      egressBandwidthMB: 512,
      degradedSources: ["http"],
      loading: false,
      error: undefined,
    });

    renderCard();

    const footer = screen.getByText(/used this month/);
    expect(footer.textContent).toContain("*");
    expect(footer.getAttribute("title")).toContain("http");
  });
});
