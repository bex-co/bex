import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useDatastoreMetrics } from "@/features/metrics/hooks/use-datastore-metrics";
import { METRICS_UNAVAILABLE_MESSAGE } from "@/features/metrics/hooks/use-metrics";

const mockUseQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("useDatastoreMetrics", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
  });

  it("builds a DatastoreMetricsQueryInput naming the resource + kind directly (no filters array)", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
    });

    renderHook(() => useDatastoreMetrics("database", "pg", "disk"));

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: {
          query: {
            kind: "database",
            resource: "pg",
            name: "DISK",
            start: undefined,
            end: undefined,
            resolution: undefined,
          },
        },
        skip: false,
        pollInterval: 30_000,
      }),
    );
  });

  it("maps db_connections/replication_lag/disk_capacity to their GraphQL name enum", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
    });

    renderHook(() => useDatastoreMetrics("database", "pg", "db_connections"));
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: expect.objectContaining({
          query: expect.objectContaining({ name: "DB_CONNECTIONS" }),
        }),
      }),
    );

    renderHook(() => useDatastoreMetrics("database", "pg", "replication_lag"));
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: expect.objectContaining({
          query: expect.objectContaining({ name: "REPLICATION_LAG" }),
        }),
      }),
    );

    renderHook(() => useDatastoreMetrics("keyvalue", "kv", "disk_capacity"));
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: expect.objectContaining({
          query: expect.objectContaining({
            kind: "keyvalue",
            name: "DISK_CAPACITY",
          }),
        }),
      }),
    );
  });

  it("passes skip through — the panel's escape hatch for a metric that doesn't apply to a kind", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: undefined,
    });

    renderHook(() =>
      useDatastoreMetrics("keyvalue", "kv", "db_connections", { skip: true }),
    );

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ skip: true }),
    );
  });

  it("maps GraphQL series into ChartSeries, dropping null entries", () => {
    mockUseQuery.mockReturnValue({
      loading: false,
      error: undefined,
      data: {
        datastoreMetrics: [
          {
            unit: "bytes",
            labels: [{ field: "instance", value: "pg-1" }],
            values: [{ time: "2026-07-06T09:00:00Z", value: 1024 }],
          },
          null,
        ],
      },
    });

    const { result } = renderHook(() =>
      useDatastoreMetrics("database", "pg", "disk"),
    );

    expect(result.current.series).toEqual([
      {
        unit: "bytes",
        labels: { instance: "pg-1" },
        points: [{ timestamp: "2026-07-06T09:00:00Z", value: 1024 }],
      },
    ]);
    expect(result.current.unavailable).toBe(false);
  });

  it("reports unavailable (not a generic error) when bex-api has no source wired", () => {
    const unavailableError = new CombinedGraphQLErrors({
      errors: [{ message: METRICS_UNAVAILABLE_MESSAGE }],
    } as never);
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: unavailableError,
    });

    const { result } = renderHook(() =>
      useDatastoreMetrics("database", "pg", "db_connections"),
    );

    expect(result.current.unavailable).toBe(true);
    expect(result.current.error).toBeUndefined();
    expect(result.current.series).toEqual([]);
  });
});
