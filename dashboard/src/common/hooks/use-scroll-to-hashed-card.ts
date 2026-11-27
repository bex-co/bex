import { useEffect, useRef } from "react";
import { useRouterState } from "@tanstack/react-router";

/** How long a hashed card that mounts late (a lazy section's) is waited for,
 *  and how long a scrolled card is kept in view while the page settles. */
const LATE_CARD_WAIT_MS = 10_000;

/** What the reader does that hands the scroll position back to them. */
const READER_EVENTS = ["wheel", "touchstart", "keydown", "pointerdown"];

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
 *
 * Cards above the target can still change height once their own reads land
 * (a skeleton taller than its card), which a browser corrects for a real
 * fragment but not for an alias (Billing's `#free-usage`, w3/m707), so the
 * card is scrolled to again whenever its ancestors resize, until the reader
 * scrolls, types or taps, or the wait runs out.
 */
export function useScrollToHashedCard(
  pathname: string,
  ready: boolean,
  aliases: Readonly<Record<string, string>> = {},
) {
  const hash = useRouterState({ select: (s) => s.location.hash });
  const scrolledFor = useRef<string | null>(null);
  // The hold outlives the effect's re-runs (`ready` can flip back while a
  // read refetches); a new hash or leaving the page ends it, and a page
  // effects run on again (React's development double run) scrolls again.
  const holding = useRef<(() => void) | null>(null);
  const id = Object.hasOwn(aliases, hash) ? aliases[hash] : hash;
  useEffect(
    () => () => {
      holding.current?.();
      holding.current = null;
    },
    [pathname, hash],
  );
  useEffect(() => {
    const key = `${pathname}#${hash}`;
    if (!ready || !id || (scrolledFor.current === key && holding.current)) {
      return;
    }
    const scrollTo = (card: HTMLElement) => {
      scrolledFor.current = key;
      card.scrollIntoView();
      holding.current = keepInView(card);
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

/** Scroll `card` back into view whenever an ancestor resizes, until the
 *  reader takes over or the wait runs out; returns the stop. */
function keepInView(card: HTMLElement): () => void {
  const settled = card.getBoundingClientRect().top;
  const resizes = new ResizeObserver(() => {
    if (Math.abs(card.getBoundingClientRect().top - settled) > 1) {
      card.scrollIntoView();
    }
  });
  for (let el = card.parentElement; el; el = el.parentElement) {
    resizes.observe(el);
  }
  const stop = () => {
    resizes.disconnect();
    window.clearTimeout(timer);
    for (const event of READER_EVENTS) {
      window.removeEventListener(event, stop, true);
    }
  };
  const timer = window.setTimeout(stop, LATE_CARD_WAIT_MS);
  for (const event of READER_EVENTS) {
    window.addEventListener(event, stop, { capture: true, passive: true });
  }
  return stop;
}
