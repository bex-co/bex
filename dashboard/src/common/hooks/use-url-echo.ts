import { useState } from "react";

// Own states remembered between URL echoes; far more than a router commit spans.
const OWN_KEYS_KEPT = 16;

/**
 * Tells a navigation a view did not make from its own URL write echoing back,
 * for a view that keeps local state and mirrors it into the URL.
 *
 * `urlKey` is one comparable value for the state the URL names. `foreign` is
 * true in the render where the URL changed to a state this view has not itself
 * shown since the URL it last adopted — a nav link, Back/Forward, a pasted
 * link: adopt it in that same render. A URL naming a state the view did show
 * is its own write coming back, even when a control has moved on since, and is
 * left alone. Call `showing` every render with the key of the state being
 * rendered (the adopted one, when adopting).
 */
export function useUrlEcho(urlKey: string) {
  const [sync, setSync] = useState({ urlKey, ownKeys: [urlKey] });
  let ownKeys = sync.ownKeys;
  let foreign = false;
  if (sync.urlKey !== urlKey) {
    const echoed = ownKeys.lastIndexOf(urlKey);
    foreign = echoed === -1;
    ownKeys = foreign ? [urlKey] : ownKeys.slice(echoed);
  }
  return {
    foreign,
    showing(key: string) {
      const next =
        ownKeys.at(-1) === key
          ? ownKeys
          : [...ownKeys, key].slice(-OWN_KEYS_KEPT);
      if (sync.urlKey !== urlKey || sync.ownKeys !== next) {
        setSync({ urlKey, ownKeys: next });
      }
    },
  };
}
