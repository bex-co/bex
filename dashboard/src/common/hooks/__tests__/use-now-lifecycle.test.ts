import { createElement, StrictMode } from "react";
import { renderToString } from "react-dom/server";
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

describe("useNow render and subscription boundaries", () => {
  it("samples SSR per render without starting a browser subscription", () => {
    function Clock() {
      return createElement("time", null, useNow());
    }
    const first = renderToString(createElement(Clock));
    vi.setSystemTime(START + 120_000);
    const second = renderToString(createElement(Clock));
    expect(first).toContain(String(START));
    expect(second).toContain(String(START + 120_000));
    expect(vi.getTimerCount()).toBe(0);
  });

  it("joins an active cadence without postponing its next sample", () => {
    const first = renderHook(() => useNow());
    act(() => vi.advanceTimersByTime(30_000));
    const joining = renderHook(() => useNow());
    expect(joining.result.current).toBe(first.result.current);
    act(() => vi.advanceTimersByTime(30_000));
    expect(first.result.current).toBe(START + 60_000);
    expect(joining.result.current).toBe(START + 60_000);
    expect(vi.getTimerCount()).toBe(1);
  });

  it("uses wall time after a delayed callback and cleans up StrictMode replay", () => {
    const { result, unmount } = renderHook(() => useNow(), {
      wrapper: ({ children }) => createElement(StrictMode, null, children),
    });
    expect(vi.getTimerCount()).toBe(1);
    vi.setSystemTime(START + 5 * 60_000);
    act(() => vi.advanceTimersByTime(60_000));
    expect(result.current).toBe(START + 6 * 60_000);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
