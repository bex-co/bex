import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useIsMobile, MOBILE_BREAKPOINT } from "../use-mobile";

describe("useIsMobile Hook", () => {
  // Store original values to restore later
  let originalMatchMedia: typeof window.matchMedia;
  let listeners: Array<(event: MediaQueryListEvent) => void> = [];

  beforeEach(() => {
    // Clear listeners
    listeners = [];

    // Save original matchMedia
    originalMatchMedia = window.matchMedia;

    // Mock window.matchMedia
    const mockMatchMedia = (query: string) => {
      // Parse the query to extract max-width value
      const maxWidthMatch = query.match(/max-width:\s*(\d+)px/);
      const maxWidth = maxWidthMatch
        ? parseInt(maxWidthMatch[1], 10)
        : MOBILE_BREAKPOINT - 1;

      const mql: MediaQueryList = {
        get matches() {
          // Recalculate matches based on current window width
          return window.innerWidth <= maxWidth;
        },
        media: query,
        onchange: null,
        addListener: vi.fn(), // deprecated
        removeListener: vi.fn(), // deprecated
        addEventListener: vi.fn(
          (event: string, handler: EventListenerOrEventListenerObject) => {
            if (event === "change" && typeof handler === "function") {
              listeners.push(handler);
            }
          },
        ),
        removeEventListener: vi.fn(
          (event: string, handler: EventListenerOrEventListenerObject) => {
            if (event === "change" && typeof handler === "function") {
              listeners = listeners.filter((l) => l !== handler);
            }
          },
        ),
        dispatchEvent: vi.fn(),
      };
      return mql;
    };

    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: mockMatchMedia,
    });
  });

  afterEach(() => {
    // Restore original matchMedia
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: originalMatchMedia,
    });
    listeners = [];
  });

  it("should return false initially for desktop width", () => {
    // Set desktop width
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1024,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);
  });

  it("should return true initially for mobile width", () => {
    // Set mobile width
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 375,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(true);
  });

  it("should return false for exactly breakpoint width", () => {
    // Set width to exactly the breakpoint
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: MOBILE_BREAKPOINT,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);
  });

  it("should return true for one pixel below breakpoint", () => {
    // Set width to one pixel below breakpoint
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: MOBILE_BREAKPOINT - 1,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(true);
  });

  it("should update when window is resized to mobile", () => {
    // Start with desktop width
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1024,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);

    // Resize to mobile
    act(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 375,
      });

      // Trigger the change event
      listeners.forEach((listener) => {
        listener({} as MediaQueryListEvent);
      });
    });

    expect(result.current).toBe(true);
  });

  it("should update when window is resized to desktop", () => {
    // Start with mobile width
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 375,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(true);

    // Resize to desktop
    act(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 1024,
      });

      // Trigger the change event
      listeners.forEach((listener) => {
        listener({} as MediaQueryListEvent);
      });
    });

    expect(result.current).toBe(false);
  });

  it("should handle multiple resize events", () => {
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1024,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);

    // Resize to mobile
    act(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 375,
      });
      listeners.forEach((listener) => listener({} as MediaQueryListEvent));
    });
    expect(result.current).toBe(true);

    // Resize back to desktop
    act(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 1024,
      });
      listeners.forEach((listener) => listener({} as MediaQueryListEvent));
    });
    expect(result.current).toBe(false);

    // Resize to tablet (still desktop)
    act(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 800,
      });
      listeners.forEach((listener) => listener({} as MediaQueryListEvent));
    });
    expect(result.current).toBe(false);
  });

  it("should handle very small mobile widths", () => {
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 320,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(true);
  });

  it("should handle very large desktop widths", () => {
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 2560,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);
  });

  it("should clean up event listener on unmount", () => {
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1024,
    });

    const { unmount } = renderHook(() => useIsMobile());

    expect(listeners.length).toBeGreaterThan(0);

    unmount();

    // After unmount, listeners should be cleaned up
    expect(listeners.length).toBe(0);
  });
});
