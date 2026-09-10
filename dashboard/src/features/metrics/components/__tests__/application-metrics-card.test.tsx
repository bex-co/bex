import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ApplicationMetricsCard } from "../application-metrics-card";
import { summarizeLimits } from "@/features/metrics/lib/limit-summary";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";
import { useMetricsFilterValues } from "@/features/metrics/hooks/use-metrics-filter-values";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-metrics-filter-values", () => ({
  useMetricsFilterValues: vi.fn(() => []),
}));

const mockUseMetrics = vi.mocked(useMetrics);
const mockFilterValues = vi.mocked(useMetricsFilterValues);

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

/** A stepped series with real timestamps (the chart maps x by time). */
function seriesResult(unit: string, values: number[], instance?: string) {
  const labels: Record<string, string> = instance ? { instance } : {};
  return {
    series: [
      {
        unit,
        labels,
        points: values.map((value, i) => ({
          timestamp: new Date(1_751_800_000_000 + i * 60_000).toISOString(),
          value,
        })),
      },
    ],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    error: undefined,
    degradedSources: [],
  };
}

function multiSeriesResult(
  unit: string,
  perInstance: Record<string, number[]>,
) {
  return {
    series: Object.entries(perInstance).map(([instance, values]) => ({
      unit,
      labels: { instance },
      points: values.map((value, i) => ({
        timestamp: new Date(1_751_800_000_000 + i * 60_000).toISOString(),
        value,
      })),
    })),
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    error: undefined,
    degradedSources: [],
  };
}

describe("ApplicationMetricsCard", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockFilterValues.mockReset();
    mockFilterValues.mockReturnValue([]);
  });

  // The card's Limit / Manage-scaling header links (w5/m42) need a router
  // around the render — the service-detail-header test's harness pattern.
  function renderCard(resource = "app") {
    const rootRoute = createRootRoute();
    const indexRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => (
        <ApplicationMetricsCard resource={resource} window={WINDOW} />
      ),
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([indexRoute]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
      context: { client: {} as never, session: null },
    });
    return render(<RouterProvider router={router} />);
  }

  /** The card boots on Percentage (Render's default); switch via its own tabs. */
  async function selectTotalTab() {
    await userEvent.click(await screen.findByRole("tab", { name: "Total" }));
  }

  it("fetches absolute + percentage cpu/memory over the shared window; per-instance _limit queries ride the same window unaggregated", async () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard("beancount-cms");
    await screen.findByText("Application Metrics");

    // Absolute usage (Total tab + percentage-unavailable witness).
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "cpu",
      WINDOW,
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory",
      WINDOW,
    );
    // Server-side percentages (w5/m90) — never divided client-side.
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu", {
      ...WINDOW,
      percentage: true,
    });
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "memory", {
      ...WINDOW,
      percentage: true,
    });
    // Limits are per-instance (no aggregateMax collapse) over the same window.
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu_limit", {
      ...WINDOW,
    });
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory_limit",
      { ...WINDOW },
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "instance_count",
      WINDOW,
    );
  });

  it("owns the Percentage/Total tabs in its header, defaulting to Percentage (w5/m42)", async () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard();

    expect(
      await screen.findByRole("tab", { name: "Percentage" }),
    ).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Total" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
  });

  it("draws a multi-point history chart per metric on the Total tab, with the latest value in the header", async () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") return seriesResult("bytes", [100, 200]);
      if (metric === "instance_count") return seriesResult("count", [3, 3]);
      return emptyResult();
    });

    renderCard();
    await selectTotalTab();

    // Memory renders a 2-point line chart, not a lone stat…
    expect(
      screen.getAllByRole("img", { name: /2 data points/ }).length,
    ).toBeGreaterThanOrEqual(2); // memory + instances
    // …and its header shows the LAST point (200 bytes), not the first —
    // getAllBy: the y-axis max tick legitimately shows the same value.
    expect(screen.getAllByText("200 B").length).toBeGreaterThan(0);
    expect(screen.getAllByText("3").length).toBeGreaterThan(0);
  });

  it("renders server percentages as-is, never dividing by the _limit series (w5/m90)", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      // Absolute usage (bytes) and server percentages (already 0..100).
      if (metric === "memory" && opts?.percentage)
        return seriesResult("percentage", [80, 50]);
      if (metric === "memory") return seriesResult("bytes", [100, 200]);
      if (metric === "memory_limit") return seriesResult("bytes", [200]);
      return emptyResult();
    });

    renderCard("beancount-cms");

    // Latest server percentage (50) renders directly — NOT 100/200 = 50% via
    // client division (indistinguishable here by value, but the _limit value
    // must not leak into the math: change the mock limit and it still holds).
    expect(await screen.findByText("50.0%")).toBeInTheDocument();
    expect(screen.getByText("200 B")).toBeInTheDocument();
    const limitLinks = screen.getAllByRole("link", { name: "Limit" });
    expect(limitLinks.length).toBe(2); // memory + cpu sections
    expect(limitLinks[0]).toHaveAttribute(
      "href",
      "/services/beancount-cms/plan",
    );
  });

  it("shows one uniform limit value, or 'Limits vary' for mixed per-replica limits", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "memory" && opts?.percentage)
        return seriesResult("percentage", [80, 50]);
      if (metric === "memory") return seriesResult("bytes", [100, 200]);
      // Two replicas on different limits: no single applicable value.
      if (metric === "memory_limit")
        return multiSeriesResult("bytes", {
          "web-a": [536870912],
          "web-b": [1073741824],
        });
      return emptyResult();
    });

    renderCard();

    expect(await screen.findByText("Limits vary")).toBeInTheDocument();
    expect(await screen.findByText("50.0%")).toBeInTheDocument();
  });

  it("links Total Instances to the scaling tab (w5/m42)", async () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard("beancount-cms");

    expect(
      await screen.findByRole("link", { name: "Manage scaling" }),
    ).toHaveAttribute("href", "/services/beancount-cms/scaling");
  });

  it("shows the autoscale-target label alongside the limit in percentage mode (w3/m10)", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "memory" && opts?.percentage)
        return seriesResult("percentage", [25, 50]);
      if (metric === "memory") return seriesResult("bytes", [50, 100]);
      if (metric === "memory_limit") return seriesResult("bytes", [200]);
      if (metric === "memory_target") return seriesResult("percentage", [70]);
      return emptyResult();
    });

    renderCard();

    expect(await screen.findByText("Target 70.0%")).toBeInTheDocument();
    expect(screen.getByText("200 B")).toBeInTheDocument();
  });

  it("fetches cpu_target/memory_target over the shared window (w3/m10)", async () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard("beancount-cms");
    await screen.findByText("Application Metrics");

    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "cpu_target",
      WINDOW,
    );
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory_target",
      WINDOW,
    );
  });

  it("omits the target label when autoscaling is disabled/unconfigured (no fake value)", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "memory" && opts?.percentage)
        return seriesResult("percentage", [25, 50]);
      if (metric === "memory") return seriesResult("bytes", [50, 100]);
      if (metric === "memory_limit") return seriesResult("bytes", [200]);
      return emptyResult(); // memory_target: server omits it entirely
    });

    renderCard();

    expect(await screen.findByText("50.0%")).toBeInTheDocument();
    expect(screen.queryByText(/^Target/)).not.toBeInTheDocument();
    expect(screen.getByText("200 B")).toBeInTheDocument();
  });

  it("shows the honest no-limit state for percentage when no _limit series exists", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      // No limits configured AND no percentage survived (zero denominators):
      // the absolute read witnesses observed usage, the percentage read is
      // empty, the _limit reads are empty.
      if (metric === "cpu" && opts?.percentage) return emptyResult();
      if (metric === "cpu") return seriesResult("cpu", [0.5]);
      return emptyResult();
    });

    renderCard();

    expect(
      (await screen.findAllByText(/No limit configured/)).length,
    ).toBeGreaterThan(0);
  });

  it("renders retained percentages when the current-limit read is empty (suspended service)", async () => {
    // bex-api's cpu_limit is a current-pod read (empty with 0 pods), while
    // the percentage read keeps own-limit history — the card must not hide
    // usable retained points behind "No limit configured" (w5/m90 t008 live
    // walkthrough on a suspended mixed-limit fixture).
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "cpu" && opts?.percentage)
        return seriesResult("percentage", [90.6, 64.4]);
      if (metric === "cpu") return seriesResult("cpu", [0.45, 0.64]);
      return emptyResult(); // _limit/_target reads: no live pods
    });

    renderCard();

    expect(await screen.findByText("64.4%")).toBeInTheDocument();
    expect(screen.queryByText(/No limit configured/)).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Percentages unavailable/),
    ).not.toBeInTheDocument();
  });

  it("shows the unavailable state when usage exists but no percentage survived (untrustworthy limits)", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      // Usage observed (absolute), but bex-api omitted every percentage
      // point (deleted pods / predated limit history) while limits exist.
      if (metric === "cpu" && opts?.percentage) return emptyResult();
      if (metric === "cpu") return seriesResult("cpu", [0.5]);
      if (metric === "cpu_limit") return seriesResult("cpu", [1]);
      return emptyResult();
    });

    renderCard();

    expect(
      (await screen.findAllByText(/Percentages unavailable/)).length,
    ).toBeGreaterThan(0);
    // The Total tab still charts the absolute usage — the witness that
    // distinguishes "unavailable percentages" from "no usage". Memory and
    // Total Instances are genuinely empty here, so they keep "No data".
    await selectTotalTab();
    expect(screen.getAllByText("No data in range")).toHaveLength(2);
  });

  it("draws one line per instance and a legend naming each", async () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "memory") {
        return {
          series: [
            seriesResult("bytes", [1, 2], "web-a").series[0],
            seriesResult("bytes", [3, 4], "web-b").series[0],
          ],
          loading: false,
          unavailable: false,
          storeUnavailable: false,
          error: undefined,
          degradedSources: [],
        };
      }
      return emptyResult();
    });

    renderCard();
    await selectTotalTab();

    expect(screen.getByText("web-a")).toBeInTheDocument();
    expect(screen.getByText("web-b")).toBeInTheDocument();
  });

  it("renders the unavailable state instead of a chart when a metric has no source", async () => {
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "cpu") {
        return {
          series: [],
          loading: false,
          unavailable: true,
          storeUnavailable: false,
          error: undefined,
          degradedSources: [],
        };
      }
      return emptyResult();
    });

    renderCard();
    await selectTotalTab();

    expect(
      screen.getByText("Metrics source not configured"),
    ).toBeInTheDocument();
    // Memory and Total Instances still render their (empty) charts, not unavailable.
    expect(screen.getAllByText("No data in range")).toHaveLength(2);
  });

  it("retains the selection across a discovery gap instead of broadening to all (w5/m91)", async () => {
    mockFilterValues.mockReturnValue(["web-a"]);
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      const instances = (
        opts as { instances?: string[] } | undefined
      )?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    renderCard();
    const select = (await screen.findByLabelText("Instances")) as HTMLElement;
    await userEvent.selectOptions(select, ["web-a"]);
    // Discovery gap: live list empties and the inventory goes quiet. Force a
    // re-render via the Total tab so the card recomputes choices while the
    // explicit selection must survive.
    mockFilterValues.mockReturnValue([]);
    mockUseMetrics.mockImplementation(() => emptyResult());
    await userEvent.click(await screen.findByRole("tab", { name: "Total" }));
    const reselected = (await screen.findByLabelText(
      "Instances",
    )) as unknown as HTMLSelectElement;
    expect(Array.from(reselected.selectedOptions).map((o) => o.value)).toEqual(
      ["web-a"],
    );
    expect(
      await screen.findByRole("button", { name: "Show all" }),
    ).toBeInTheDocument();
    // Filtered reads still carry the retained id — never broadened to all.
    expect(mockUseMetrics).toHaveBeenCalledWith(
      expect.anything(),
      "cpu",
      expect.objectContaining({ instances: ["web-a"] }),
    );
  });

  it("keeps a selected historical instance and offers an explicit return to all (w5/m91)", async () => {
    mockFilterValues.mockReturnValue(["web-a", "web-b"]);
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      const instances = (
        opts as { instances?: string[] } | undefined
      )?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    renderCard();
    const select = (await screen.findByLabelText("Instances")) as HTMLElement;
    await userEvent.selectOptions(select, ["web-a"]);
    // Filtered usage reads carry the retained id even though the window has
    // no samples for it.
    expect(mockUseMetrics).toHaveBeenCalledWith(
      expect.anything(),
      "cpu",
      expect.objectContaining({ instances: ["web-a"] }),
    );
    // Explicit return-to-all clears the filter (no silent broadening before).
    await userEvent.click(
      await screen.findByRole("button", { name: "Show all" }),
    );
    expect(mockUseMetrics).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.anything(),
      expect.not.objectContaining({ instances: ["web-a"] }),
    );
  });

  it("resets the selection on resource navigation so choices never cross services (w5/m91)", async () => {
    mockFilterValues.mockReturnValue(["web-a"]);
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      const instances = (
        opts as { instances?: string[] } | undefined
      )?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    function Harness() {
      const [res, setRes] = useState("app1");
      return (
        <>
          <button type="button" onClick={() => setRes("app2")}>
            switch-service
          </button>
          {/* key={resource} mirrors the route: navigation remounts the card */}
          <ApplicationMetricsCard key={res} resource={res} window={WINDOW} />
        </>
      );
    }
    const rootRoute = createRootRoute();
    const indexRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: Harness,
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([indexRoute]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
      context: { client: {} as never, session: null },
    });
    render(<RouterProvider router={router} />);
    await userEvent.selectOptions(
      (await screen.findByLabelText("Instances")) as HTMLElement,
      ["web-a"],
    );
    expect(
      await screen.findByRole("button", { name: "Show all" }),
    ).toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", { name: "switch-service" }),
    );
    expect(screen.queryByRole("button", { name: "Show all" })).toBeNull();
    expect(mockUseMetrics).toHaveBeenLastCalledWith(
      "app2",
      expect.anything(),
      expect.not.objectContaining({ instances: ["web-a"] }),
    );
  });

  it("marks retained-but-unavailable choices and names the empty selection result (w5/m91)", async () => {
    mockFilterValues.mockReturnValue(["web-a"]);
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      const instances = (
        opts as { instances?: string[] } | undefined
      )?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    renderCard();
    const select = (await screen.findByLabelText("Instances")) as HTMLElement;
    await userEvent.selectOptions(select, ["web-a"]);
    // The choice leaves the window: force a re-render with quiet discovery.
    mockUseMetrics.mockImplementation(() => emptyResult());
    mockFilterValues.mockReturnValue([]);
    await userEvent.click(await screen.findByRole("tab", { name: "Total" }));
    expect(await screen.findByRole("button", { name: "Show all" }))
      .toBeInTheDocument();
    expect(
      await screen.findByText(/unavailable in this window/),
    ).toBeInTheDocument();
    expect(
      (await screen.findAllByText(/No data for the selected instances/)).length,
    ).toBeGreaterThan(0);
  });
});

describe("summarizeLimits", () => {
  function limitSeries(instance: string, value: number) {
    return {
      unit: "bytes",
      labels: { instance },
      points: [
        {
          timestamp: new Date(1_751_800_000_000).toISOString(),
          value,
        },
      ],
    };
  }

  it("reports none for no series", () => {
    expect(summarizeLimits([])).toEqual({ kind: "none" });
  });

  it("reports a single uniform value", () => {
    expect(
      summarizeLimits([limitSeries("a", 200), limitSeries("b", 200)]),
    ).toEqual({ kind: "single", value: 200 });
  });

  it("reports vary for mixed per-replica limits", () => {
    expect(
      summarizeLimits([limitSeries("a", 100), limitSeries("b", 200)]),
    ).toEqual({ kind: "vary" });
  });
});
