import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { LogLineList } from "../log-line-list";
import type { LogLine } from "../../types";
import { scrollViewport, setupVirtualGeometry } from "@/test/virtual-geometry";

/**
 * w4/m228 (a896): toggling wrap or timestamps changes row heights without
 * changing the records, and the reader lost its row (ROW=128 → 134 on
 * mobile). The browser remeasures through ResizeObserver, reporting only rows
 * whose height changed; this file installs one it can trigger. Wrapped rows
 * are twice as tall, and neither height equals the list's 24px estimate.
 */
const WRAPPED = 40;
const ONE_LINE = 20;
setupVirtualGeometry({
  rowHeight: (row) =>
    row.className.includes("whitespace-pre-wrap") ? WRAPPED : ONE_LINE,
});

const observers: Array<{
  callback: ResizeObserverCallback;
  targets: Map<Element, number | undefined>;
}> = [];
let originalObserver: typeof ResizeObserver;
beforeEach(() => {
  originalObserver = globalThis.ResizeObserver;
  globalThis.ResizeObserver = class {
    private entry: {
      callback: ResizeObserverCallback;
      targets: Map<Element, number | undefined>;
    };
    constructor(callback: ResizeObserverCallback) {
      this.entry = { callback, targets: new Map() };
      observers.push(this.entry);
    }
    observe(target: Element) {
      this.entry.targets.set(target, undefined);
    }
    unobserve(target: Element) {
      this.entry.targets.delete(target);
    }
    disconnect() {
      this.entry.targets.clear();
    }
  } as unknown as typeof ResizeObserver;
});
afterEach(() => {
  globalThis.ResizeObserver = originalObserver;
  observers.length = 0;
});

/**
 * What the browser does after layout: report the rows whose height changed,
 * and fire the scroll event a programmatic scrollTop change produces (jsdom
 * fires none), so the virtualizer renders the window at the new offset.
 */
async function remeasure() {
  await act(async () => {
    viewport()?.dispatchEvent(new Event("scroll"));
    for (const { callback, targets } of observers) {
      const changed: ResizeObserverEntry[] = [];
      for (const [target, last] of targets) {
        const height = (target as HTMLElement).offsetHeight;
        if (target.isConnected && height !== last) {
          targets.set(target, height);
          changed.push({ target } as unknown as ResizeObserverEntry);
        }
      }
      if (changed.length) callback(changed, {} as ResizeObserver);
    }
  });
}

const makeLines = (count: number): LogLine[] =>
  Array.from({ length: count }, (_, i) => ({
    key: `line-${i}`,
    timestamp: "2026-10-17T00:00:00Z",
    time: "00:00:00",
    instance: "",
    message: `ROW=${i}`,
    spans: null,
    type: "app",
    level: "",
    method: "",
    statusCode: "",
  }));
const lines = makeLines(300);

const viewport = () =>
  document.querySelector<HTMLElement>("[data-log-viewport]")!;

/**
 * The row the reader sees at the top, from the rendered DOM: the spacer that
 * stands in for the rows above, then each rendered row's height.
 */
function rowAtTop(): { row: number; offset: number } {
  const rows = [...viewport().querySelectorAll<HTMLElement>("[data-index]")];
  const spacer = rows[0].previousElementSibling as HTMLElement;
  let start = parseFloat(spacer.style.height);
  const top = viewport().scrollTop;
  for (const row of rows) {
    const end = start + row.offsetHeight;
    if (end > top) {
      return { row: Number(row.dataset.index), offset: top - start };
    }
    start = end;
  }
  throw new Error("no rendered row at the top");
}

/** A reader scroll, then the browser's remeasure, until the window settles. */
async function scrollTo(scrollTop: number) {
  for (let pass = 0; pass < 3; pass += 1) {
    act(() =>
      scrollViewport(viewport(), {
        scrollTop,
        scrollHeight: 100_000,
        clientHeight: 520,
      }),
    );
    await remeasure();
  }
}

const settle = () => act(() => new Promise((r) => setTimeout(r, 300)));
const jump = () => screen.queryByRole("button", { name: /latest/i });

describe("LogLineList display reflow (w4/m228)", () => {
  it("keeps the reader's row when wrapping is turned on", async () => {
    const view = render(<LogLineList lines={lines} wrap={false} />);
    await remeasure();
    await scrollTo(2100);
    const before = rowAtTop();

    view.rerender(<LogLineList lines={lines} wrap />);
    await remeasure();
    await remeasure();
    await settle();
    expect(rowAtTop()).toEqual(before);
    // The reflow neither re-pinned nor jumped the reader to the end.
    expect(jump()).toBeInTheDocument();
  });

  it("keeps the row when timestamps change and row heights do not", async () => {
    const view = render(<LogLineList lines={lines} wrap />);
    await remeasure();
    await scrollTo(4000);
    const before = rowAtTop();
    // No row changes height, so the browser reports nothing.
    view.rerender(<LogLineList lines={lines} wrap showTimestamps={false} />);
    await remeasure();
    await settle();
    expect(rowAtTop()).toEqual(before);
  });

  it("clamps the offset into a row that became shorter, and keeps it after an append", async () => {
    const view = render(<LogLineList lines={lines} wrap />);
    await remeasure();
    // Near the top every row above has been measured, so positions are exact:
    // ROW=9, 30px into its 40px.
    const INSET = 12;
    await scrollTo(INSET + 9 * WRAPPED + 30);
    const row = 9;
    expect(rowAtTop()).toEqual({ row, offset: 30 });

    view.rerender(<LogLineList lines={lines} wrap={false} />);
    await remeasure();
    await settle();
    expect(rowAtTop()).toEqual({ row, offset: ONE_LINE - 1 });

    // A new line arrives after the reflow settled: the reader stays put.
    view.rerender(<LogLineList lines={makeLines(301)} wrap={false} />);
    await remeasure();
    expect(rowAtTop()).toEqual({ row, offset: ONE_LINE - 1 });
  });

  it("stops settling within its cap while lines keep arriving", async () => {
    const view = render(<LogLineList lines={lines} wrap={false} />);
    await remeasure();
    await scrollTo(2100);
    view.rerender(<LogLineList lines={lines} wrap />);
    // A busy live tail: a new line every 100ms for 1.5s.
    for (let n = 301; n <= 315; n += 1) {
      view.rerender(<LogLineList lines={makeLines(n)} wrap />);
      await remeasure();
      await act(() => new Promise((r) => setTimeout(r, 100)));
    }
    // Settled: a reader scroll to the end is seen and pins again.
    act(() =>
      scrollViewport(viewport(), {
        scrollTop: 100_000 - 520,
        scrollHeight: 100_000,
        clientHeight: 520,
      }),
    );
    expect(jump()).toBeNull();
  });

  it("keeps a pinned reader following the newest line", async () => {
    const view = render(<LogLineList lines={lines} wrap />);
    await remeasure();
    view.rerender(
      <LogLineList lines={lines} wrap={false} showTimestamps={false} />,
    );
    await remeasure();
    await settle();
    expect(jump()).toBeNull();
  });
});
