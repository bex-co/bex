import type { ReactElement } from "react";
import { act } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { within } from "@testing-library/react";
import { onTestFinished, vi } from "vitest";
import i18n from "@/i18n/init";
import { memoryStorage } from "@/test/mocks/storage";

export interface HydrationProbe {
  /** The markup the SSR pass produced, at `serverNow`. */
  html: string;
  /**
   * Recoverable errors React raised while hydrating the SSR markup at
   * `clientNow`. A server/client text or attribute divergence — React error
   * #418 — lands here unless the diverging element carries
   * `suppressHydrationWarning`.
   */
  recovered: unknown[];
  /**
   * Errors React reported as uncaught or caught while hydrating (the tree threw
   * rendering on the client), kept out of the test run's unhandled errors.
   */
  failed: unknown[];
  /**
   * True when the server pass threw inside a Suspense boundary: React then
   * marks it for client rendering and re-throws the server's error when the
   * client hydrates it, so there is no markup to compare and nothing is hydrated.
   */
  serverErrored: boolean;
}

/**
 * Server-render `node` with the clock at `serverNow`, then hydrate that exact
 * markup with the clock at `clientNow`, and report what React complained about.
 *
 * The two clocks are what makes this a *boundary* probe: an elapsed-time
 * formatter (`formatRelativeAge`/`formatRelativeUntil`, w6/m102) reads Date.now()
 * once per render pass, so a bucket boundary crossed between the SSR render and
 * hydration makes the same node render different text on each side — the live
 * `/webhook/<id>` repro this guards against.
 */
export function hydrateAcrossBoundary(
  node: ReactElement,
  { serverNow, clientNow }: { serverNow: number; clientNow: number },
): HydrationProbe {
  const nowSpy = vi.spyOn(Date, "now").mockReturnValue(serverNow);
  const html = renderToString(node);

  nowSpy.mockReturnValue(clientNow);
  const container = document.createElement("div");
  container.innerHTML = html;
  document.body.appendChild(container);

  const recovered: unknown[] = [];
  // React logs the mismatch as well; onRecoverableError is the assertable
  // channel, so keep the console quiet.
  const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  let root: ReturnType<typeof hydrateRoot> | undefined;
  act(() => {
    root = hydrateRoot(container, node, {
      onRecoverableError: (error) => recovered.push(error),
    });
  });
  act(() => root?.unmount());
  container.remove();
  errorSpy.mockRestore();
  nowSpy.mockRestore();

  return { html, recovered, failed: [], serverErrored: false };
}

/**
 * Server-render `node` into a container in the document, so a test can assert
 * what the server HTML offers before React attaches; `hydrate()` then attaches
 * React to that exact markup. Returns the container's Testing Library queries
 * like `render` does. Unmounted and removed when the test finishes,
 * pass or fail, so a failed assertion cannot leak a second copy into the
 * tests that follow.
 */
export function serverRendered(node: ReactElement) {
  const container = document.createElement("div");
  container.innerHTML = renderToString(node);
  document.body.appendChild(container);
  let root: ReturnType<typeof hydrateRoot> | undefined;
  onTestFinished(() => {
    act(() => root?.unmount());
    container.remove();
  });
  return {
    container,
    ...within(container),
    hydrate: async () => {
      await act(async () => {
        root = hydrateRoot(container, node);
      });
    },
  };
}

/**
 * What differs between the server that renders a page and the browser that
 * hydrates it. Every field is optional; an unset field keeps the test's own.
 */
export interface HydrationEnvironment {
  /** An IANA zone; Node applies a change of `process.env.TZ` immediately. */
  timeZone?: string;
  /** The i18n language, and the browser's `navigator.language(s)`. */
  language?: string;
  /** The clock, for `Date.now()` and `new Date()` alike. */
  now?: number;
  /** `document.cookie` entries to set, name to value. */
  cookies?: Record<string, string>;
  /** `localStorage` contents; the test's no-op storage is replaced. */
  storage?: Record<string, string>;
}

// Just before a month boundary in UTC, which has already passed in the browser
// zone below: a month label, a "today" and an elapsed-time ratio all differ
// between the two sides, the mismatches w6/049, w6/m107 and w21/013 shipped.
const SERVER_NOW = Date.UTC(2026, 9, 31, 23, 59, 30);

/** The container's side: UTC, English, the clock at SERVER_NOW. */
export const SERVER_ENVIRONMENT: HydrationEnvironment = {
  timeZone: "UTC",
  language: "en",
  now: SERVER_NOW,
};

/** A viewer's browser: Shanghai, English, a minute later than the server. */
export const BROWSER_ENVIRONMENT: HydrationEnvironment = {
  timeZone: "Asia/Shanghai",
  language: "en",
  now: SERVER_NOW + 60_000,
};

/** The Chinese UI for a viewer in Los Angeles, a minute after the server. */
export const CHINESE_BROWSER_ENVIRONMENT: HydrationEnvironment = {
  timeZone: "America/Los_Angeles",
  language: "zh",
  now: SERVER_NOW + 60_000,
  cookies: { i18nextLng: "zh" },
};

async function applyEnvironment(
  env: HydrationEnvironment,
): Promise<() => Promise<void>> {
  const restores: Array<() => void | Promise<void>> = [];

  if (env.timeZone !== undefined) {
    const previous = process.env.TZ;
    process.env.TZ = env.timeZone;
    restores.push(() => {
      if (previous === undefined) delete process.env.TZ;
      else process.env.TZ = previous;
    });
  }

  if (env.now !== undefined) {
    if (vi.isFakeTimers()) {
      const before = Date.now();
      vi.setSystemTime(env.now);
      restores.push(() => void vi.setSystemTime(before));
    } else {
      // Only the clock is faked, so act() and the test's own timers still run.
      vi.useFakeTimers({ toFake: ["Date"], now: env.now });
      restores.push(() => void vi.useRealTimers());
    }
  }

  if (env.language !== undefined) {
    const previous = i18n.language;
    await i18n.changeLanguage(env.language);
    const language = vi
      .spyOn(navigator, "language", "get")
      .mockReturnValue(env.language);
    const languages = vi
      .spyOn(navigator, "languages", "get")
      .mockReturnValue([env.language]);
    restores.push(async () => {
      language.mockRestore();
      languages.mockRestore();
      await i18n.changeLanguage(previous);
    });
  }

  if (env.cookies) {
    const names = Object.keys(env.cookies);
    for (const name of names) {
      document.cookie = `${name}=${encodeURIComponent(env.cookies[name])}; path=/`;
    }
    restores.push(() => {
      for (const name of names) {
        document.cookie = `${name}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT`;
      }
    });
  }

  if (env.storage) {
    const previous = globalThis.localStorage;
    const storage = memoryStorage();
    for (const [key, value] of Object.entries(env.storage)) {
      storage.setItem(key, value);
    }
    vi.stubGlobal("localStorage", storage);
    restores.push(() => void vi.stubGlobal("localStorage", previous));
  }

  return async () => {
    for (const restore of restores.reverse()) await restore();
  };
}

async function inEnvironment<T>(
  env: HydrationEnvironment,
  run: () => Promise<T>,
): Promise<T> {
  const restore = await applyEnvironment(env);
  try {
    return await run();
  } finally {
    await restore();
  }
}

/**
 * Server-render the tree `build` returns in the server environment, then
 * hydrate that markup in the browser environment, and report React's
 * recoverable errors (a text or attribute that differs is error #418).
 *
 * `build` runs once per side, so a router or a store is fresh for each; it may
 * be async to load a router first. Time zone, language, clock, cookies and
 * storage are put back when each side finishes.
 */
export async function hydrateAcrossEnvironments(
  build: () => ReactElement | Promise<ReactElement>,
  {
    server = SERVER_ENVIRONMENT,
    client = BROWSER_ENVIRONMENT,
  }: { server?: HydrationEnvironment; client?: HydrationEnvironment } = {},
): Promise<HydrationProbe> {
  const html = await inEnvironment(server, async () =>
    renderToString(await build()),
  );

  const recovered: unknown[] = [];
  const failed: unknown[] = [];
  // `<!--$!-->` is the marker React writes for a boundary whose server render
  // errored.
  const serverErrored = html.includes("<!--$!-->");
  if (serverErrored) return { html, recovered, failed, serverErrored };
  await inEnvironment(client, async () => {
    const node = await build();
    const container = document.createElement("div");
    container.innerHTML = html;
    document.body.appendChild(container);
    // React logs the mismatch as well; onRecoverableError is the assertable
    // channel, so keep the console quiet.
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    let root: ReturnType<typeof hydrateRoot> | undefined;
    try {
      await act(async () => {
        root = hydrateRoot(container, node, {
          onRecoverableError: (error) => recovered.push(error),
          onUncaughtError: (error) => failed.push(error),
          onCaughtError: (error) => failed.push(error),
        });
      });
    } finally {
      act(() => root?.unmount());
      container.remove();
      errorSpy.mockRestore();
    }
  });

  return { html, recovered, failed, serverErrored };
}
