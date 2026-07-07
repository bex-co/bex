import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  useMetrics,
  METRICS_UNAVAILABLE_MESSAGE,
} from "@/features/metrics/hooks/use-metrics";

const mockUseQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("useMetrics", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
  });

  it("passes resource/metric and query options through as GraphQL variables", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
    });

    renderHook(() =>
      useMetrics("beancount-cms-v2", "cpu", {
        percentage: true,
        quantile: 0.95,
      }),
    );

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: {
          resource: "beancount-cms-v2",
          metric: "cpu",
          percentage: true,
          quantile: 0.95,
        },
        pollInterval: 30_000,
      }),
    );
  });

  it("defaults to a 30s poll interval and honors an override", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: undefined,
    });

    renderHook(() => useMetrics("app", "cpu", { pollIntervalMs: 5_000 }));

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ pollInterval: 5_000 }),
    );
  });

  it("maps GraphQL series into ChartSeries, dropping null entries", () => {
    mockUseQuery.mockReturnValue({
      loading: false,
      error: undefined,
      data: {
        metrics: [
          {
            unit: "bytes",
            labels: [
              { field: "resource", value: "beancount-cms-v2" },
              { field: "instance", value: "web-1" },
              null, // a malformed label must not crash the mapping
            ],
            points: [
              { timestamp: "2026-07-06T09:00:00Z", value: 1024 },
              { timestamp: null, value: 2048 }, // dropped: no timestamp
            ],
          },
          null, // a malformed series must not crash the mapping
        ],
      },
    });

    const { result } = renderHook(() =>
      useMetrics("beancount-cms-v2", "memory"),
    );

    expect(result.current.series).toEqual([
      {
        unit: "bytes",
        labels: { resource: "beancount-cms-v2", instance: "web-1" },
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

    const { result } = renderHook(() => useMetrics("app", "http_requests"));

    expect(result.current.unavailable).toBe(true);
    expect(result.current.error).toBeUndefined();
    expect(result.current.series).toEqual([]);
  });

  it("surfaces an unrelated GraphQL error as a real error, not unavailable", () => {
    const otherError = new CombinedGraphQLErrors({
      errors: [{ message: "app not found" }],
    } as never);
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: otherError,
    });

    const { result } = renderHook(() => useMetrics("nope", "cpu"));

    expect(result.current.unavailable).toBe(false);
    expect(result.current.error).toBe(otherError);
  });
});
