import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatastoreMetricsPanel } from "../datastore-metrics-panel";
import { DatastoreMetricsSkeleton } from "@/common/components/route-skeletons";
import { useDatastoreMetrics } from "@/features/metrics/hooks/use-datastore-metrics";
import { useMetricsRetentionBound } from "@/features/metrics/hooks/use-metrics-retention";
import {
  RANGE_PRESETS,
  type RangeBound,
} from "@/features/metrics/lib/range";

vi.mock("@/features/metrics/hooks/use-datastore-metrics", () => ({
  useDatastoreMetrics: vi.fn(),
}));
// The plan's window is bex-api's `metricsRetention` (Apollo); its own hook
// test covers the read, so the panel's tests set the bound it hands over.
vi.mock("@/features/metrics/hooks/use-metrics-retention", () => ({
  useMetricsRetentionBound: vi.fn(),
}));

const mockUseDatastoreMetrics = vi.mocked(useDatastoreMetrics);
const mockUseMetricsRetentionBound = vi.mocked(useMetricsRetentionBound);

// Radix Select leans on APIs jsdom doesn't implement.
beforeAll(() => {
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.releasePointerCapture ??= () => {};
  Element.prototype.scrollIntoView ??= () => {};
});

const HOUR_MS = 60 * 60 * 1000;

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
    throttled: false,
    error: undefined,
    degradedSources: [],
  };
}

/** One series per (instance, direction), as bex-api answers a two-series graph. */
function directionResult(
  unit: string,
  instances: string[],
): ReturnType<typeof useDatastoreMetrics> {
  return {
    ...emptyResult(),
    series: instances.flatMap((instance) =>
      // Write first: the panel orders read before write itself.
      ["write", "read"].map((direction) => ({
        unit,
        labels: { instance, direction },
        points: [
          { timestamp: new Date(1_751_800_000_000).toISOString(), value: 4 },
        ],
      })),
    ),
  };
}

function headings(container: HTMLElement) {
  return Array.from(container.querySelectorAll(".text-sm.font-medium")).map(
    (el) => el.textContent,
  );
}

function errorResult() {
  return { ...emptyResult(), error: new Error("boom") };
}

describe("DatastoreMetricsPanel", () => {
  beforeEach(() => {
    mockUseDatastoreMetrics.mockReset();
    mockUseMetricsRetentionBound.mockReset();
  });

  // w9/m86 / w5/045: a failed metrics query must render a distinct error card,
  // not the empty "No data in range" chart that hid the wrong-identifier bug.
  it("surfaces a distinct error card when a datastore metric query fails", () => {
    mockUseDatastoreMetrics.mockReturnValue(errorResult());
    render(<DatastoreMetricsPanel kind="database" resource="dpg-abc" />);
    expect(screen.getAllByRole("alert").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Couldn't load this metric").length,
    ).toBeGreaterThan(0);
  });

  it("fetches disk usage for both database and keyvalue kinds, connections/lag only for database", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    render(
      <DatastoreMetricsPanel
        kind="keyvalue"
        resource="sessions-cache"
        highAvailabilityEnabled={false}
      />,
    );

    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "disk",
      expect.not.objectContaining({ skip: true }),
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "disk_capacity",
      expect.not.objectContaining({ skip: true }),
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "db_connections",
      expect.objectContaining({ skip: true }),
    );
    // No Postgres replication-lag chart section for a keyvalue, and one
    // Active Connections chart, its own connected clients (db_connections is
    // skipped above), titled as Render titles both (w6/m459).
    expect(screen.getAllByText("Active Connections")).toHaveLength(1);
    expect(screen.queryByText("Replication Lag")).not.toBeInTheDocument();
    // But Memory (its used memory, w5/011) and the keyvalue-only connections
    // sections DO render, and their queries are not skipped. Its container
    // memory limit is no line for that chart (Valkey evicts below it).
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "memory",
      expect.objectContaining({ skip: false }),
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "memory_limit",
      expect.objectContaining({ skip: true }),
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "kv_connections",
      expect.objectContaining({ skip: false }),
    );
    expect(screen.getByText("Memory")).toBeInTheDocument();
    expect(screen.getByText("Active Connections")).toBeInTheDocument();
  });

  it("skips the keyvalue connections metric for a database resource", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "kv_connections",
      expect.objectContaining({ skip: true }),
    );
    // A database's Memory is its container's (w1/m180), not the Key Value
    // used-memory series, and the Key Value connections section stays out:
    // one Active Connections chart, its own.
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "memory",
      expect.objectContaining({ skip: false }),
    );
    expect(screen.getByText("Memory")).toBeInTheDocument();
    expect(screen.getAllByText("Active Connections")).toHaveLength(1);
  });

  it("charts CPU and memory against their limits for a datastore", () => {
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) => {
      if (metric === "cpu") return seriesResult("cpu", [0.1, 0.2]);
      if (metric === "cpu_limit") return seriesResult("cpu", [0.5]);
      if (metric === "memory") return seriesResult("bytes", [100, 200]);
      if (metric === "memory_limit") return seriesResult("bytes", [536870912]);
      return emptyResult();
    });

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    expect(screen.getByText("CPU")).toBeInTheDocument();
    expect(screen.getByText(/^Limit 0\.5/)).toBeInTheDocument();
    expect(screen.getByText(/^Limit 512/)).toBeInTheDocument();
  });

  it("asks no CPU or memory for a service's disk", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    render(<DatastoreMetricsPanel kind="service" resource="srv-x" />);

    for (const metric of ["cpu", "cpu_limit", "memory", "memory_limit"]) {
      expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
        "service",
        "srv-x",
        metric,
        expect.objectContaining({ skip: true }),
      );
    }
    expect(screen.queryByText("CPU")).not.toBeInTheDocument();
  });

  it("renders disk usage with a capacity reference label", () => {
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) => {
      if (metric === "disk") return seriesResult("bytes", [1024, 2048]);
      if (metric === "disk_capacity") return seriesResult("bytes", [10240]);
      return emptyResult();
    });

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    expect(screen.getByText("Disk")).toBeInTheDocument();
    expect(screen.getByText("Capacity 10 KiB")).toBeInTheDocument();
  });

  // w4/m91: the Capacity label reflects the datastore's logical/billed disk size
  // (bex-api now sources disk_capacity from StorageGB, not the physical PVC). A
  // near-full 1 GB Postgres shows "Capacity 1 GB" against ~810 MiB used, so its
  // fullness reads truthfully instead of ~8% of a fixed 10 GiB PVC floor.
  it("shows the logical/billed capacity, so a near-full 1 GB datastore reads near-full", () => {
    const gib = 1024 * 1024 * 1024;
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) => {
      if (metric === "disk") return seriesResult("bytes", [0.79 * gib]);
      if (metric === "disk_capacity") return seriesResult("bytes", [gib]);
      return emptyResult();
    });

    render(<DatastoreMetricsPanel kind="database" resource="dpg-abc" />);

    expect(screen.getByText("Capacity 1 GB")).toBeInTheDocument();
  });

  // Beside the Postgres autoscaling box at phone width the capacity label takes
  // a row of its own, so its slot is held while it loads.
  it("holds the capacity label's slot while it loads, and drops it when there is none", () => {
    let capacity: ReturnType<typeof useDatastoreMetrics> = {
      ...emptyResult(),
      loading: true,
    };
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) =>
      metric === "disk_capacity" ? capacity : emptyResult(),
    );
    const panel = () => (
      <DatastoreMetricsPanel kind="database" resource="dpg-abc" />
    );

    const { rerender } = render(panel());
    expect(screen.getByTestId("disk-capacity-pending")).toBeInTheDocument();
    expect(screen.queryByText(/^Capacity /)).not.toBeInTheDocument();

    capacity = seriesResult("bytes", [1024 * 1024 * 1024]);
    rerender(panel());
    expect(
      screen.queryByTestId("disk-capacity-pending"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Capacity 1 GB")).toBeInTheDocument();

    capacity = emptyResult();
    rerender(panel());
    expect(
      screen.queryByTestId("disk-capacity-pending"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/^Capacity /)).not.toBeInTheDocument();
  });

  it("renders the active-connections chart for a database resource", () => {
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) => {
      if (metric === "db_connections")
        return seriesResult("count", [3, 5], "pg-1");
      return emptyResult();
    });

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    expect(screen.getByText("Active Connections")).toBeInTheDocument();
  });

  it("shows a clear N/A state for replication lag before HA is enabled — never a fake chart", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    render(
      <DatastoreMetricsPanel
        kind="database"
        resource="pg"
        highAvailabilityEnabled={false}
      />,
    );

    expect(screen.getByText("Replication Lag")).toBeInTheDocument();
    expect(screen.getByText(/N\/A — no replica/)).toBeInTheDocument();
    // The lag query itself is skipped pre-HA — no point asking a Prometheus
    // metric that reports a fake 0 from a lone primary (datastore.go's gate).
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "replication_lag",
      expect.objectContaining({ skip: true }),
    );
  });

  it("renders a real replication-lag chart once HA is enabled", () => {
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) => {
      if (metric === "replication_lag") return seriesResult("seconds", [0.4]);
      return emptyResult();
    });

    render(
      <DatastoreMetricsPanel
        kind="database"
        resource="pg"
        highAvailabilityEnabled={true}
      />,
    );

    expect(screen.queryByText(/N\/A/)).not.toBeInTheDocument();
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "replication_lag",
      expect.objectContaining({ skip: false }),
    );
  });

  it("renders the unavailable state when Prometheus isn't wired for a live HA instance", () => {
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) => {
      if (metric === "replication_lag") {
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

    render(
      <DatastoreMetricsPanel
        kind="database"
        resource="pg"
        highAvailabilityEnabled={true}
      />,
    );

    expect(
      screen.getByText("Metrics source not configured"),
    ).toBeInTheDocument();
  });

  // w7/m430: Render's database Metrics page — disk storage, then database
  // activity, in Render's order.
  it("charts Render's five database-activity and disk graphs for a Postgres", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    const { container } = render(
      <DatastoreMetricsPanel kind="database" resource="pg" />,
    );

    expect(headings(container)).toEqual([
      "Disk",
      "Disk Activity",
      "Disk Operations",
      "CPU",
      "Memory",
      "Active Connections",
      "Network Activity",
      "Transaction Volume",
      "Replication Lag",
      "Lock-Delayed Queries",
    ]);
    for (const metric of [
      "network_activity",
      "transaction_volume",
      "lock_delayed_queries",
      "disk_operations",
    ]) {
      expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
        "database",
        "pg",
        metric,
        expect.objectContaining({ skip: false }),
      );
    }
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "disk_activity",
      expect.not.objectContaining({ skip: true }),
    );
  });

  it("gives a Key Value Disk Activity and none of Postgres's activity graphs", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    const { container } = render(
      <DatastoreMetricsPanel kind="keyvalue" resource="red-x" />,
    );

    expect(headings(container)).toEqual([
      "Disk",
      "Disk Activity",
      "CPU",
      "Memory",
      "Active Connections",
    ]);
    for (const metric of [
      "network_activity",
      "transaction_volume",
      "lock_delayed_queries",
      "disk_operations",
    ]) {
      expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
        "keyvalue",
        "red-x",
        metric,
        expect.objectContaining({ skip: true }),
      );
    }
  });

  it("gives a service's disk Disk Activity and Disk Operations beside its usage", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    const { container } = render(
      <DatastoreMetricsPanel kind="service" resource="srv-x" />,
    );

    expect(headings(container)).toEqual([
      "Disk",
      "Disk Activity",
      "Disk Operations",
    ]);
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "service",
      "srv-x",
      "disk_operations",
      expect.objectContaining({ skip: false }),
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "service",
      "srv-x",
      "network_activity",
      expect.objectContaining({ skip: true }),
    );
  });

  it("draws the two-series graphs as Read and Write, per instance once there are several", () => {
    let instances = ["pg-1"];
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) =>
      metric === "disk_activity"
        ? directionResult("bytes", instances)
        : emptyResult(),
    );
    const { rerender } = render(
      <DatastoreMetricsPanel kind="database" resource="pg" />,
    );
    // One instance: the legend names only the direction, read first.
    const legend = () =>
      Array.from(document.querySelectorAll(".mt-1 > span")).map(
        (el) => el.textContent,
      );
    expect(legend()).toEqual(["Read", "Write"]);

    instances = ["pg-2", "pg-1"];
    rerender(<DatastoreMetricsPanel kind="database" resource="pg" />);
    expect(legend()).toEqual([
      "pg-1 · Read",
      "pg-1 · Write",
      "pg-2 · Read",
      "pg-2 · Write",
    ]);
  });

  it("says Lock-Delayed Queries needs the log store, not that the read failed", () => {
    mockUseDatastoreMetrics.mockImplementation((_kind, _resource, metric) =>
      metric === "lock_delayed_queries"
        ? { ...emptyResult(), storeUnavailable: true }
        : emptyResult(),
    );

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    expect(
      screen.getByText("Lock-delayed queries need the log store"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Host and Path filters/)).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows an empty window as no data, for counts and bytes alike", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    // Every chart section without data says so; none is an error.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(
      screen.getAllByText("No data in range").length,
    ).toBeGreaterThanOrEqual(9);
  });

  // w7/072: the panel has the service Metrics page's range picker, and every
  // read (graphs, limits, capacity) covers the range it shows. It used to send
  // no range at all, so every chart was bex-api's default last hour.
  describe("time range", () => {
    /** Each metric's read window from its latest call: span, step and poll. */
    function readWindows() {
      const latest = new Map<string, unknown>();
      for (const [, , metric, opts] of mockUseDatastoreMetrics.mock.calls) {
        latest.set(metric, {
          span: Date.parse(opts!.endTime!) - Date.parse(opts!.startTime!),
          step: opts!.resolutionSeconds,
          poll: opts!.pollIntervalMs,
        });
      }
      return latest;
    }

    it("reads every metric over Render's 12-hour default, refreshed by the window's own tick", () => {
      mockUseDatastoreMetrics.mockReturnValue(emptyResult());
      render(<DatastoreMetricsPanel kind="database" resource="dpg-abc" />);

      expect(
        screen.getByRole("combobox", { name: "Time range" }),
      ).toHaveTextContent("Last 12 hours");
      const windows = readWindows();
      expect(windows.size).toBe(14);
      for (const [metric, window] of windows) {
        expect([metric, window]).toEqual([
          metric,
          { span: 12 * HOUR_MS, step: 300, poll: 0 },
        ]);
      }
    });

    it("moves every read, limits and capacity included, to the range picked", async () => {
      const user = userEvent.setup();
      mockUseDatastoreMetrics.mockReturnValue(emptyResult());
      render(<DatastoreMetricsPanel kind="keyvalue" resource="red-abc" />);

      await user.click(screen.getByRole("combobox", { name: "Time range" }));
      await user.click(screen.getByRole("option", { name: "Last 7 days" }));

      expect(
        screen.getByRole("combobox", { name: "Time range" }),
      ).toHaveTextContent("Last 7 days");
      for (const [metric, window] of readWindows()) {
        expect([metric, window]).toEqual([
          metric,
          { span: 7 * 24 * HOUR_MS, step: 5040, poll: 0 },
        ]);
      }
    });

    // w4/703: a host route threads the URL-persisted range in, so the panel
    // shows and reads the range it is given and hands a pick back to it.
    it("follows a host-controlled range and hands each pick back to the host", async () => {
      const user = userEvent.setup();
      const onRangeChange = vi.fn();
      mockUseDatastoreMetrics.mockReturnValue(emptyResult());
      render(
        <DatastoreMetricsPanel
          kind="keyvalue"
          resource="red-abc"
          range={RANGE_PRESETS[2]}
          onRangeChange={onRangeChange}
        />,
      );

      expect(
        screen.getByRole("combobox", { name: "Time range" }),
      ).toHaveTextContent("Last 4 hours");
      for (const [metric, window] of readWindows()) {
        expect([metric, window]).toEqual([
          metric,
          { span: 4 * HOUR_MS, step: 120, poll: 0 },
        ]);
      }

      await user.click(screen.getByRole("combobox", { name: "Time range" }));
      await user.click(screen.getByRole("option", { name: "Last 7 days" }));

      expect(onRangeChange).toHaveBeenCalledWith(
        expect.objectContaining({ id: "7d" }),
      );
      // The host owns the range: until it passes the new one, the panel stays.
      expect(
        screen.getByRole("combobox", { name: "Time range" }),
      ).toHaveTextContent("Last 4 hours");
    });

    it("bounds the picker by the plan's metrics window and names the plans that keep more", async () => {
      const user = userEvent.setup();
      const bound: RangeBound = {
        maxSpanSeconds: 7 * 24 * 60 * 60,
        hint: "Metrics are kept for 7 days on the Hobby plan.",
      };
      mockUseMetricsRetentionBound.mockReturnValue(bound);
      mockUseDatastoreMetrics.mockReturnValue(emptyResult());
      render(<DatastoreMetricsPanel kind="service" resource="srv-x" />);

      // The window is the resource's own workspace plan's, read by its id.
      expect(mockUseMetricsRetentionBound).toHaveBeenCalledWith("srv-x");
      await user.click(screen.getByRole("combobox", { name: "Time range" }));
      expect(
        screen.getByRole("option", { name: "Last 7 days" }),
      ).not.toHaveAttribute("aria-disabled");
      for (const name of ["Last 14 days", "Last 30 days"]) {
        expect(screen.getByRole("option", { name })).toHaveAttribute(
          "aria-disabled",
          "true",
        );
      }
      expect(screen.getByText(bound.hint!)).toBeInTheDocument();
    });
  });

  // The route skeleton previews this panel: the same sections in the same
  // order, so nothing moves when the panel replaces it.
  it.each(["database", "keyvalue"] as const)(
    "matches its %s skeleton section for section",
    (kind) => {
      mockUseDatastoreMetrics.mockReturnValue(emptyResult());
      const ready = render(<DatastoreMetricsPanel kind={kind} resource="r" />);
      const pending = render(<DatastoreMetricsSkeleton kind={kind} />);
      expect(headings(pending.container)).toEqual(headings(ready.container));

      // Both open with the range row, and the skeleton's slot is the small
      // w-40 trigger's size (h-8 is the trigger's sm size).
      const rangeRow = (root: HTMLElement) =>
        root.querySelector("[data-slot=card-content]")!.firstElementChild!;
      expect(rangeRow(pending.container).className).toBe(
        rangeRow(ready.container).className,
      );
      const trigger = rangeRow(ready.container).querySelector(
        "[role=combobox]",
      );
      expect(trigger).toHaveClass("w-40");
      expect(trigger).toHaveAttribute("data-size", "sm");
      expect(
        rangeRow(pending.container).querySelector("[data-slot=skeleton]"),
      ).toHaveClass("h-8", "w-40");
    },
  );
});

// w6/m459, w6/138: Render's Active Connections chart reads the plan's
// connection limit beside its title ("Limit 250"), the whole number, for Key
// Value and Postgres alike (its DatabaseMetrics chunk, read 2026-11-18).
describe("DatastoreMetricsPanel connection limit", () => {
  const titleRow = (title: string) =>
    screen.getByText(title, { selector: ".text-sm.font-medium" })
      .parentElement as HTMLElement;

  it("reads the Key Value plan's limit beside Active Connections", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());
    render(
      <DatastoreMetricsPanel
        kind="keyvalue"
        resource="red-x"
        connectionLimit={1000}
      />,
    );
    expect(titleRow("Active Connections")).toHaveTextContent(
      /^Active ConnectionsLimit 1,000$/,
    );
  });

  it("reads the Postgres plan's limit beside Active Connections", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());
    render(
      <DatastoreMetricsPanel
        kind="database"
        resource="dpg-x"
        connectionLimit={100}
      />,
    );
    expect(titleRow("Active Connections")).toHaveTextContent(
      /^Active ConnectionsLimit 100$/,
    );
  });

  it.each(["keyvalue", "database"] as const)(
    "reads no %s limit until the plan catalog says it",
    (kind) => {
      mockUseDatastoreMetrics.mockReturnValue(emptyResult());
      render(<DatastoreMetricsPanel kind={kind} resource="x" />);
      expect(titleRow("Active Connections")).toHaveTextContent(
        /^Active Connections$/,
      );
    },
  );
});
