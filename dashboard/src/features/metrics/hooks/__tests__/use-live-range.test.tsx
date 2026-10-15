import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useLiveRange } from "../use-live-range";
import {
  RANGE_PRESETS,
  type CustomRange,
  type RangeSelection,
} from "../../lib/range";

const relative = RANGE_PRESETS[0]; // Last 30 minutes, 15-second resolution.
const last12h = RANGE_PRESETS.find((preset) => preset.id === "12h")!;
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

  const renderSelection = (initial: RangeSelection) => {
    const seen: string[] = [];
    const hook = renderHook(
      ({ range }: { range: RangeSelection }) => {
        const window = useLiveRange(range);
        seen.push(window.endTime);
        return window;
      },
      { initialProps: { range: initial } },
    );
    return { ...hook, seen };
  };

  it("ends a relative window selected after a custom range at the selection time", () => {
    const { result, rerender, seen } = renderSelection(absolute);
    act(() => vi.advanceTimersByTime(90_000));
    seen.length = 0;
    rerender({ range: last12h });
    // No render after the selection may expose the mount clock: the first
    // query is built from the first bounds a consumer sees.
    expect(seen).not.toContain("2026-10-02T12:00:00.000Z");
    expect(result.current).toEqual({
      startTime: "2026-10-02T00:01:30.000Z",
      endTime: "2026-10-02T12:01:30.000Z",
      resolutionSeconds: last12h.resolutionSeconds,
    });
    expect(vi.getTimerCount()).toBe(1);
    act(() => vi.advanceTimersByTime(last12h.resolutionSeconds * 1000));
    expect(result.current.endTime).toBe("2026-10-02T12:06:30.000Z");
  });

  it("samples again when the same preset is selected after a custom range", () => {
    const { result, rerender } = renderSelection(relative);
    act(() => vi.advanceTimersByTime(15_000));
    rerender({ range: absolute });
    act(() => vi.advanceTimersByTime(100_000));
    rerender({ range: relative });
    expect(result.current.endTime).toBe("2026-10-02T12:01:55.000Z");
    act(() => vi.advanceTimersByTime(15_000));
    expect(result.current.endTime).toBe("2026-10-02T12:02:10.000Z");
  });

  it("samples again when one relative preset replaces another", () => {
    const { result, rerender } = renderSelection(last12h);
    act(() => vi.advanceTimersByTime(128_000));
    expect(result.current.endTime).toBe("2026-10-02T12:00:00.000Z");
    rerender({ range: relative });
    expect(result.current).toEqual({
      startTime: "2026-10-02T11:32:08.000Z",
      endTime: "2026-10-02T12:02:08.000Z",
      resolutionSeconds: 15,
    });
    // The 300-second schedule is gone; only the 15-second one runs.
    expect(vi.getTimerCount()).toBe(1);
    act(() => vi.advanceTimersByTime(15_000));
    expect(result.current.endTime).toBe("2026-10-02T12:02:23.000Z");
  });

  it("samples again and restarts the cadence when only the span changes", () => {
    const window = { spanSeconds: 3600, resolutionSeconds: 60 };
    const { result, rerender } = renderHook(
      ({ range }) => useLiveRange(range),
      { initialProps: { range: window } },
    );
    act(() => vi.advanceTimersByTime(40_000));
    rerender({ range: { ...window, spanSeconds: 7200 } });
    expect(result.current.startTime).toBe("2026-10-02T10:00:40.000Z");
    expect(result.current.endTime).toBe("2026-10-02T12:00:40.000Z");
    // The earlier schedule would have fired 20 seconds from here.
    act(() => vi.advanceTimersByTime(59_999));
    expect(result.current.endTime).toBe("2026-10-02T12:00:40.000Z");
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.endTime).toBe("2026-10-02T12:01:40.000Z");
    expect(vi.getTimerCount()).toBe(1);
  });

  it("holds the window still across rerenders that select nothing new", () => {
    const { result, rerender } = renderHook(() =>
      // A fresh object each render, as the Scaling page's caller could pass.
      useLiveRange({ spanSeconds: 172_800, resolutionSeconds: 1440 }),
    );
    const initial = result.current;
    act(() => vi.advanceTimersByTime(600_000));
    rerender();
    expect(result.current).toEqual(initial);
    expect(vi.getTimerCount()).toBe(1);
    act(() => vi.advanceTimersByTime(840_000));
    expect(result.current.endTime).toBe("2026-10-02T12:24:00.000Z");
  });

  it("samples again when a new reset key is named under the same preset (w4/306)", () => {
    const hourly = RANGE_PRESETS.find((preset) => preset.id === "1h")!;
    const seen: string[] = [];
    const { result, rerender } = renderHook(
      ({ resetKey }: { resetKey?: string }) => {
        const window = useLiveRange(hourly, resetKey);
        seen.push(window.endTime);
        return window;
      },
      { initialProps: { resetKey: "a" as string | undefined } },
    );
    act(() => vi.advanceTimersByTime(15_000));
    seen.length = 0;
    rerender({ resetKey: "b" });
    // The first bounds the new key sees end at its commit, not the mount.
    expect(seen).not.toContain("2026-10-02T12:00:00.000Z");
    expect(result.current.endTime).toBe("2026-10-02T12:00:15.000Z");
    // The new key owns one cadence, counted from its own sample.
    expect(vi.getTimerCount()).toBe(1);
    act(() => vi.advanceTimersByTime(hourly.resolutionSeconds * 1000 - 1));
    expect(result.current.endTime).toBe("2026-10-02T12:00:15.000Z");
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.endTime).toBe("2026-10-02T12:00:45.000Z");
    // The same key again is not a new selection, nor is an unchanged one.
    const renders = seen.length;
    rerender({ resetKey: "b" });
    expect(seen.length).toBe(renders + 1);
    expect(result.current.endTime).toBe("2026-10-02T12:00:45.000Z");
  });

  it("keeps a custom window fixed when its reset key changes", () => {
    const { result, rerender } = renderHook(
      ({ resetKey }: { resetKey: string }) => useLiveRange(absolute, resetKey),
      { initialProps: { resetKey: "a" } },
    );
    act(() => vi.advanceTimersByTime(60_000));
    rerender({ resetKey: "b" });
    expect(result.current.startTime).toBe(absolute.startTime);
    expect(result.current.endTime).toBe(absolute.endTime);
    expect(vi.getTimerCount()).toBe(0);
  });
});
