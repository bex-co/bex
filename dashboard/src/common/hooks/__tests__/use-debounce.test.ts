import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDebounce } from "../use-debounce";

describe("useDebounce Hook", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should return initial value immediately", () => {
    const { result } = renderHook(() => useDebounce("initial", 300));

    expect(result.current).toBe("initial");
  });

  it("should debounce value changes", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: "initial" } },
    );

    expect(result.current).toBe("initial");

    // Update the value
    rerender({ value: "updated" });

    // Value should still be the old one before delay
    expect(result.current).toBe("initial");

    // Fast forward time by 300ms
    act(() => {
      vi.advanceTimersByTime(300);
    });

    // Now the value should be updated
    expect(result.current).toBe("updated");
  });

  it("should use default delay of 300ms", () => {
    const { result, rerender } = renderHook(({ value }) => useDebounce(value), {
      initialProps: { value: "initial" },
    });

    rerender({ value: "updated" });

    // Should not update after 299ms
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe("initial");

    // Should update after 1 more ms (total 300ms)
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe("updated");
  });

  it("should reset timer when value changes rapidly", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: "initial" } },
    );

    // First update
    rerender({ value: "first" });

    // Wait 200ms
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current).toBe("initial");

    // Second update (should reset timer)
    rerender({ value: "second" });

    // Wait another 200ms (400ms total, but timer was reset)
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current).toBe("initial");

    // Wait remaining time
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current).toBe("second");
  });

  it("should handle number values", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: 0 } },
    );

    expect(result.current).toBe(0);

    rerender({ value: 42 });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe(42);
  });

  it("should handle object values", () => {
    const initialObj = { name: "initial" };
    const updatedObj = { name: "updated" };

    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: initialObj } },
    );

    expect(result.current).toBe(initialObj);

    rerender({ value: updatedObj });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe(updatedObj);
  });

  it("should handle null values", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce<string | null>(value, 300),
      { initialProps: { value: "initial" as string | null } },
    );

    expect(result.current).toBe("initial");

    rerender({ value: null });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBeNull();
  });

  it("should handle undefined values", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce<string | undefined>(value, 300),
      { initialProps: { value: "initial" as string | undefined } },
    );

    expect(result.current).toBe("initial");

    rerender({ value: undefined });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBeUndefined();
  });

  it("should handle different delay values", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 500),
      { initialProps: { value: "initial" } },
    );

    rerender({ value: "updated" });

    // Should not update after 300ms
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(result.current).toBe("initial");

    // Should update after another 200ms (total 500ms)
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current).toBe("updated");
  });

  it("should handle zero delay", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 0),
      { initialProps: { value: "initial" } },
    );

    rerender({ value: "updated" });

    act(() => {
      vi.advanceTimersByTime(0);
    });

    expect(result.current).toBe("updated");
  });

  it("should handle boolean values", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: false } },
    );

    expect(result.current).toBe(false);

    rerender({ value: true });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe(true);
  });

  it("should handle array values", () => {
    const initialArr = [1, 2, 3];
    const updatedArr = [4, 5, 6];

    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: initialArr } },
    );

    expect(result.current).toBe(initialArr);

    rerender({ value: updatedArr });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe(updatedArr);
  });

  it("should cleanup timer on unmount", () => {
    const clearTimeoutSpy = vi.spyOn(global, "clearTimeout");

    const { rerender, unmount } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: "initial" } },
    );

    rerender({ value: "updated" });
    unmount();

    // clearTimeout should have been called during cleanup
    expect(clearTimeoutSpy).toHaveBeenCalled();

    clearTimeoutSpy.mockRestore();
  });

  it("should handle multiple rapid value changes", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: "initial" } },
    );

    // Simulate rapid typing
    for (let i = 0; i < 10; i++) {
      rerender({ value: `value-${i}` });
      act(() => {
        vi.advanceTimersByTime(50);
      });
    }

    expect(result.current).toBe("initial");

    // Wait for the final debounce
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe("value-9");
  });

  it("should work with delay changes", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      { initialProps: { value: "initial", delay: 300 } },
    );

    // Change both value and delay
    rerender({ value: "updated", delay: 500 });

    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(result.current).toBe("initial");

    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current).toBe("updated");
  });
});
