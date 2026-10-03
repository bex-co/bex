import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useNow } from "../use-now";

const START = Date.parse("2026-07-16T03:00:00Z");

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(START);
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("useNow", () => {
  it("starts at the current clock and re-reads it every interval", () => {
    const { result } = renderHook(() => useNow(60_000));
    expect(result.current).toBe(START);

    act(() => {
      vi.advanceTimersByTime(59_000);
    });
    expect(result.current).toBe(START);

    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(result.current).toBe(START + 60_000);
  });

  it("stops its timer on unmount", () => {
    const { unmount } = renderHook(() => useNow(60_000));
    expect(vi.getTimerCount()).toBe(1);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("useNow shared clock lifecycle", () => {
  it("shares cadence subscriptions, keeps remaining readers alive, and starts fresh after navigation", () => {
    const a = renderHook(() => useNow());
    const b = renderHook(() => useNow());
    expect(vi.getTimerCount()).toBe(1);
    a.unmount();
    expect(vi.getTimerCount()).toBe(1);
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(b.result.current).toBe(START + 60_000);
    b.unmount();
    expect(vi.getTimerCount()).toBe(0);
    vi.setSystemTime(START + 600_000);
    const next = renderHook(() => useNow());
    expect(next.result.current).toBe(START + 600_000);
    expect(vi.getTimerCount()).toBe(1);
    next.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("honors custom cadence without allocating a timer per matching subscriber", () => {
    const normal = renderHook(() => useNow());
    const fast = renderHook(() => useNow(10_000));
    const other = renderHook(() => useNow(10_000));
    expect(vi.getTimerCount()).toBeLessThanOrEqual(2);
    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(fast.result.current).toBe(START + 10_000);
    expect(other.result.current).toBe(START + 10_000);
    normal.unmount();
    fast.unmount();
    other.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("moves a mounted reader to a new cadence without leaking the old timer", () => {
    const { result, rerender, unmount } = renderHook(
      ({ interval }) => useNow(interval),
      { initialProps: { interval: 60_000 } },
    );
    act(() => {
      vi.advanceTimersByTime(15_000);
    });
    rerender({ interval: 5_000 });
    expect(result.current).toBe(START + 15_000);
    expect(vi.getTimerCount()).toBe(1);
    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    expect(result.current).toBe(START + 20_000);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("refreshes immediately when a throttled hidden tab becomes visible", () => {
    const visibility = vi.spyOn(document, "visibilityState", "get");
    visibility.mockReturnValue("visible");
    const { result, unmount } = renderHook(() => useNow());
    visibility.mockReturnValue("hidden");
    act(() => document.dispatchEvent(new Event("visibilitychange")));
    // Background clocks may be throttled entirely: no timer callback is delivered.
    vi.setSystemTime(START + 5 * 60_000);
    visibility.mockReturnValue("visible");
    act(() => document.dispatchEvent(new Event("visibilitychange")));
    expect(result.current).toBe(START + 5 * 60_000);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("useNow resumed active clock", () => {
  it("does not rerender an existing reader when fresh readers join", () => {
    const rendered = vi.fn();
    const existing = renderHook(() => {
      rendered();
      return useNow();
    });
    const initialRenders = rendered.mock.calls.length;
    for (let elapsed = 1; elapsed <= 10; elapsed++) {
      vi.setSystemTime(START + elapsed);
      const joined = renderHook(() => useNow());
      expect(joined.result.current).toBe(existing.result.current);
      joined.unmount();
    }
    expect(rendered).toHaveBeenCalledTimes(initialRenders);
    expect(vi.getTimerCount()).toBe(1);
  });

  it("gives a newly mounted reader a fresh sample while an older reader remains mounted", () => {
    const existing = renderHook(() => useNow());
    // A suspended browser has advanced its clock without delivering callbacks.
    vi.setSystemTime(START + 5 * 60_000);
    const joined = renderHook(() => useNow());
    expect(joined.result.current).toBe(START + 5 * 60_000);
    expect(existing.result.current).toBe(joined.result.current);
    expect(vi.getTimerCount()).toBe(1);
    act(() => vi.advanceTimersByTime(60_000));
    expect(existing.result.current).toBe(START + 6 * 60_000);
    expect(joined.result.current).toBe(existing.result.current);
    vi.setSystemTime(START - 1_000);
    const afterReversal = renderHook(() => useNow());
    expect(afterReversal.result.current).toBe(START - 1_000);
    expect(existing.result.current).toBe(afterReversal.result.current);
  });

  it("refreshes on focus with one listener until the last reader unmounts", () => {
    const added = vi.spyOn(window, "addEventListener");
    const removed = vi.spyOn(window, "removeEventListener");
    const first = renderHook(() => useNow());
    const second = renderHook(() => useNow());
    vi.setSystemTime(START + 5 * 60_000);
    act(() => window.dispatchEvent(new Event("focus")));
    expect(first.result.current).toBe(START + 5 * 60_000);
    expect(second.result.current).toBe(first.result.current);
    expect(
      added.mock.calls.filter(([event]) => event === "focus"),
    ).toHaveLength(1);
    expect(vi.getTimerCount()).toBe(1);

    first.unmount();
    expect(
      removed.mock.calls.filter(([event]) => event === "focus"),
    ).toHaveLength(0);
    vi.setSystemTime(START + 10 * 60_000);
    act(() => window.dispatchEvent(new Event("focus")));
    expect(second.result.current).toBe(START + 10 * 60_000);
    second.unmount();
    expect(
      removed.mock.calls.filter(([event]) => event === "focus"),
    ).toHaveLength(1);
    expect(vi.getTimerCount()).toBe(0);
  });
});
