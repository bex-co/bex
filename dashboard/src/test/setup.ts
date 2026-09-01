import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// React Testing Library unmounts rendered trees after each test, but Radix
// FocusScope dispatches its unmount autofocus event from a zero-delay timer.
// Drain that timer while this test file's jsdom realm is still alive; otherwise
// the event can be constructed in the next realm and rejected by the old DOM's
// dispatchEvent during worker/file teardown.
afterEach(async () => {
  cleanup();

  // Reset sessionStorage between tests. `useReauthDraft` (w3/m80 t005) mirrors
  // an in-progress editor draft there; without this, a test that unmounts
  // mid-edit would leave a draft that the next test's editor restores on mount,
  // silently opening it into edit mode. (localStorage is left alone — some
  // suites seed it once in beforeAll, e.g. i18n/workspace persistence.)
  try {
    window.sessionStorage.clear();
  } catch {
    // no storage in this environment — nothing to reset
  }

  if (vi.isFakeTimers()) {
    await vi.advanceTimersByTimeAsync(0);
    return;
  }

  await new Promise<void>((resolve) => setTimeout(resolve, 0));
});

// Replace the server-biased createIsomorphicFn runtime with a client-biased
// one before anything imports it (hoisted above the imports below) — tests
// are the client environment. See src/test/tanstack-start-fn-stubs.ts.
vi.mock("@tanstack/react-start", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-start")>();
  const stubs = await import("./tanstack-start-fn-stubs");
  return { ...actual, createIsomorphicFn: stubs.createIsomorphicFn };
});

// Role-awareness (w9/m84) is read through useCapabilities, which calls Apollo's
// useQuery — so any component test that renders a gated control would otherwise
// need an ApolloProvider. Mock it here with a permissive default (every
// capability granted = the pre-m84 behavior) so unrelated tests are unaffected;
// gating tests override per file with vi.mocked(useCapabilities).mockReturnValue.
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: vi.fn(() => ({
    role: "ADMIN",
    canView: true,
    canViewLogs: true,
    canOperate: true,
    canCreate: true,
    canViewSensitive: true,
    canManageKeys: true,
    canManage: true,
    canManageBilling: true,
    loading: false,
    loaded: true,
  })),
}));

import i18n from "@/i18n/init";
import zhResources from "@/i18n/resources-zh";

// w9/m60 t003 made non-default locales lazy-loaded in the app (an `import()`
// gated behind `ensureLanguage`), so `i18n.changeLanguage("zh")` alone no
// longer has the zh catalog registered. Tests switch language synchronously
// and assert translated strings, so preload the (statically importable in the
// test runtime) zh bundle once here to restore eager availability.
i18n.addResourceBundle("zh", "translation", zhResources, true, true);

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
    constructor(_callback: ResizeObserverCallback) {}
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

  // ProseMirror positions selections with Range geometry. jsdom exposes
  // Range, but not these layout methods, so rich-editor interactions otherwise
  // fail while dispatching an otherwise-valid document transaction.
  if (typeof Range !== "undefined") {
    Range.prototype.getBoundingClientRect = vi.fn(() => new DOMRect());
    Range.prototype.getClientRects = vi.fn(
      () =>
        ({
          length: 0,
          item: () => null,
          [Symbol.iterator]: function* () {},
        }) as DOMRectList,
    );
  }
  if (typeof Document !== "undefined") {
    Document.prototype.elementFromPoint = vi.fn(
      () => document.activeElement ?? document.body,
    );
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
