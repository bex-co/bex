import { useEffect, useRef } from "react";
import { useRouterState } from "@tanstack/react-router";

/** How long a hashed card that mounts late (a lazy section's) is waited for. */
const LATE_CARD_WAIT_MS = 10_000;

/**
 * Scroll the card the URL's hash names into view once the page renders
 * (w3/m641, w3/m649). The router scrolls to a hash as it navigates, but not
 * on a full load, and a datastore page's cards wait for the resource's read:
 * Render's `/high-availability-upgrade` landing on `plan#high-availability`,
 * or an old Overview anchor, would otherwise open at the top of the page. A
 * card inside a lazily loaded section (Plan's `#storage`, w3/m656) mounts
 * after the page does, so it is waited for. `aliases` names the card another
 * hash means (a Key Value instance's Render-shaped `#compute` is its `#plan`
 * card).
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
    const scrollTo = (card: HTMLElement) => {
      scrolledFor.current = key;
      card.scrollIntoView();
    };
    const card = document.getElementById(id);
    if (card) {
      scrollTo(card);
      return;
    }
    const observer = new MutationObserver(() => {
      const late = document.getElementById(id);
      if (!late) return;
      observer.disconnect();
      scrollTo(late);
    });
    observer.observe(document.body, { childList: true, subtree: true });
    const giveUp = window.setTimeout(
      () => observer.disconnect(),
      LATE_CARD_WAIT_MS,
    );
    return () => {
      observer.disconnect();
      window.clearTimeout(giveUp);
    };
  }, [pathname, hash, id, ready]);
}
