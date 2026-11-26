import { useEffect, useRef } from "react";
import { useRouterState } from "@tanstack/react-router";

/**
 * Scroll the card the URL's hash names into view once the page renders
 * (w3/m641, w3/m649). The router scrolls to a hash as it navigates, but not
 * on a full load, and a datastore page's cards wait for the resource's read:
 * Render's `/high-availability-upgrade` landing on `plan#high-availability`,
 * or an old Overview anchor, would otherwise open at the top of the page.
 * `aliases` names the card another hash means (a Key Value instance's
 * Render-shaped `#compute` is its `#plan` card).
 */
export function useScrollToHashedCard(
  pathname: string,
  ready: boolean,
  aliases: Readonly<Record<string, string>> = {},
) {
  const hash = useRouterState({ select: (s) => s.location.hash });
  const scrolledFor = useRef<string | null>(null);
  const id = Object.hasOwn(aliases, hash) ? aliases[hash] : hash;
  useEffect(() => {
    const key = `${pathname}#${hash}`;
    if (!ready || !id || scrolledFor.current === key) return;
    const card = document.getElementById(id);
    if (!card) return;
    scrolledFor.current = key;
    card.scrollIntoView();
  }, [pathname, hash, id, ready]);
}
