import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  installPreloadErrorReload,
  PRELOAD_RELOAD_KEY,
} from "@/common/lib/preload-error-reload";

function preloadError(target: EventTarget) {
  target.dispatchEvent(new Event("vite:preloadError", { cancelable: true }));
}

describe("installPreloadErrorReload (w4/651)", () => {
  beforeEach(() => sessionStorage.clear());

  it("reloads once per build id, and a new build re-arms it", () => {
    const reload = vi.fn();
    const oldTab = new EventTarget();
    installPreloadErrorReload(oldTab, sessionStorage, reload, "build-a");

    preloadError(oldTab);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(PRELOAD_RELOAD_KEY)).toBe("build-a");

    // The reloaded document is the same build: a second failure stays put.
    preloadError(oldTab);
    expect(reload).toHaveBeenCalledTimes(1);

    // A document from a newer build may reload once more.
    const newTab = new EventTarget();
    installPreloadErrorReload(newTab, sessionStorage, reload, "build-b");
    preloadError(newTab);
    expect(reload).toHaveBeenCalledTimes(2);
    expect(sessionStorage.getItem(PRELOAD_RELOAD_KEY)).toBe("build-b");
  });

  it("leaves the event uncancelled, so the importer still sees the error", () => {
    const tab = new EventTarget();
    installPreloadErrorReload(tab, sessionStorage, vi.fn(), "build-a");
    const event = new Event("vite:preloadError", { cancelable: true });
    tab.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
  });
});
