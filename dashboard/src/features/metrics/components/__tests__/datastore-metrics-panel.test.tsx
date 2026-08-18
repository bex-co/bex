import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { DatastoreMetricsPanel } from "../datastore-metrics-panel";
import { useDatastoreMetrics } from "@/features/metrics/hooks/use-datastore-metrics";

vi.mock("@/features/metrics/hooks/use-datastore-metrics", () => ({
  useDatastoreMetrics: vi.fn(),
}));

const mockUseDatastoreMetrics = vi.mocked(useDatastoreMetrics);

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

function errorResult() {
  return { ...emptyResult(), error: new Error("boom") };
}

describe("DatastoreMetricsPanel", () => {
  beforeEach(() => {
    mockUseDatastoreMetrics.mockReset();
  });

  // w9/m86 / w5/045: a failed metrics query must render a distinct error card,
  // not the empty "No data in range" chart that hid the wrong-identifier bug.
  it("surfaces a distinct error card when a datastore metric query fails", () => {
    mockUseDatastoreMetrics.mockReturnValue(errorResult());
    render(<DatastoreMetricsPanel kind="database" resource="dpg-abc" />);
    expect(screen.getAllByRole("alert").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Couldn't load this metric").length).toBeGreaterThan(
      0,
    );
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
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "disk_capacity",
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "db_connections",
      { skip: true },
    );
    // No Postgres connections/replication-lag chart sections for a keyvalue.
    expect(screen.queryByText("Active Connections")).not.toBeInTheDocument();
    expect(screen.queryByText("Replication Lag")).not.toBeInTheDocument();
    // But the keyvalue-only Memory + Connections sections DO render (w5/011),
    // and their queries are not skipped.
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "kv_memory",
      { skip: false },
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "keyvalue",
      "sessions-cache",
      "kv_connections",
      { skip: false },
    );
    expect(screen.getByText("Memory")).toBeInTheDocument();
    expect(screen.getByText("Connections")).toBeInTheDocument();
  });

  it("skips the keyvalue memory/connections metrics for a database resource", () => {
    mockUseDatastoreMetrics.mockReturnValue(emptyResult());

    render(<DatastoreMetricsPanel kind="database" resource="pg" />);

    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "kv_memory",
      {
        skip: true,
      },
    );
    expect(mockUseDatastoreMetrics).toHaveBeenCalledWith(
      "database",
      "pg",
      "kv_connections",
      { skip: true },
    );
    // The keyvalue-only Memory section doesn't render for a database.
    expect(screen.queryByText("Memory")).not.toBeInTheDocument();
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
      { skip: true },
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
      { skip: false },
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
});
