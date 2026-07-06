import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Mock DOM-specific globals only in DOM environments
if (typeof window !== "undefined") {
  // Mock localStorage with proper typing
  const localStorageMock: Storage = {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
    key: vi.fn(),
    length: 0,
  };

  global.localStorage = localStorageMock;

  // Mock ResizeObserver
  global.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };

  // Mock scrollIntoView
  if (typeof Element !== "undefined") {
    Element.prototype.scrollIntoView = vi.fn();
  }

  // Mock matchMedia (jsdom doesn't implement it) — used by useIsMobile and
  // the theme provider's system-theme detection.
  window.matchMedia =
    window.matchMedia ||
    vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
}
