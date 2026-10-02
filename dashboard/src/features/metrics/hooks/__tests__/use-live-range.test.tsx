import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useLiveRange } from "../use-live-range";
import {
  RANGE_PRESETS,
  type CustomRange,
  type RangeSelection,
} from "../../lib/range";

const relative = RANGE_PRESETS[0]; // Last 30 minutes, 15-second resolution.
const absolute: CustomRange = {
  id: "custom",
  startTime: "2026-10-01T10:00:00.000Z",
  endTime: "2026-10-01T10:30:00.000Z",
  resolutionSeconds: 15,
};

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-02T12:00:00.000Z"));
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("Metrics selected range clock", () => {
  it("moves both relative bounds every 15 seconds so later events enter the window", () => {
    const { result, unmount } = renderHook(() => useLiveRange(relative));
    expect(result.current).toEqual({
      startTime: "2026-10-02T11:30:00.000Z",
      endTime: "2026-10-02T12:00:00.000Z",
      resolutionSeconds: 15,
    });
    const laterEventAt = "2026-10-02T12:00:10.000Z";
    expect(laterEventAt > result.current.endTime).toBe(true);
    act(() => vi.advanceTimersByTime(14_999));
    expect(result.current.endTime).toBe("2026-10-02T12:00:00.000Z");
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toEqual({
      startTime: "2026-10-02T11:30:15.000Z",
      endTime: "2026-10-02T12:00:15.000Z",
      resolutionSeconds: 15,
    });
    expect(laterEventAt >= result.current.startTime).toBe(true);
    expect(laterEventAt <= result.current.endTime).toBe(true);
    expect(vi.getTimerCount()).toBe(1);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("keeps a custom absolute window fixed without scheduling refreshes", () => {
    const { result, rerender } = renderHook(() => useLiveRange(absolute));
    const initial = result.current;
    expect(vi.getTimerCount()).toBe(0);
    act(() => vi.advanceTimersByTime(60_000));
    rerender();
    expect(result.current).toEqual(initial);
    expect(result.current.startTime).toBe(absolute.startTime);
    expect(result.current.endTime).toBe(absolute.endTime);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("stops the relative schedule when the user selects an absolute range", () => {
    const { result, rerender } = renderHook(
      ({ range }: { range: RangeSelection }) => useLiveRange(range),
      { initialProps: { range: relative as RangeSelection } },
    );
    act(() => vi.advanceTimersByTime(15_000));
    rerender({ range: absolute });
    expect(vi.getTimerCount()).toBe(0);
    act(() => vi.advanceTimersByTime(30_000));
    expect(result.current).toEqual({
      startTime: absolute.startTime,
      endTime: absolute.endTime,
      resolutionSeconds: absolute.resolutionSeconds,
    });
  });
});
