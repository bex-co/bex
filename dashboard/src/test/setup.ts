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
    canViewBilling: true,
    canViewActivity: true,
    loading: false,
    loaded: true,
    stale: false,
    unavailable: false,
    generation: 1,
    allows: () => true,
    denied: () => false,
    reasonKey: () => undefined,
    refresh: async () => undefined,
  })),
}));
// …and get that default back after every test, so a test that models a lower
// role needs no reset of its own. A file that unmocks the hook, or mocks it
// with a plain function, has no mock here to reset.
afterEach(() => {
  if (vi.isMockFunction(useCapabilities)) {
    vi.mocked(useCapabilities).mockReset();
  }
});

// The protected-environment gates of a datastore's and an environment group's
// controls (w4/m594) read the resource's action projection through Apollo, so
// default to "nothing refused", as useCapabilities above grants everything.
// Gating tests override per file with
// vi.mocked(useAdminOnlyActions).mockReturnValue; the hook's own test unmocks it.
vi.mock("@/features/capabilities/hooks/use-admin-only-actions", () => ({
  useAdminOnlyActions: vi.fn(() => () => undefined),
}));

// Whether the caller may open a service's shell (w4/726) is read from the
// service's action projection through Apollo too, so default to "may": the
// Shell tab and the Connect menu render as before. Gating tests override per
// file with vi.mocked(useShellGate).mockReturnValue; the hook's own test
// unmocks it.
vi.mock("@/features/services/hooks/use-shell-gate", () => ({
  useShellGate: vi.fn(() => ({ gate: { kind: "ready" }, reason: undefined })),
}));

// Whether the workspace is offered Router (w21/042) is the router flag AND the
// companion service's answer, read through Apollo's useQuery — so every test
// that renders the sidebar or the ⌘K palette would otherwise need an
// ApolloProvider. Default to "not offered", the common case; tests of the
// gating override per file, and the hook's own test unmocks it.
vi.mock(
  "@/common/components/dashboard-layout/use-router-nav-available",
  () => ({
    useRouterNavAvailable: vi.fn(() => false),
  }),
);

// The workspace switcher lists the caller's organizations (w7/m464) through
// Apollo's useQuery, so every test that renders the sidebar would otherwise
// need an ApolloProvider. Default to "in no organization", the common case;
// the switcher's and the org pages' tests override per file.
vi.mock("@/features/organizations/hooks/use-organizations", () => ({
  useOrganizations: vi.fn(() => ({ organizations: [] })),
}));

// The signed-in shell binds the account's theme (w3/m531) through Apollo's
// useQuery, so every test that renders the root shell would otherwise need an
// ApolloProvider. Default to "no account bound"; the hook's own test unmocks
// it.
vi.mock("@/features/appearance/hooks/use-account-theme-sync", () => ({
  useAccountThemeSync: vi.fn(),
}));

// Whether the workspace's pipeline minutes are spent (w3/108) is read through
// Apollo's useQuery from Manual Build, Manual Deploy and a build source's
// Trigger Build, so every test that renders them would otherwise need an
// ApolloProvider. Default to "minutes remain"; the gating tests override per
// file with vi.mocked(usePipelineMinutesExceeded).mockReturnValue, which the
// reset below undoes after each test, and the hook's own test unmocks it.
vi.mock("@/features/usage/hooks/use-pipeline-minutes-exceeded", () => ({
  usePipelineMinutesExceeded: vi.fn(() => false),
}));
afterEach(() => {
  if (vi.isMockFunction(usePipelineMinutesExceeded)) {
    vi.mocked(usePipelineMinutesExceeded).mockReset();
  }
});

import i18n from "@/i18n/init";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { usePipelineMinutesExceeded } from "@/features/usage/hooks/use-pipeline-minutes-exceeded";
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
