import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { useState } from "react";
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

async function renderAt(href: string) {
  const router = createRouter({
    routeTree: createRootRoute({ component: Page }),
    history: createMemoryHistory({ initialEntries: [href] }),
  });
  await router.load();
  render(<RouterProvider router={router} />);
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
});
