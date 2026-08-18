import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useOnMounted } from "../use-on-mounted";

describe("useOnMounted", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should call callback on mount", () => {
    const callback = vi.fn();

    renderHook(() => useOnMounted(callback));

    expect(callback).toHaveBeenCalledTimes(1);
  });

  it("should only call callback once even on re-render", () => {
    const callback = vi.fn();

    const { rerender } = renderHook(() => useOnMounted(callback));

    expect(callback).toHaveBeenCalledTimes(1);

    rerender();

    expect(callback).toHaveBeenCalledTimes(1);
  });

  it("should handle synchronous callback", () => {
    const values: string[] = [];
    const callback = () => {
      values.push("mounted");
    };

    renderHook(() => useOnMounted(callback));

    expect(values).toEqual(["mounted"]);
  });

  it("should handle async callback", async () => {
    const values: string[] = [];
    const callback = async () => {
      await Promise.resolve();
      values.push("async mounted");
    };

    renderHook(() => useOnMounted(callback));

    // Initially the promise is pending
    expect(values).toEqual([]);

    // Wait for async callback to complete
    await vi.runAllTimersAsync();
    await Promise.resolve();

    expect(values).toEqual(["async mounted"]);
  });

  it("should call cleanup function for sync callbacks on unmount", () => {
    const cleanup = vi.fn();
    const callback = () => cleanup;

    const { unmount } = renderHook(() => useOnMounted(callback));

    expect(cleanup).not.toHaveBeenCalled();

    unmount();

    expect(cleanup).toHaveBeenCalledTimes(1);
  });

  it("should not use async callback return value as cleanup", async () => {
    const cleanup = vi.fn();
    // Typed as a void-returning callback (the hook's contract): an async
    // callback's resolved value is never used as cleanup, which is what this
    // test asserts. The void return position deliberately discards the Promise.
    const callback: () => void = async () => {
      await Promise.resolve();
      return cleanup;
    };

    const { unmount } = renderHook(() => useOnMounted(callback));

    await vi.runAllTimersAsync();

    unmount();

    // Cleanup should NOT be called for async callbacks
    expect(cleanup).not.toHaveBeenCalled();
  });

  it("should handle callback that returns void", () => {
    const callback = () => {
      // Intentionally empty - testing void return
    };

    const { unmount } = renderHook(() => useOnMounted(callback));

    // Should not throw
    expect(() => unmount()).not.toThrow();
  });

  it("should work with multiple instances", () => {
    const callback1 = vi.fn();
    const callback2 = vi.fn();

    renderHook(() => useOnMounted(callback1));
    renderHook(() => useOnMounted(callback2));

    expect(callback1).toHaveBeenCalledTimes(1);
    expect(callback2).toHaveBeenCalledTimes(1);
  });

  it("should not re-run callback when props change", () => {
    const callback = vi.fn();
    let dependency = 1;

    const { rerender } = renderHook(() => {
      const _dep = dependency;
      void _dep; // Use the variable
      useOnMounted(callback);
    });

    expect(callback).toHaveBeenCalledTimes(1);

    dependency = 2;
    rerender();

    expect(callback).toHaveBeenCalledTimes(1);
  });

  it("should handle callback that sets up interval with cleanup", () => {
    const intervalCallback = vi.fn();
    let intervalId: number | null = null;

    const callback = () => {
      intervalId = setInterval(intervalCallback, 1000);
      return () => {
        if (intervalId) clearInterval(intervalId);
      };
    };

    const { unmount } = renderHook(() => useOnMounted(callback));

    // Advance timers to trigger interval
    vi.advanceTimersByTime(3000);
    expect(intervalCallback).toHaveBeenCalledTimes(3);

    unmount();

    // After unmount, interval should be cleared
    vi.advanceTimersByTime(3000);
    expect(intervalCallback).toHaveBeenCalledTimes(3); // No new calls
  });

  it("should handle callback that throws error", () => {
    const errorCallback = () => {
      throw new Error("Test error");
    };

    expect(() => renderHook(() => useOnMounted(errorCallback))).toThrow(
      "Test error",
    );
  });
});
