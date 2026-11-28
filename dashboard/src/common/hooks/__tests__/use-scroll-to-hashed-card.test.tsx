import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { StrictMode, useState } from "react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRouter,
} from "@tanstack/react-router";
import { useScrollToHashedCard } from "@/common/hooks/use-scroll-to-hashed-card";

// w3/m656: Render's almost-full warning links Plan's `#storage`, a control
// inside a lazily loaded section that mounts after the page does. The hook
// waits for such a card instead of giving up on the first render.

let showLate: () => void = () => {};

function Page() {
  const [late, setLate] = useState(false);
  showLate = () => setLate(true);
  useScrollToHashedCard("/page", true);
  return (
    <>
      <section id="early" />
      {late ? <div id="storage" /> : null}
    </>
  );
}

async function renderAt(href: string, strict = false) {
  const router = createRouter({
    routeTree: createRootRoute({ component: Page }),
    history: createMemoryHistory({ initialEntries: [href] }),
  });
  await router.load();
  const app = <RouterProvider router={router} />;
  render(strict ? <StrictMode>{app}</StrictMode> : app);
}

const scrolled: string[] = [];

beforeEach(() => {
  scrolled.length = 0;
  Element.prototype.scrollIntoView = vi.fn(function (this: Element) {
    scrolled.push(this.id);
  });
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("useScrollToHashedCard", () => {
  it("scrolls to a card already on the page", async () => {
    await renderAt("/#early");
    // The router scrolls to it too.
    expect(new Set(scrolled)).toEqual(new Set(["early"]));
  });

  it("waits for a card that mounts after the page", async () => {
    await renderAt("/#storage");
    expect(scrolled).toEqual([]);

    // Neither the router nor a first look finds it; the hook keeps looking.
    await act(async () => showLate());
    await vi.waitFor(() => expect(scrolled).toEqual(["storage"]));
  });

  it("does nothing without a hash", async () => {
    await renderAt("/");
    await act(async () => showLate());
    expect(scrolled).toEqual([]);
  });

  // w3/120: a reader who scrolls while the card is still loading keeps
  // their place, as a browser stops chasing a fragment once they scroll.
  it("leaves the page where the reader scrolled it before the card came", async () => {
    await renderAt("/#storage");

    window.dispatchEvent(new Event("wheel"));
    await act(async () => showLate());
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(scrolled).toEqual([]);
  });

  // w3/m707: cards above the target change height once their own reads land
  // (Billing's plan and payment cards at 390px moved `#free-usage` 220px up).
  // w3/120: on a Postgres database's Info page, Access control's ready card
  // outgrows its 240px reserve above `#sql-console` and `#danger-zone`.
  describe("while the page around it settles", () => {
    // One scroller in one dimension: the card's offset in the page, how far
    // the page is scrolled and how far it can be. scrollIntoView brings the
    // card to the top, or as near as the page's end allows.
    const page = { card: 0, scrollTop: 0, maxScroll: 0 };
    const cardTop = () => page.card - page.scrollTop;
    let resized: () => void = () => {};
    const realObserver = global.ResizeObserver;

    /** Cards grow by `above` px over the card and `below` px under it. The
     *  browser's scroll anchoring holds the card where it is on screen when
     *  `anchored`, else the content above it, which pushes the card down. */
    function grow(above: number, below: number, anchored = false) {
      page.card += above;
      page.maxScroll += above + below;
      if (anchored) page.scrollTop += above;
      resized();
    }

    beforeEach(() => {
      // The page is too short at first: the card ends 300px down, at its end.
      Object.assign(page, { card: 1000, scrollTop: 0, maxScroll: 700 });
      resized = () => {};
      global.ResizeObserver = class {
        constructor(callback: ResizeObserverCallback) {
          resized = () => callback([], this as unknown as ResizeObserver);
        }
        observe() {}
        unobserve() {}
        disconnect() {
          resized = () => {};
        }
      } as unknown as typeof ResizeObserver;
      Element.prototype.scrollIntoView = vi.fn(function (this: Element) {
        scrolled.push(this.id);
        page.scrollTop = Math.min(page.card, page.maxScroll);
      });
      vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(
        () => ({ top: cardTop() }) as DOMRect,
      );
    });

    afterEach(() => {
      global.ResizeObserver = realObserver;
      vi.restoreAllMocks();
    });

    it("scrolls the card back up when a card above it grows", async () => {
      await renderAt("/#early");
      expect(cardTop()).toBe(300);

      grow(352, 800);
      expect(cardTop()).toBe(0);
    });

    it("brings the card up once the page has grown tall enough", async () => {
      await renderAt("/#early");

      // Where it was, but the page can now bring it to the top.
      grow(352, 800, true);
      expect(cardTop()).toBe(0);
    });

    it("keeps holding through React's development double run of effects", async () => {
      await renderAt("/#early", true);

      grow(352, 800);
      expect(cardTop()).toBe(0);
    });

    it("hands the scroll position back once the reader scrolls", async () => {
      await renderAt("/#early");
      scrolled.length = 0;

      window.dispatchEvent(new Event("wheel"));
      grow(352, 800);
      expect(scrolled).toEqual([]);
      expect(cardTop()).toBe(652);
    });
  });
});
