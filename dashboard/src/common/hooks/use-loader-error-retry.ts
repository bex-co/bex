import { useEffect, useRef } from "react";
import { useRouter } from "@tanstack/react-router";

/**
 * Self-heal a dehydrated loader error (w1/m52 t003): `loadRouteResource`
 * serializes `{ state: "error" }` into the page when its query fails during
 * SSR — say, while bex-api rolls — and no child ever re-runs the loader, so
 * the page stays on "Couldn't load resources" until a manual reload even
 * though the API recovered seconds later. When a resource shell hydrates (or
 * client-navigates) into that state, re-run the loaders ONCE via a
 * same-location `replace` navigation; the retry link has usually already
 * healed the transport by then, so the re-run returns real data.
 *
 * One retry per resource `key` (the route param): a retry that still errors
 * stays an error — no loop — while navigating to a different resource arms
 * the retry again. Mounted by the resource shells beside `useNotFoundRedirect`
 * with the shell's own dehydrated `RouteResource`; an absent resource (a shell
 * rendered outside its route, as in tests) is a no-op.
 *
 * Both re-runs are same-location `replace` navigations, NOT
 * `router.invalidate()`: an invalidate re-runs the loaders but leaves every
 * match's head/`meta` stale (verified live 2026-07-18 — `loaderData` recovered
 * to `ready` while the tab kept the error `<title>`). A match's `meta` is
 * computed at navigation commit from the loaderData available THEN, so even
 * the retry navigation — which begins while still errored — captures the
 * error title; once the retry heals to `ready`, one more replace navigation
 * recomputes the head from the recovered data and clears the stale `<title>`.
 * `search`/`hash: true` keep the URL byte-identical throughout.
 */
const SAME_LOCATION_REPLACE = {
  to: ".",
  search: true,
  hash: true,
  replace: true,
} as const;

export function useLoaderErrorRetry(
  resource: { state: string } | undefined,
  key: string,
) {
  const state = resource?.state;
  const router = useRouter();
  const retriedFor = useRef<string | null>(null);
  const headHealedFor = useRef<string | null>(null);
  useEffect(() => {
    if (state === "error" && retriedFor.current !== key) {
      retriedFor.current = key;
      void router.navigate(SAME_LOCATION_REPLACE);
      return;
    }
    if (
      state === "ready" &&
      retriedFor.current === key &&
      headHealedFor.current !== key
    ) {
      headHealedFor.current = key;
      void router.navigate(SAME_LOCATION_REPLACE);
    }
  }, [state, key, router]);
}
