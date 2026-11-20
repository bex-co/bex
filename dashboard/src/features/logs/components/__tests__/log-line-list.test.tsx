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
    instance: "bv612",
    resource: "",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
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

  it("shows a request line as Render does: chips, host and path, then the summary (w4/m325)", () => {
    const summary =
      'clientIP="198.51.100.3" requestID="8ebfa3c3-8929-4885" responseTimeMS=8 responseBytes=311 userAgent="curl/8.7.1"';
    const { container } = render(
      <LogLineList
        lines={[
          line({
            key: "r1",
            type: "request",
            method: "GET",
            statusCode: "503",
            host: "qa.onbex.co",
            path: "/qa-err",
            message: summary,
          }),
        ]}
      />,
    );
    expect(screen.getByText("GET")).toBeInTheDocument();
    expect(screen.getByText("503")).toBeInTheDocument();
    expect(container.textContent).toContain(
      `GET503qa.onbex.co/qa-err ${summary}`,
    );
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

  it("shows a short clickable slug and filters with the full public instance id", () => {
    const onInstanceFilter = vi.fn();
    // Backend-provided public instance id (srv-<20>-<20>); presentation shortens
    // the opaque suffix, filtering always uses the full value.
    const instance = "srv-c185th5c2rvvnhbfiltg-ohoplujmv928mnmaj7j7";
    render(
      <LogLineList
        lines={[line({ instance })]}
        onInstanceFilter={onInstanceFilter}
      />,
    );

    const instanceButton = screen.getByRole("button", {
      name: `Filter logs by instance ${instance}`,
    });
    expect(instanceButton).toHaveTextContent("[ohopl]");
    expect(instanceButton).toHaveAttribute("title", instance);

    fireEvent.click(instanceButton);
    expect(onInstanceFilter).toHaveBeenCalledWith(instance);
  });

  // w21/019: the faded `/70` token measured 3.13:1 in the light theme.
  it("draws the instance id at full muted-foreground contrast", () => {
    render(<LogLineList lines={[line()]} onInstanceFilter={vi.fn()} />);
    const button = screen.getByRole("button", {
      name: /Filter logs by instance/,
    });
    expect(button.className).toContain("text-muted-foreground");
    expect(button.className).not.toContain("text-muted-foreground/");
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

  it("starts pinned (no Return to Bottom)", () => {
    render(<LogLineList lines={buffer(1000)} />);
    expect(
      screen.queryByRole("button", { name: "Return to Bottom" }),
    ).not.toBeInTheDocument();
  });

  it("scrolling up releases the pin and surfaces Render's icon-only Return to Bottom (w3/m528)", async () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    const viewport = viewportOf(container);

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });

    const back = screen.getByRole("button", { name: "Return to Bottom" });
    // Icon-only, at the panel's bottom right, as Render's
    // LogViewerReturnButton; "Jump to latest" is gone.
    expect(back).toHaveTextContent("");
    expect(back).toHaveAttribute("title", "Return to Bottom");
    expect(back.className).toContain("right-6");
    expect(back.className).toContain("bottom-4");
    expect(screen.queryByText(/jump to latest/i)).toBeNull();

    await settleVirtualScroll();
  });

  it("returning to the bottom re-pins and hides Return to Bottom", async () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    const viewport = viewportOf(container);

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();

    // Scroll to the bottom: distance from bottom is within PIN_THRESHOLD.
    scrollViewport(viewport, {
      scrollTop: 5000 - VIRTUAL_VIEWPORT_HEIGHT,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    expect(
      screen.queryByRole("button", { name: "Return to Bottom" }),
    ).not.toBeInTheDocument();

    await settleVirtualScroll();
  });

  it("clicking Return to Bottom re-pins", async () => {
    const { container } = render(<LogLineList lines={buffer(1000)} />);
    const viewport = viewportOf(container);

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 5000,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    const jump = screen.getByRole("button", { name: "Return to Bottom" });
    fireEvent.click(jump);

    expect(
      screen.queryByRole("button", { name: "Return to Bottom" }),
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

describe("LogLineList reader anchor during refresh", () => {
  setupVirtualGeometry();

  it("retains the same row through head appends, older prepends, and expiry", async () => {
    const rows = buffer(140);
    const { container, rerender } = render(<LogLineList lines={rows} />);
    const viewport = viewportOf(container);
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 12 + 50 * 24 + 7,
        scrollHeight: 140 * 24 + 24,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    const before = viewport.scrollTop;
    Object.defineProperty(viewport, "scrollHeight", {
      configurable: true,
      value: 150 * 24 + 24,
    });
    const appended = [
      ...rows,
      ...buffer(10).map((row) => ({ ...row, key: `new-${row.key}` })),
    ];
    rerender(<LogLineList lines={appended} />);
    expect(viewportOf(container)).toBe(viewport);
    expect(viewport.scrollTop).toBe(before);
    const older = buffer(20).map((row) => ({
      ...row,
      key: `older-${row.key}`,
    }));
    Object.defineProperty(viewport, "scrollHeight", {
      configurable: true,
      value: 170 * 24 + 24,
    });
    rerender(<LogLineList lines={[...older, ...appended]} />);
    expect(viewport.scrollTop).toBe(before + 20 * 24);
    Object.defineProperty(viewport, "scrollHeight", {
      configurable: true,
      value: 130 * 24 + 24,
    });
    rerender(<LogLineList lines={appended.slice(20)} />);
    expect(viewport.scrollTop).toBe(before - 20 * 24);
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();
    await settleVirtualScroll();
  });
});

describe("LogLineList ended (a finished deploy's log)", () => {
  setupVirtualGeometry();

  // The harness has no layout, so the panel keeps its 520px fixed height and
  // every row measures 24px; the list adds a 12px inset above and below, and
  // an ended log its 24px end line.
  const heightOf = (container: HTMLElement) =>
    viewportOf(container).style.height;

  it("closes a finished log with an end line", () => {
    render(<LogLineList lines={buffer(3)} ended />);
    expect(screen.getByText("End of log")).toBeInTheDocument();
  });

  it("has no end line while the log can still grow", () => {
    render(<LogLineList lines={buffer(3)} />);
    expect(screen.queryByText("End of log")).not.toBeInTheDocument();
  });

  it("never shrinks the panel under a short log, so its placeholder holds", () => {
    const { container } = render(<LogLineList lines={buffer(3)} ended />);
    expect(heightOf(container)).toBe("520px");
  });

  it("grows the panel to show a log that only just overflows it whole", () => {
    // 24 rows + 24px inset + 24px end line + 2px border = 626px, within
    // 1.25 × 520px.
    const { container } = render(<LogLineList lines={buffer(24)} ended />);
    expect(heightOf(container)).toBe("626px");
  });

  it("keeps a live log's panel at its height as lines arrive", () => {
    const { container } = render(<LogLineList lines={buffer(24)} />);
    expect(heightOf(container)).toBe("520px");
  });

  it("scrolls a long finished log inside the panel's own height", () => {
    const { container } = render(<LogLineList lines={buffer(40)} ended />);
    expect(heightOf(container)).toBe("520px");
  });
});

describe("LogLineList keyboard row cursor", () => {
  setupVirtualGeometry();

  it("moves a row cursor with the arrow keys and filters by its instance on Enter", () => {
    const onInstanceFilter = vi.fn();
    const lines = [
      line({ key: "a", message: "first", instance: "web-1" }),
      line({ key: "b", message: "second", instance: "web-2" }),
    ];
    const { container } = render(
      <LogLineList lines={lines} onInstanceFilter={onInstanceFilter} />,
    );
    const viewport = viewportOf(container);
    // The region names its keys for a screen reader.
    expect(viewport).toHaveAccessibleDescription(
      "Use the up and down arrow keys to move between lines, Enter to show only that line's instance, and Shift+F10 to open its menu.",
    );

    act(() => viewport.focus());
    fireEvent.keyDown(viewport, { key: "ArrowDown" });
    fireEvent.keyDown(viewport, { key: "ArrowDown" });
    const rows = container.querySelectorAll("[data-index]");
    expect(rows[1]).toHaveClass("outline-ring");
    expect(rows[0]).not.toHaveClass("outline-ring");

    fireEvent.keyDown(viewport, { key: "Enter" });
    expect(onInstanceFilter).toHaveBeenCalledWith("web-2");

    fireEvent.keyDown(viewport, { key: "Escape" });
    expect(rows[1]).not.toHaveClass("outline-ring");
  });
});
