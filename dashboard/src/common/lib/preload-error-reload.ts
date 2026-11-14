/**
 * Reload once per build when a dynamic import fails (w4/651).
 *
 * During a rollout's old/new pod overlap, one page load can fetch some chunks
 * from a pod that does not have them. Vite reports each such failure as a
 * `vite:preloadError` event on window before rethrowing it. TanStack Router's
 * own reload (one per error message) and ErrorPage's Try again (w4/269) stay;
 * this covers the loads where TanStack's one reload is already spent and the
 * page would end on "Something went wrong".
 *
 * The guard is a sessionStorage key holding the build id that reloaded, so a
 * persistent failure cannot loop, and a newer build re-arms it. The event is
 * not cancelled: cancelling would hand the importer an undefined module.
 */
export const PRELOAD_RELOAD_KEY = "bex:preload-error-reload";

/** Stamped by vite.config.ts at build time; "dev" in dev and tests. */
const BUILD_ID = import.meta.env.VITE_BEX_BUILD_ID ?? "dev";

export function installPreloadErrorReload(
  target: Pick<Window, "addEventListener"> = window,
  storage: Pick<Storage, "getItem" | "setItem"> = sessionStorage,
  reload: () => void = () => window.location.reload(),
  buildId: string = BUILD_ID,
): void {
  target.addEventListener("vite:preloadError", () => {
    if (storage.getItem(PRELOAD_RELOAD_KEY) === buildId) return;
    storage.setItem(PRELOAD_RELOAD_KEY, buildId);
    reload();
  });
}
