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
 * Cards above and below the target can still change height once their own
 * reads land (a skeleton taller than its card, Billing's `#free-usage`,
 * w3/m707), and a page too short to bring the card up when it is first
 * scrolled to grows later (a Postgres Info page's Access control and SQL
 * console, its Metrics charts under `#insights`, w3/120). A browser corrects
 * neither for a card it did not scroll to itself, so the card is scrolled to
 * again whenever its ancestors resize, until the wait runs out. Once the
 * reader scrolls, types or taps, before the card is reached or after, the
 * scroll position is theirs, as a browser leaves a fragment alone then.
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
  // Set by the reader's first scroll, key or tap under this hash.
  const readerTookOver = useRef(false);
  const id = Object.hasOwn(aliases, hash) ? aliases[hash] : hash;
  useEffect(() => {
    readerTookOver.current = false;
    if (!hash) return;
    const takeOver = () => {
      readerTookOver.current = true;
      holding.current?.();
      unlisten();
    };
    const unlisten = () => {
      for (const event of READER_EVENTS) {
        window.removeEventListener(event, takeOver, true);
      }
    };
    for (const event of READER_EVENTS) {
      window.addEventListener(event, takeOver, {
        capture: true,
        passive: true,
      });
    }
    return () => {
      unlisten();
      holding.current?.();
      holding.current = null;
    };
  }, [pathname, hash]);
  useEffect(() => {
    const key = `${pathname}#${hash}`;
    if (
      !ready ||
      !id ||
      readerTookOver.current ||
      (scrolledFor.current === key && holding.current)
    ) {
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
      if (!readerTookOver.current) scrollTo(late);
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

/** Scroll `card` into view again whenever an ancestor resizes, until the
 *  wait runs out; returns the stop. A resize that leaves the card where it
 *  was still counts: the page may only now be tall enough to bring it up. */
function keepInView(card: HTMLElement): () => void {
  const resizes = new ResizeObserver(() => card.scrollIntoView());
  for (let el = card.parentElement; el; el = el.parentElement) {
    resizes.observe(el);
  }
  const timer = window.setTimeout(
    () => resizes.disconnect(),
    LATE_CARD_WAIT_MS,
  );
  return () => {
    resizes.disconnect();
    window.clearTimeout(timer);
  };
}
