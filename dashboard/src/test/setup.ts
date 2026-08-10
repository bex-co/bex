import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Replace the server-biased createIsomorphicFn runtime with a client-biased
// one before anything imports it (hoisted above the imports below) — tests
// are the client environment. See src/test/tanstack-start-fn-stubs.ts.
vi.mock("@tanstack/react-start", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-start")>();
  const stubs = await import("./tanstack-start-fn-stubs");
  return { ...actual, createIsomorphicFn: stubs.createIsomorphicFn };
});

import "@/i18n/init";

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

  // Mock scrollIntoView + the Pointer Capture API. jsdom implements neither;
  // Radix primitives (e.g. the Select trigger) call hasPointerCapture during a
  // userEvent click and throw without these, so any test that drives a shadcn
  // Select needs them.
  if (typeof Element !== "undefined") {
    Element.prototype.scrollIntoView = vi.fn();
    Element.prototype.hasPointerCapture = vi.fn(() => false);
    Element.prototype.setPointerCapture = vi.fn();
    Element.prototype.releasePointerCapture = vi.fn();
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
