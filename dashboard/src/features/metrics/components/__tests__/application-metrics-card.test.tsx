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
import { useServiceInstances } from "@/features/services/hooks/use-service-instances";

vi.mock("@/features/metrics/hooks/use-metrics", () => ({
  useMetrics: vi.fn(),
}));
vi.mock("@/features/metrics/hooks/use-metrics-filter-values", () => ({
  useMetricsFilterValues: vi.fn(() => []),
}));
vi.mock("@/features/services/hooks/use-service-instances", () => ({
  useServiceInstances: vi.fn(() => ({
    instances: [],
    loading: false,
    refetch: vi.fn(),
  })),
}));

const mockUseMetrics = vi.mocked(useMetrics);
const mockFilterValues = vi.mocked(useMetricsFilterValues);
const mockUseServiceInstances = vi.mocked(useServiceInstances);

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
    throttled: false,
    error: undefined,
    degradedSources: [],
  };
}

/** A stepped series with real timestamps (the chart maps x by time). */
function seriesResult(unit: string, values: number[], instance?: string) {
  const labels: Record<string, string> = instance ? { INSTANCE: instance } : {};
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
    throttled: false,
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
      labels: { INSTANCE: instance },
      points: values.map((value, i) => ({
        timestamp: new Date(1_751_800_000_000 + i * 60_000).toISOString(),
        value,
      })),
    })),
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    throttled: false,
    error: undefined,
    degradedSources: [],
  };
}

/** Toggles one instance in the card's checkbox menu, then closes the menu. */
async function toggleInstance(label: string) {
  await userEvent.click(
    await screen.findByRole("button", { name: /^Instances/ }),
  );
  await userEvent.click(
    await screen.findByRole("menuitemcheckbox", {
      name: new RegExp(`^${label}`),
    }),
  );
  await userEvent.keyboard("{Escape}");
}

/** The instances the menu shows as checked. */
async function checkedInstances() {
  await userEvent.click(
    await screen.findByRole("button", { name: /^Instances/ }),
  );
  const checked = (await screen.findAllByRole("menuitemcheckbox"))
    .filter((item) => item.getAttribute("aria-checked") === "true")
    .map((item) => item.textContent);
  await userEvent.keyboard("{Escape}");
  return checked;
}

describe("ApplicationMetricsCard", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockFilterValues.mockReset();
    mockFilterValues.mockReturnValue([]);
    mockUseServiceInstances.mockReset();
    mockUseServiceInstances.mockReturnValue({
      instances: [],
      loading: false,
      refetch: vi.fn(),
    });
  });

  // The card's Limit / Manage-scaling header links (w5/m42) need a router
  // around the render — the service-detail-header test's harness pattern.
  function renderCard(
    resource = "app",
    { runsInstances = true, scalable = true, shellEligible = false } = {},
  ) {
    const rootRoute = createRootRoute();
    const indexRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => (
        <ApplicationMetricsCard
          resource={resource}
          window={WINDOW}
          runsInstances={runsInstances}
          scalable={scalable}
          shellEligible={shellEligible}
        />
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

    // Inventory shares the absolute CPU query when unfiltered (w4/m100) —
    // the dedicated inventory hook is skipped.
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu", {
      ...WINDOW,
      skip: true,
    });
    // Absolute usage (Total tab + percentage-unavailable witness).
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu", WINDOW);
    expect(mockUseMetrics).toHaveBeenCalledWith(
      "beancount-cms",
      "memory",
      WINDOW,
    );
    // Server-side percentages (w5/m90) — only while the Percentage tab is on.
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "cpu", {
      ...WINDOW,
      percentage: true,
      skip: false,
    });
    expect(mockUseMetrics).toHaveBeenCalledWith("beancount-cms", "memory", {
      ...WINDOW,
      percentage: true,
      skip: false,
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

  // w4/446: on a narrow header the instance filter wraps onto its own row, so
  // while its choices load the card holds that box instead of growing late.
  it("holds the instance filter's box while the inventory read loads", async () => {
    mockUseMetrics.mockReturnValue({ ...emptyResult(), loading: true });
    const loading = renderCard();
    await screen.findByRole("tab", { name: "Total" });
    expect(
      loading.container.querySelector("[data-instance-filter-slot]"),
    ).toHaveClass("h-9", "min-w-[9rem]", "max-lg:h-11");
    loading.unmount();

    mockUseMetrics.mockReturnValue(multiSeriesResult("cpu", { "web-a": [1] }));
    const ready = renderCard();
    await screen.findByRole("button", { name: /^Instances/ });
    expect(
      ready.container.querySelector("[data-instance-filter-slot]"),
    ).toBeNull();
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

  it("keeps a lightly used service's CPU headline positive (w4/296)", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "cpu" && !opts?.percentage)
        return seriesResult(
          "cpu",
          [0.00007821397872822744, 0.0002505198684260272],
        );
      return emptyResult();
    });

    renderCard();
    await selectTotalTab();

    expect(screen.getByText("0.000251 cores")).toBeInTheDocument();
    expect(screen.queryByText("0.000 cores")).not.toBeInTheDocument();
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
      "/services/beancount-cms/compute",
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

  it("links Total Instances to the Compute page's Manual Scaling section (w5/m42, w6/m483)", async () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard("beancount-cms");

    expect(
      await screen.findByRole("link", { name: "Manage scaling" }),
    ).toHaveAttribute("href", "/services/beancount-cms/compute#manual-scaling");
  });

  // w4/567: a cron job has no replica count and no Scaling page to link to.
  it("offers no scaling link for a service that cannot scale", async () => {
    mockUseMetrics.mockReturnValue(emptyResult());

    renderCard("nightly", { scalable: false });

    expect(await screen.findByText("Total Instances")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Manage scaling" }),
    ).not.toBeInTheDocument();
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

  it("never calls a sleeping service limitless: its empty limit read is no pods, not no limit", async () => {
    // The same reads as the no-limit case above, from a service parked at zero
    // instances: the current-limit read is empty because no pod runs (w5/m125).
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "cpu" && opts?.percentage) return emptyResult();
      if (metric === "cpu") return seriesResult("cpu", [0.5]);
      return emptyResult();
    });

    renderCard("app", { runsInstances: false });

    expect(
      (await screen.findAllByText(/Percentages unavailable/)).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText(/No limit configured/)).not.toBeInTheDocument();
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

    renderCard("app", { runsInstances: false });

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
          throttled: false,
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
          throttled: false,
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
      const instances = (opts as { instances?: string[] } | undefined)
        ?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    renderCard();
    await toggleInstance("web-a");
    // Discovery gap: live list empties and the inventory goes quiet. Force a
    // re-render via the Total tab so the card recomputes choices while the
    // explicit selection must survive.
    mockFilterValues.mockReturnValue([]);
    mockUseMetrics.mockImplementation(() => emptyResult());
    await userEvent.click(await screen.findByRole("tab", { name: "Total" }));
    // Still checked, and marked as out of this window rather than dropped.
    expect(await checkedInstances()).toEqual(["web-a (unavailable)"]);
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
      const instances = (opts as { instances?: string[] } | undefined)
        ?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    renderCard();
    await toggleInstance("web-a");
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
      const instances = (opts as { instances?: string[] } | undefined)
        ?.instances;
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
          <ApplicationMetricsCard
            key={res}
            resource={res}
            window={WINDOW}
            runsInstances
            scalable
          />
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
    await toggleInstance("web-a");
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
      const instances = (opts as { instances?: string[] } | undefined)
        ?.instances;
      if (metric === "cpu" && !instances)
        return seriesResult("cpu", [0.5], "web-a");
      return emptyResult();
    });
    renderCard();
    await toggleInstance("web-a");
    // The choice leaves the window: force a re-render with quiet discovery.
    mockUseMetrics.mockImplementation(() => emptyResult());
    mockFilterValues.mockReturnValue([]);
    await userEvent.click(await screen.findByRole("tab", { name: "Total" }));
    expect(
      await screen.findByRole("button", { name: "Show all" }),
    ).toBeInTheDocument();
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
      labels: { INSTANCE: instance },
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

// w4/258: Raw CPU said 0.1% — a retired instance's last point, because the
// header read series[0] — while the running instance sat near 100%.
describe("ApplicationMetricsCard usage header across instances (w4/258)", () => {
  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockFilterValues.mockReset();
    mockFilterValues.mockReturnValue([]);
  });

  function renderCard() {
    const rootRoute = createRootRoute();
    const indexRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => (
        <ApplicationMetricsCard
          resource="app"
          window={WINDOW}
          runsInstances
          scalable
        />
      ),
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([indexRoute]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
      context: { client: {} as never, session: null },
    });
    return render(<RouterProvider router={router} />);
  }

  /** The bold scalars beside section titles — not axis ticks or legends. */
  function headerScalars(container: HTMLElement): string[] {
    return [
      ...container.querySelectorAll("span.text-sm.font-semibold.tabular-nums"),
    ].map((el) => el.textContent ?? "");
  }

  const OLD = "web-old";
  const NEW = "web-new";
  function cpu(order: "old-first" | "new-first", unit: string) {
    const old = { [OLD]: [0.13470908047852115] };
    const fresh = { [NEW]: [92.55276727078095, 100.04089244388521] };
    return multiSeriesResult(
      unit,
      order === "old-first" ? { ...old, ...fresh } : { ...fresh, ...old },
    );
  }

  for (const order of ["old-first", "new-first"] as const) {
    it(`omits the Raw scalar for several instances, keeping their lines (${order})`, async () => {
      mockUseMetrics.mockImplementation((_resource, metric, opts) => {
        if (metric === "cpu")
          return cpu(order, opts?.percentage ? "percentage" : "cpu");
        return emptyResult();
      });
      const { container } = renderCard();

      expect(await screen.findAllByText(NEW)).not.toHaveLength(0);
      expect(screen.getAllByText(OLD)).not.toHaveLength(0);
      expect(headerScalars(container)).toEqual([]);

      await userEvent.click(screen.getByRole("tab", { name: "Total" }));
      expect(screen.getAllByText(NEW)).not.toHaveLength(0);
      expect(headerScalars(container)).toEqual([]);
    });
  }

  it("shows the one series that has points, ignoring empty siblings", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric !== "cpu" || !opts?.percentage) return emptyResult();
      return multiSeriesResult("percentage", {
        [OLD]: [],
        [NEW]: [92.5, 99.9],
      });
    });
    const { container } = renderCard();
    await screen.findAllByText(NEW);
    expect(headerScalars(container)).toEqual(["99.9%"]);
  });

  it("keeps the scalar for an aggregate's single series", async () => {
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric !== "cpu" || !opts?.percentage) return emptyResult();
      return opts.aggregateMethod
        ? seriesResult("percentage", [99.99772292754182])
        : cpu("old-first", "percentage");
    });
    const { container } = renderCard();
    await screen.findAllByText(NEW);
    expect(headerScalars(container)).toEqual([]);

    await userEvent.click(screen.getByRole("tab", { name: "Max" }));
    expect(headerScalars(container)).toEqual(["100.0%"]);
  });

  it("returns the scalar when a filter narrows the result to one instance, historical included", async () => {
    mockFilterValues.mockReturnValue([OLD, NEW]);
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric !== "cpu" || !opts?.percentage) return emptyResult();
      const instances = (opts as { instances?: string[] }).instances;
      if (instances?.length === 1) {
        const all = cpu("old-first", "percentage");
        return {
          ...all,
          series: all.series.filter((s) => s.labels.INSTANCE === instances[0]),
        };
      }
      return cpu("old-first", "percentage");
    });
    const { container } = renderCard();
    await screen.findAllByText(NEW);
    expect(headerScalars(container)).toEqual([]);

    await toggleInstance(OLD);
    expect(headerScalars(container)).toEqual(["0.1%"]);
    await toggleInstance(OLD);
    await toggleInstance(NEW);
    expect(headerScalars(container)).toEqual(["100.0%"]);
  });
});

// w4/717: Render's service metrics action menu on Memory and CPU charts opens a
// dashboard shell on a running instance, disables SSH with the log row's reason
// for a stopped instance, and omits SSH when the service's Shell cannot open.
describe("ApplicationMetricsCard instance shell actions (w4/717)", () => {
  const RUNNING = "srv-web-0123456789abcdefghij";
  const STOPPED = "srv-web-9999999999abcdefghij";

  beforeEach(() => {
    mockUseMetrics.mockReset();
    mockFilterValues.mockReset();
    mockFilterValues.mockReturnValue([]);
    mockUseServiceInstances.mockReset();
    mockUseServiceInstances.mockReturnValue({
      instances: [{ id: RUNNING, createdAt: "2026-11-23T00:00:00Z" }],
      loading: false,
      refetch: vi.fn(),
    });
  });

  function renderCard({ shellEligible = true } = {}) {
    const rootRoute = createRootRoute();
    const indexRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => (
        <ApplicationMetricsCard
          resource="srv-web"
          window={WINDOW}
          runsInstances
          scalable
          shellEligible={shellEligible}
        />
      ),
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([indexRoute]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
      context: { client: {} as never, session: null },
    });
    return render(<RouterProvider router={router} />);
  }

  it("links SSH into instance on a running instance and disables it with Render's reason on a stopped one", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "cpu" && opts?.percentage) {
        return multiSeriesResult("percentage", {
          [RUNNING]: [45, 52],
          [STOPPED]: [12, 18],
        });
      }
      if (metric === "cpu" && !opts?.percentage) {
        return multiSeriesResult("cpu", {
          [RUNNING]: [0.2, 0.3],
          [STOPPED]: [0.1, 0.1],
        });
      }
      return emptyResult();
    });

    renderCard({ shellEligible: true });

    // Running instance: clicking its legend trigger opens the menu with an
    // active Shell link targeting that instance.
    const runningTrigger = await screen.findByRole("button", {
      name: "Actions for instance srv-web-01234",
    });
    expect(mockUseServiceInstances).toHaveBeenCalledWith("srv-web", {
      skip: false,
    });
    await user.click(runningTrigger);
    const sshLink = screen.getByRole("menuitem", {
      name: "SSH into instance",
    });
    expect(sshLink).toHaveAttribute(
      "href",
      `/services/srv-web/shell?instance=${RUNNING}`,
    );
    expect(sshLink).toHaveAttribute("target", "_blank");
    const logsLink = screen.getByRole("menuitem", {
      name: "View instance logs",
    });
    expect(logsLink).toHaveAttribute(
      "href",
      `/services/srv-web/logs?instance=${RUNNING}`,
    );
    expect(logsLink).toHaveAttribute("target", "_blank");

    // Copy instance ID writes the full canonical ID (not the shortened slug).
    await user.click(
      screen.getByRole("menuitem", { name: "Copy instance ID" }),
    );
    expect(await navigator.clipboard.readText()).toBe(RUNNING);

    // Stopped instance: SSH into instance is disabled with no href and carries
    // the log row's exact reason.
    const stoppedTrigger = screen.getByRole("button", {
      name: "Actions for instance srv-web-99999",
    });
    await user.click(stoppedTrigger);
    const disabledSsh = screen.getByRole("menuitem", {
      name: "SSH into instance",
    });
    expect(disabledSsh).toHaveAttribute("aria-disabled", "true");
    expect(disabledSsh).not.toHaveAttribute("href");
    expect(disabledSsh).toHaveAccessibleDescription(
      "You can only SSH into running instances. This one is not available.",
    );
    await user.keyboard("{Escape}");
  });

  it("opens Render's floating action card when clicking a charted series, and filters or resets from it", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "memory" && opts?.percentage) {
        return multiSeriesResult("percentage", {
          [RUNNING]: [40, 42],
          [STOPPED]: [10, 12],
        });
      }
      if (metric === "cpu" && !opts?.percentage) {
        return multiSeriesResult("cpu", {
          [RUNNING]: [0.2],
          [STOPPED]: [0.1],
        });
      }
      return emptyResult();
    });

    renderCard({ shellEligible: true });
    const charts = await screen.findAllByRole("img", {
      name: /2 data points/,
    });
    // Clicking the Memory chart opens the floating action card for the nearest
    // series (RUNNING first in jsdom's zero-rect fallback).
    await user.click(charts[0]);
    const sshLink = screen.getByRole("link", { name: "SSH into instance" });
    expect(sshLink).toHaveAttribute(
      "href",
      `/services/srv-web/shell?instance=${RUNNING}`,
    );
    expect(sshLink).toHaveAttribute("target", "_blank");

    // Filter to instance narrows the card, and reopening shows Reset instance filter.
    await user.click(
      screen.getByRole("button", { name: "Filter to instance" }),
    );
    expect(
      screen.getByRole("button", { name: "Show all" }),
    ).toBeInTheDocument();

    await user.click(charts[0]);
    expect(
      screen.getByRole("button", { name: "Reset instance filter" }),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Reset instance filter" }),
    );
    expect(
      screen.queryByRole("button", { name: "Show all" }),
    ).not.toBeInTheDocument();
  });

  it("omits SSH into instance and skips instance polling when the service's Shell cannot open", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockImplementation((_resource, metric, opts) => {
      if (metric === "cpu" && opts?.percentage) {
        return multiSeriesResult("percentage", {
          [RUNNING]: [45, 52],
          [STOPPED]: [12, 18],
        });
      }
      if (metric === "cpu" && !opts?.percentage) {
        return multiSeriesResult("cpu", {
          [RUNNING]: [0.2, 0.3],
          [STOPPED]: [0.1, 0.1],
        });
      }
      return emptyResult();
    });

    renderCard({ shellEligible: false });

    const trigger = await screen.findByRole("button", {
      name: "Actions for instance srv-web-01234",
    });
    expect(mockUseServiceInstances).toHaveBeenCalledWith("srv-web", {
      skip: true,
    });
    await user.click(trigger);
    expect(
      screen.queryByRole("menuitem", { name: "SSH into instance" }),
    ).toBeNull();
    expect(
      screen.getByRole("menuitem", { name: "View instance logs" }),
    ).toHaveAttribute("href", `/services/srv-web/logs?instance=${RUNNING}`);
  });

  it("offers no instance action menu on Total Instances (Memory and CPU only, as on Render)", async () => {
    const user = userEvent.setup();
    mockUseMetrics.mockImplementation((_resource, metric) => {
      if (metric === "instance_count") {
        return seriesResult("count", [2, 2], RUNNING);
      }
      return emptyResult();
    });

    renderCard({ shellEligible: true });
    const chart = await screen.findByRole("img", { name: /2 data points/ });
    await user.click(chart);
    expect(
      screen.queryByRole("link", { name: "SSH into instance" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy instance ID" }),
    ).not.toBeInTheDocument();
  });
});
