import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useNow } from "../use-now";

const START = Date.parse("2026-07-16T03:00:00Z");

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(START);
});

afterEach(() => {
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
