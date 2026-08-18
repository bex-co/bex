import { describe, it, expect, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { LogLineList } from "../log-line-list";
import { needsAnsiParse, parseAnsi } from "../../lib/ansi";
import type { LogLine } from "../../types";
import {
  scrollViewport,
  setupVirtualGeometry,
  VIRTUAL_VIEWPORT_HEIGHT,
} from "@/test/virtual-geometry";

function line(over: Partial<LogLine> = {}): LogLine {
  const message = over.message ?? "hello world";
  return {
    key: over.key ?? "k",
    timestamp: "2026-07-05T10:36:01.709Z",
    time: "10:36:01",
    instance: "bv612",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    ...over,
    message,
    // Mirror ingest (makeLogLine): ANSI parsed once, so the list reads
    // `line.spans` (w9/m63 t001).
    spans: needsAnsiParse(message) ? parseAnsi(message) : null,
  };
}

// A buffer of n app lines with distinct keys/messages.
function buffer(n: number): LogLine[] {
  return Array.from({ length: n }, (_, i) =>
    line({ key: `k${i}`, message: `line ${i}` }),
  );
}

function viewportOf(container: HTMLElement): HTMLElement {
  const el = container.querySelector<HTMLElement>("[data-log-viewport]");
  if (!el) throw new Error("log viewport not found");
  return el;
}

function renderedRowCount(container: HTMLElement): number {
  return container.querySelectorAll("[data-index]").length;
}

async function settleVirtualScroll(): Promise<void> {
  // TanStack Virtual debounces its synthetic scroll-end notification by 150ms.
  // Let that callback run while jsdom and the mounted component still exist;
  // otherwise a fast test-file teardown can leave it firing against a deleted
  // `window`, which Vitest reports as an unhandled error after every assertion
  // has already passed.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 175));
  });
}

describe("LogLineList request-line rendering (w5/008)", () => {
  setupVirtualGeometry();

  it("shows method + status chips for a request line", () => {
    render(
      <LogLineList
        lines={[
          line({
            key: "r1",
            type: "request",
            method: "GET",
            statusCode: "200",
            message: '{"RequestPath":"/health"}',
          }),
        ]}
      />,
    );
    expect(screen.getByText("GET")).toBeInTheDocument();
    expect(screen.getByText("200")).toBeInTheDocument();
    // The message is still rendered (honest — no data hidden).
    expect(screen.getByText('{"RequestPath":"/health"}')).toBeInTheDocument();
  });

  it("does not render method/status chips for an app line", () => {
    render(
      <LogLineList
        lines={[line({ key: "a1", type: "app", message: "starting up" })]}
      />,
    );
    expect(screen.getByText("starting up")).toBeInTheDocument();
    expect(screen.queryByText("GET")).not.toBeInTheDocument();
    expect(screen.queryByText("200")).not.toBeInTheDocument();
  });

  it("interprets a build line's ANSI instead of leaking the parameter tail", () => {
    const esc = "\u001b";
    const { container } = render(
      <LogLineList
        lines={[
          line({
            key: "b1",
            type: "build",
            message: `#11 94.34 ${esc}[2m│${esc}[22m   ${esc}[33m^--${esc}[39m oops`,
          }),
        ]}
      />,
    );

    // The reader sees the text, never `[2m` / `[22m` / `[33m`.
    expect(container.textContent).toContain("#11 94.34 │   ^-- oops");
    expect(container.textContent).not.toContain("[2m");
    expect(container.textContent).not.toContain("[22m");
    expect(container.textContent).not.toContain("[33m");
    expect(container.textContent).not.toContain(esc);

    expect(screen.getByText("│").className).toContain("opacity-70");
    expect(screen.getByText("^--").className).toContain("text-amber-700");
  });

  it("leaves a plain line as a single unstyled text node", () => {
    render(<LogLineList lines={[line({ message: "no escapes here" })]} />);
    const message = screen.getByText("no escapes here");
    expect(message.tagName).toBe("SPAN");
    expect(message.querySelector("span")).toBeNull();
  });

  it("shows a short clickable pod slug and filters with the full instance", () => {
    const onInstanceFilter = vi.fn();
    const instance = "hello-go-6f7d8f9c4b-bv612";
    render(
      <LogLineList
        lines={[line({ instance })]}
        onInstanceFilter={onInstanceFilter}
      />,
    );

    const instanceButton = screen.getByRole("button", {
      name: `Filter logs by instance ${instance}`,
    });
    expect(instanceButton).toHaveTextContent("[bv612]");
    expect(instanceButton).toHaveAttribute("title", instance);

    fireEvent.click(instanceButton);
    expect(onInstanceFilter).toHaveBeenCalledWith(instance);
  });
});

describe("LogLineList virtualization (w9/m83)", () => {
  setupVirtualGeometry();

  it("renders only the visible window of a 1,000-line buffer, not every row", () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);

    const rows = renderedRowCount(container);
    // A realistic window plus overscan — on the order of viewport/rowHeight,
    // nowhere near 1,000. The exact number depends on overscan; assert the
    // range so the test survives an overscan tweak but fails if virtualization
    // regresses (whole buffer) or renders nothing (the m63 0-height failure).
    expect(rows).toBeGreaterThan(0);
    expect(rows).toBeLessThan(120);
  });

  it("keeps the DOM bounded as the buffer grows an order of magnitude", () => {
    const { container: small } = render(<LogLineList lines={buffer(100)} />);
    const { container: large } = render(<LogLineList lines={buffer(1000)} />);

    // 10× the lines must not mean ~10× the rows in the DOM.
    const smallRows = renderedRowCount(small);
    const largeRows = renderedRowCount(large);
    expect(largeRows).toBeLessThan(smallRows * 2);
  });

  it("renders the first lines of the buffer at rest (top of the window)", () => {
    render(<LogLineList lines={buffer(1000)} />);
    // The window starts at the top (scrollTop 0), so early lines are present
    // and far-down lines are not in the DOM.
    expect(screen.getByText("line 0")).toBeInTheDocument();
    expect(screen.queryByText("line 900")).not.toBeInTheDocument();
  });
});

describe("LogLineList follow / pin (w9/m83)", () => {
  setupVirtualGeometry();

  it("starts pinned (no jump-to-latest affordance)", () => {
    render(<LogLineList lines={buffer(1000)} />);
    expect(
      screen.queryByRole("button", { name: /jump to latest/i }),
    ).not.toBeInTheDocument();
  });

  it("scrolling up releases the pin and surfaces jump-to-latest", async () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    const viewport = viewportOf(container);

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });

    expect(
      screen.getByRole("button", { name: /jump to latest/i }),
    ).toBeInTheDocument();

    await settleVirtualScroll();
  });

  it("returning to the bottom re-pins and hides jump-to-latest", async () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    const viewport = viewportOf(container);

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    expect(
      screen.getByRole("button", { name: /jump to latest/i }),
    ).toBeInTheDocument();

    // Scroll to the bottom: distance from bottom is within PIN_THRESHOLD.
    scrollViewport(viewport, {
      scrollTop: 5000 - VIRTUAL_VIEWPORT_HEIGHT,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    expect(
      screen.queryByRole("button", { name: /jump to latest/i }),
    ).not.toBeInTheDocument();

    await settleVirtualScroll();
  });

  it("clicking jump-to-latest re-pins", async () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    const viewport = viewportOf(container);

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    const jump = screen.getByRole("button", { name: /jump to latest/i });
    fireEvent.click(jump);

    expect(
      screen.queryByRole("button", { name: /jump to latest/i }),
    ).not.toBeInTheDocument();

    await settleVirtualScroll();
  });
});

describe("LogLineList wrap / nowrap (w9/m83)", () => {
  setupVirtualGeometry();

  it("wraps long lines by default (whitespace-pre-wrap rows)", () => {
    const { container } = render(<LogLineList lines={buffer(10)} />);
    const row = container.querySelector<HTMLElement>("[data-index]");
    expect(row?.className).toContain("whitespace-pre-wrap");
    expect(row?.className).not.toContain("whitespace-pre ");
  });

  it("uses single-line rows in nowrap mode", () => {
    const { container } = render(
      <LogLineList lines={buffer(10)} wrap={false} />,
    );
    const row = container.querySelector<HTMLElement>("[data-index]");
    expect(row?.className).toContain("whitespace-pre");
    expect(row?.className).not.toContain("whitespace-pre-wrap");
    // nowrap keeps the content sizer intrinsically wide for horizontal scroll.
    const sizer = viewportOf(container).firstElementChild as HTMLElement;
    expect(sizer.className).toContain("w-max");
  });
});

describe("LogLineList text selection tradeoff (w9/m83)", () => {
  setupVirtualGeometry();

  it("keeps on-screen rows selectable (their text is in the DOM)", () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    // The documented tradeoff: only the on-screen window is in the DOM, so its
    // text is selectable/copyable; scrolled-away rows are not present (that is
    // the cost, asserted by their absence in the virtualization suite).
    const firstRow = container.querySelector<HTMLElement>(
      '[data-index="0"]',
    ) as HTMLElement;
    expect(within(firstRow).getByText("line 0")).toBeInTheDocument();
  });

  it("does not keep far-offscreen rows in the DOM (the copy tradeoff)", () => {
    render(<LogLineList lines={buffer(1000)} />);
    expect(screen.queryByText("line 999")).not.toBeInTheDocument();
  });
});
