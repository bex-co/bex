import { afterEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { useRef } from "react";
import { useOpenDocked } from "../use-open-docked";

function Frame() {
  const ref = useRef<HTMLDivElement>(null);
  useOpenDocked(ref);
  return (
    <div ref={ref} data-log-frame="">
      <div data-log-dock="" />
    </div>
  );
}

// A page that shows 600px, 1000px of content: the tab's frame (its filter
// bar first) 296px down the page and 500px tall, then 204px of the page's
// own padding and footer below it.
const SHOWN = 600;
const CONTENT = 1000;
const FRAME_TOP = 296;
const FRAME_HEIGHT = 500;

function renderInScroller() {
  const scroller = document.createElement("div");
  scroller.setAttribute("data-page-scroll", "");
  Object.defineProperty(scroller, "clientHeight", { value: SHOWN });
  Object.defineProperty(scroller, "scrollHeight", { value: CONTENT });
  document.body.append(scroller);
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
    function (this: HTMLElement) {
      return this.hasAttribute("data-log-frame")
        ? ({ top: FRAME_TOP, bottom: FRAME_TOP + FRAME_HEIGHT } as DOMRect)
        : ({ top: 0, bottom: SHOWN } as DOMRect);
    },
  );
  render(<Frame />, { container: scroller });
  return {
    scroller,
    frame: scroller.querySelector<HTMLElement>("[data-log-frame]")!,
  };
}

function phone(matches: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({ matches }) as MediaQueryList),
  );
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  document.body.innerHTML = "";
});

describe("useOpenDocked", () => {
  it("opens a phone's tab scrolled to its docked filter bar", () => {
    phone(true);
    const { scroller, frame } = renderInScroller();
    expect(scroller.scrollTop).toBe(FRAME_TOP);
    // Tall enough that the page's end is that docked view, and no taller:
    // a screen, less the page's own space under the frame.
    const below = CONTENT - (FRAME_TOP + FRAME_HEIGHT);
    expect(frame.style.minHeight).toBe(`${SHOWN - below}px`);
  });

  it("leaves a wider screen at the top of the page", () => {
    phone(false);
    const { scroller, frame } = renderInScroller();
    expect(scroller.scrollTop).toBe(0);
    expect(frame.style.minHeight).toBe("");
  });
});
