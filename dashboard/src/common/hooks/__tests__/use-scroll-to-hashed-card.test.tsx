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

  // w3/m707: cards above the target change height once their own reads land
  // (Billing's plan and payment cards at 390px moved `#free-usage` 220px up).
  describe("while the page above it settles", () => {
    let resized: () => void = () => {};
    let top = 0;
    const realObserver = global.ResizeObserver;

    beforeEach(() => {
      resized = () => {};
      top = 0;
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
      vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(
        () => ({ top }) as DOMRect,
      );
    });

    afterEach(() => {
      global.ResizeObserver = realObserver;
      vi.restoreAllMocks();
    });

    it("scrolls the card back into view when the page moves it", async () => {
      await renderAt("/#early");
      scrolled.length = 0;

      top = -220;
      resized();
      expect(scrolled).toEqual(["early"]);
    });

    it("keeps holding through React's development double run of effects", async () => {
      await renderAt("/#early", true);
      scrolled.length = 0;

      top = -220;
      resized();
      expect(scrolled).toEqual(["early"]);
    });

    it("leaves a resize that did not move the card alone", async () => {
      await renderAt("/#early");
      scrolled.length = 0;

      resized();
      expect(scrolled).toEqual([]);
    });

    it("hands the scroll position back once the reader scrolls", async () => {
      await renderAt("/#early");
      scrolled.length = 0;

      window.dispatchEvent(new Event("wheel"));
      top = -220;
      resized();
      expect(scrolled).toEqual([]);
    });
  });
});
