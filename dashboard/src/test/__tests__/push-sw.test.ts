import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// public/push-sw.js is plain JavaScript run in a service worker, so its click
// handler is tested by running the file against a stub of `self` (w21/m410 t001).
// A notification's route is the one value that reaches client.navigate and
// clients.openWindow: it must stay a same-origin path, never `//host`.
const ORIGIN = "https://dashboard.bex.co";
const SOURCE = readFileSync(`${process.cwd()}/public/push-sw.js`, "utf8");

// Runs the worker's notificationclick handler for a notification carrying
// `route` and returns every URL it navigated or opened.
async function clicked(route: unknown): Promise<string[]> {
  const urls: string[] = [];
  const handlers: Record<string, (event: unknown) => void> = {};
  const self = {
    location: { origin: ORIGIN },
    addEventListener: (type: string, handler: (event: unknown) => void) => {
      handlers[type] = handler;
    },
    clients: {
      matchAll: () =>
        Promise.resolve([
          {
            focus: () => Promise.resolve(),
            navigate: (url: string) => {
              urls.push(url);
              return Promise.resolve();
            },
          },
        ]),
      openWindow: (url: string) => {
        urls.push(url);
        return Promise.resolve();
      },
    },
    registration: { showNotification: () => Promise.resolve() },
  };
  new Function("self", SOURCE)(self);
  let work: Promise<unknown> | undefined;
  handlers.notificationclick({
    notification: { close: () => undefined, data: { route } },
    waitUntil: (promise: Promise<unknown>) => {
      work = promise;
    },
  });
  await work;
  return urls;
}

describe("the service worker's notification click", () => {
  it.each([
    ["/services/srv-1", "/services/srv-1"],
    ["/services/srv-1?tab=logs#top", "/services/srv-1?tab=logs#top"],
    ["/a/../b", "/b"],
  ])("opens the same-origin route %s", async (route, want) => {
    expect(await clicked(route)).toEqual([want]);
  });

  it.each([
    "//evil.com",
    "/\\evil.com",
    "https://evil.com",
    "javascript:alert(1)",
    "relative/path",
    "",
    undefined,
    42,
    // a dot segment that collapses into a protocol-relative authority
    "/.//evil.com",
    "/a/..//evil.com",
    "/..//evil.com",
    "/%2e//evil.com",
    "/././/evil.com/path?x=1#h",
  ])("navigates nowhere for %j", async (route) => {
    expect(await clicked(route)).toEqual([]);
  });
});
