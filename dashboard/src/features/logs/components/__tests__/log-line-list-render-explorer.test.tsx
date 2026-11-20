import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Toaster } from "sonner";
import { LogLineList, type LogLineListHandle } from "../log-line-list";
import type { LogRowMenuItems } from "../log-row-menu";
import { heldLinesCount } from "../../lib/log-rows";
import type { LogLine } from "../../types";
import {
  scrollViewport,
  setupVirtualGeometry,
  VIRTUAL_VIEWPORT_HEIGHT,
} from "@/test/virtual-geometry";

// Render's explorer pieces w3/m509 left out (w3/m528), read from Render's
// dashboard bundle: the held-lines button, the row menu, View in context's
// marked line and the paused view's "New logs in Live Tail".

const BASE = Date.parse("2026-11-20T10:00:00.000Z");

function line(i: number, over: Partial<LogLine> = {}): LogLine {
  return {
    key: `k${i}`,
    // One second apart, oldest first, as the explorers hand them over.
    timestamp: new Date(BASE + i * 1000).toISOString(),
    instance: "",
    message: `line ${i}`,
    spans: null,
    resource: "",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
    ...over,
  };
}

const range = (from: number, to: number) =>
  Array.from({ length: to - from }, (_, i) => line(from + i));

function viewportOf(container: HTMLElement): HTMLElement {
  return container.querySelector<HTMLElement>("[data-log-viewport]")!;
}

/** Scroll a list of `rows` rows to its top: away from the newest line. */
function scrollAway(viewport: HTMLElement, rows: number) {
  act(() =>
    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: rows * 24 + 24,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    }),
  );
}

async function settle() {
  // TanStack Virtual's scroll-end timer (see log-line-list.test.tsx).
  await act(() => new Promise((resolve) => setTimeout(resolve, 175)));
}

const heldButton = (name: string | RegExp) =>
  screen.queryByRole("button", { name });

describe("held lines while scrolled away from a live tail (w3/m528 t003)", () => {
  setupVirtualGeometry();

  it("words the count as Render's BufferedLogsButton does", () => {
    expect(heldLinesCount(1)).toBeNull();
    expect(heldLinesCount(9)).toBeNull();
    expect(heldLinesCount(10)).toBe(10);
    expect(heldLinesCount(11)).toBe(15);
    expect(heldLinesCount(23)).toBe(25);
    expect(heldLinesCount(100)).toBe(100);
    expect(heldLinesCount(140)).toBe(100);
  });

  it("holds new lines under a reader scrolled away, counts them, and releases them on a click", async () => {
    const handle = createRef<LogLineListHandle>();
    const list = (to: number) => (
      <LogLineList
        lines={range(0, to)}
        holdWhileAway
        handleRef={handle}
        showTimestamps={false}
      />
    );
    const { container, rerender } = render(list(60));
    const viewport = viewportOf(container);
    scrollAway(viewport, 60);
    const before = viewport.scrollTop;

    // Nine lines arrive: held, not added under the reader.
    rerender(list(69));
    expect(heldButton("New logs")).toBeInTheDocument();
    expect(handle.current!.text().split("\n")).toHaveLength(60);
    expect(handle.current!.text()).not.toContain("line 60");
    expect(viewport.scrollTop).toBe(before);
    expect(screen.getByText("line 0")).toBeInTheDocument();

    rerender(list(70));
    expect(heldButton("10+ new logs")).toBeInTheDocument();
    rerender(list(83));
    expect(heldButton("25+ new logs")).toBeInTheDocument();
    rerender(list(200));
    const button = heldButton("100+ new logs")!;
    expect(button.className).toContain("left-1/2");
    expect(button.className).toContain("bottom-4");
    // Return to Bottom shows beside it, at the right edge.
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();

    fireEvent.click(button);
    expect(heldButton(/new logs/)).toBeNull();
    // Every held line is in the list again, in order, and the reader is at
    // the newest.
    expect(handle.current!.text()).toBe(
      range(0, 200)
        .map((l) => `${l.timestamp} ${l.message}`)
        .join("\n"),
    );
    expect(
      screen.queryByRole("button", { name: "Return to Bottom" }),
    ).toBeNull();
    await settle();
  });

  it("releases the held lines when the reader scrolls back to the newest line", async () => {
    const handle = createRef<LogLineListHandle>();
    const { container, rerender } = render(
      <LogLineList lines={range(0, 60)} holdWhileAway handleRef={handle} />,
    );
    const viewport = viewportOf(container);
    scrollAway(viewport, 60);
    rerender(
      <LogLineList lines={range(0, 75)} holdWhileAway handleRef={handle} />,
    );
    expect(heldButton("15+ new logs")).toBeInTheDocument();
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 60 * 24 + 24 - VIRTUAL_VIEWPORT_HEIGHT,
        scrollHeight: 60 * 24 + 24,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    expect(heldButton(/new logs/)).toBeNull();
    expect(handle.current!.text()).toContain("line 74");
    await settle();
  });

  it("still adds an older page at the far edge while it holds", async () => {
    const handle = createRef<LogLineListHandle>();
    const { container, rerender } = render(
      <LogLineList lines={range(10, 60)} holdWhileAway handleRef={handle} />,
    );
    scrollAway(viewportOf(container), 50);
    rerender(
      <LogLineList lines={range(0, 65)} holdWhileAway handleRef={handle} />,
    );
    // The page of ten older lines joins; the five newer ones are held.
    const shown = handle.current!.text();
    expect(shown).toContain("line 0");
    expect(shown).not.toContain("line 60");
    expect(heldButton("New logs")).toBeInTheDocument();
    await settle();
  });

  it("holds at the top in descending order, where the newest line is", async () => {
    const handle = createRef<LogLineListHandle>();
    const list = (to: number) => (
      <LogLineList
        lines={range(0, to)}
        holdWhileAway
        order="desc"
        handleRef={handle}
        showTimestamps={false}
      />
    );
    const { container, rerender } = render(list(60));
    const viewport = viewportOf(container);
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 600,
        scrollHeight: 60 * 24 + 24,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    rerender(list(72));
    const button = heldButton("15+ new logs")!;
    expect(button.className).toContain("top-4");
    // The newest lines would land at the top: none has.
    expect(handle.current!.text().split("\n")[0]).toMatch(/ line 59$/);
    expect(
      screen.getByRole("button", { name: "Return to Top" }),
    ).toBeInTheDocument();
    fireEvent.click(button);
    expect(handle.current!.text().split("\n")[0]).toMatch(/ line 71$/);
    await settle();
  });

  it("never holds a log that is not a live tail (a deploy's build log, a paused view)", async () => {
    const { container, rerender } = render(
      <LogLineList lines={range(0, 60)} />,
    );
    scrollAway(viewportOf(container), 60);
    rerender(<LogLineList lines={range(0, 80)} />);
    expect(heldButton(/new logs/)).toBeNull();
    // The rows past the window are mounted on demand; the list has them all.
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();
    await settle();
  });

  it("shows every line when none of the held view's lines is left (a Shift+L clear)", async () => {
    const { container, rerender } = render(
      <LogLineList lines={range(0, 60)} holdWhileAway />,
    );
    scrollAway(viewportOf(container), 60);
    rerender(<LogLineList lines={range(0, 70)} holdWhileAway />);
    expect(heldButton("10+ new logs")).toBeInTheDocument();
    // Shift+L cuts every line up to the newest, held ones included; what
    // arrives next is not held behind lines that are gone.
    rerender(<LogLineList lines={range(70, 73)} holdWhileAway />);
    expect(heldButton(/new logs/)).toBeNull();
    expect(screen.getByText("line 72")).toBeInTheDocument();
    await settle();
  });
});

describe("the row menu (w3/m528 t005-t007)", () => {
  setupVirtualGeometry();

  const menuButton = () =>
    screen.queryByRole("button", { name: "Log line options" });
  const rowOf = (container: HTMLElement, index: number) =>
    container.querySelector<HTMLElement>(`[data-index="${index}"]`)!;

  it("mounts no menu until a row is hovered, then only on that row, at its top right", async () => {
    const { container, rerender } = render(<LogLineList lines={range(0, 5)} />);
    expect(menuButton()).toBeNull();
    fireEvent.pointerEnter(rowOf(container, 2));
    const button = menuButton()!;
    expect(rowOf(container, 2)).toContainElement(button);
    expect(
      screen.getAllByRole("button", { name: "Log line options" }),
    ).toHaveLength(1);
    // Out of the row's flow, so the row keeps its measured height.
    expect(button.parentElement!.className).toContain("absolute");
    expect(button.parentElement!.className).toContain("top-0");
    expect(rowOf(container, 2).className).toContain("relative");
    // Appending lines leaves the hovered row's menu mounted as it was.
    rerender(<LogLineList lines={range(0, 8)} />);
    expect(menuButton()).toBe(button);
    fireEvent.pointerEnter(rowOf(container, 3));
    expect(rowOf(container, 3)).toContainElement(menuButton());
    fireEvent.pointerLeave(viewportOf(container), { pointerType: "mouse" });
    expect(menuButton()).toBeNull();
  });

  it("copies the line as Render's Copy log line: no time, no instance, and says Copied!", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <>
        <LogLineList
          lines={[
            line(0, {
              instance: "srv-a-0123456789abcdefghij",
              message: "\u001b[31mboot\u001b[39m ok",
              spans: [
                { text: "boot", className: "text-red-600" },
                { text: " ok", className: "" },
              ],
            }),
            line(1, {
              type: "request",
              method: "GET",
              statusCode: "200",
              host: "qa.onbex.co",
              path: "/health",
              message: "responseTimeMS=3",
            }),
          ]}
        />
        <Toaster />
      </>,
    );
    fireEvent.pointerEnter(rowOf(container, 0));
    await user.click(menuButton()!);
    await user.click(screen.getByRole("menuitem", { name: "Copy log line" }));
    // user-event puts its own clipboard in place of the page's.
    expect(await navigator.clipboard.readText()).toBe("boot ok");
    expect(await screen.findAllByText("Copied!")).not.toHaveLength(0);

    fireEvent.pointerEnter(rowOf(container, 1));
    await user.click(menuButton()!);
    await user.click(screen.getByRole("menuitem", { name: "Copy log line" }));
    expect(await navigator.clipboard.readText()).toBe(
      "GET 200 qa.onbex.co/health responseTimeMS=3",
    );
  });

  it("copies a project view line with its resource's name", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <LogLineList
        lines={[line(0, { resource: "srv-api", message: "ready" })]}
        resourceLabels={
          new Map([
            [
              "srv-api",
              {
                id: "srv-api",
                name: "api",
                dotClass: "bg-sky-500",
                href: null,
              },
            ],
          ])
        }
      />,
    );
    fireEvent.pointerEnter(rowOf(container, 0));
    await user.click(menuButton()!);
    await user.click(screen.getByRole("menuitem", { name: "Copy log line" }));
    expect(await navigator.clipboard.readText()).toBe("[api] ready");
  });

  it("disables Copy log line on a blank line, and has no menu at all when nothing else applies", async () => {
    const user = userEvent.setup();
    const items: LogRowMenuItems = {
      contextHref: (l) => `/ctx/${l.key}`,
    };
    const { container, rerender } = render(
      <LogLineList lines={[line(0, { message: "   " })]} />,
    );
    fireEvent.pointerEnter(rowOf(container, 0));
    expect(menuButton()).toBeNull();
    rerender(
      <LogLineList lines={[line(0, { message: "   " })]} rowMenu={items} />,
    );
    await user.click(menuButton()!);
    expect(
      screen.getByRole("menuitem", { name: "Copy log line" }),
    ).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByRole("menuitem", { name: "View in context" }),
    ).toHaveAttribute("href", "/ctx/k0");
  });

  it("opens the keyboard cursor's row menu on Shift+F10, as the hint says, and keeps the cursor", async () => {
    const user = userEvent.setup();
    const { container } = render(<LogLineList lines={range(0, 5)} />);
    const viewport = viewportOf(container);
    expect(viewport).toHaveAccessibleDescription(
      "Use the up and down arrow keys to move between lines, and Shift+F10 to open a line's menu.",
    );
    act(() => viewport.focus());
    fireEvent.keyDown(viewport, { key: "ArrowDown" });
    const active = container.querySelector<HTMLElement>("[data-active]")!;
    // The cursor's row shows the button too, with no pointer on it.
    expect(active).toContainElement(menuButton());
    fireEvent.keyDown(viewport, { key: "F10", shiftKey: true });
    expect(
      await screen.findByRole("menuitem", { name: "Copy log line" }),
    ).toBeInTheDocument();
    expect(container.querySelector("[data-active]")).toBe(active);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menuitem")).toBeNull();
    expect(viewport).toHaveFocus();
    expect(container.querySelector("[data-active]")).toBe(active);
  });

  it("offers SSH into instance as a link on a running instance, disabled with Render's reason otherwise, and not without an instance", async () => {
    const user = userEvent.setup();
    const items: LogRowMenuItems = {
      shell: (l) =>
        l.instance
          ? { href: l.instance === "up" ? `/shell?instance=up` : null }
          : null,
    };
    const { container } = render(
      <LogLineList
        lines={[
          line(0, { instance: "up" }),
          line(1, { instance: "gone" }),
          line(2),
        ]}
        rowMenu={items}
      />,
    );
    fireEvent.pointerEnter(rowOf(container, 0));
    await user.click(menuButton()!);
    const ssh = screen.getByRole("menuitem", { name: "SSH into instance" });
    expect(ssh).toHaveAttribute("href", "/shell?instance=up");
    expect(ssh).toHaveAttribute("target", "_blank");
    await user.keyboard("{Escape}");

    fireEvent.pointerEnter(rowOf(container, 1));
    await user.click(menuButton()!);
    const disabled = screen.getByRole("menuitem", {
      name: "SSH into instance",
    });
    expect(disabled).toHaveAttribute("aria-disabled", "true");
    expect(disabled).not.toHaveAttribute("href");
    expect(disabled).toHaveAccessibleDescription(
      "You can only SSH into running instances. This one is not available.",
    );
    await user.keyboard("{Escape}");

    fireEvent.pointerEnter(rowOf(container, 2));
    await user.click(menuButton()!);
    expect(
      screen.queryByRole("menuitem", { name: "SSH into instance" }),
    ).toBeNull();
  });
});

describe("View in context's marked line (w3/m528 t006)", () => {
  setupVirtualGeometry();
  // jsdom neither scrolls nor lays out: the viewport scrolls where the
  // virtualizer asks, over a list of 100 rows.
  const CONTENT = 100 * 24 + 24;
  // What the viewport's content measures before the virtualizer lays out any
  // row. A browser reports the panel's own height there; most tests skip that
  // phase and report the laid-out height throughout.
  let unlaidContent = CONTENT;
  const saved = {
    scrollTo: Object.getOwnPropertyDescriptor(Element.prototype, "scrollTo"),
    scrollHeight: Object.getOwnPropertyDescriptor(
      Element.prototype,
      "scrollHeight",
    ),
    clientHeight: Object.getOwnPropertyDescriptor(
      Element.prototype,
      "clientHeight",
    ),
  };
  beforeEach(() => {
    unlaidContent = CONTENT;
    Object.defineProperty(Element.prototype, "scrollTo", {
      configurable: true,
      value(this: Element, options: ScrollToOptions) {
        if (options.top !== undefined) this.scrollTop = options.top;
      },
    });
    const viewport = (el: Element) => el.hasAttribute("data-log-viewport");
    Object.defineProperty(Element.prototype, "scrollHeight", {
      configurable: true,
      get(this: Element) {
        if (!viewport(this)) return 0;
        return this.querySelector("[data-index]") ? CONTENT : unlaidContent;
      },
    });
    Object.defineProperty(Element.prototype, "clientHeight", {
      configurable: true,
      get(this: Element) {
        return viewport(this) ? VIRTUAL_VIEWPORT_HEIGHT : 0;
      },
    });
  });
  afterEach(() => {
    for (const [key, descriptor] of Object.entries(saved)) {
      if (descriptor) Object.defineProperty(Element.prototype, key, descriptor);
      else
        delete (Element.prototype as unknown as Record<string, unknown>)[key];
    }
  });

  it("marks the line, opens away from the newest one, and centres it once", async () => {
    const { container, rerender } = render(
      <LogLineList lines={range(0, 100)} markedKey="k40" />,
    );
    const viewport = viewportOf(container);
    // Row 40's middle (12px inset + 40 rows + half a row) at the panel's.
    expect(viewport.scrollTop).toBe(
      12 + 40 * 24 + 12 - VIRTUAL_VIEWPORT_HEIGHT / 2,
    );
    act(() => viewport.dispatchEvent(new Event("scroll")));
    expect(container.querySelector("[data-marked]")).toHaveTextContent(
      "line 40",
    );
    // Not pinned to the newest line: the way back is offered.
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();
    // The reader scrolls; a later render does not pull them back.
    act(() => {
      viewport.scrollTop = 100;
      viewport.dispatchEvent(new Event("scroll"));
    });
    rerender(<LogLineList lines={range(0, 101)} markedKey="k40" />);
    expect(viewport.scrollTop).toBe(100);
    await settle();
  });

  it("reads older pages until the marked line is loaded", () => {
    // Line 30 arrives with 30 rows above it, room to centre it: no more pages.
    const onLoadOlder = vi.fn();
    const { rerender } = render(
      <LogLineList
        lines={range(50, 100)}
        markedKey="k30"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
    rerender(
      <LogLineList
        lines={range(50, 100)}
        markedKey="k30"
        hasMore
        loadingOlder
        onLoadOlder={onLoadOlder}
      />,
    );
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
    rerender(
      <LogLineList
        lines={range(0, 100)}
        markedKey="k30"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
  });

  it("centres the mark once the rows are laid out, not at the newest line", async () => {
    // Found in production (pin 11b1e8f85f06): the first render with lines
    // has no rows laid out, so the panel had nothing to scroll in. The mark's
    // scroll landed at the top, the pin read the reader as at the newest
    // line, and the list jumped there.
    unlaidContent = VIRTUAL_VIEWPORT_HEIGHT;
    const { container } = render(
      <LogLineList lines={range(0, 100)} markedKey="k40" />,
    );
    const viewport = viewportOf(container);
    expect(viewport.scrollTop).toBe(
      12 + 40 * 24 + 12 - VIRTUAL_VIEWPORT_HEIGHT / 2,
    );
    act(() => viewport.dispatchEvent(new Event("scroll")));
    expect(viewport.scrollTop).toBe(
      12 + 40 * 24 + 12 - VIRTUAL_VIEWPORT_HEIGHT / 2,
    );
    expect(container.querySelector("[data-marked]")).toHaveTextContent(
      "line 40",
    );
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();
    await settle();
  });

  // Found in production (pin 820edf28d22d): a closed window logging a line a
  // second puts its mark 600 lines, six pages, from the newest, so the mark
  // was the oldest line of the page that brought it in. Centred at once, it
  // sat on the panel's edge, and the older page that edge asked for kept it
  // there (off the panel at 390 once the wrapped rows were measured).
  it("reads the older page before centring a mark that is the oldest line read so far", async () => {
    const onLoadOlder = vi.fn();
    const { container, rerender } = render(
      <LogLineList
        lines={range(40, 100)}
        markedKey="k40"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    const viewport = viewportOf(container);
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
    rerender(
      <LogLineList
        lines={range(40, 100)}
        markedKey="k40"
        hasMore
        loadingOlder
        onLoadOlder={onLoadOlder}
      />,
    );
    rerender(
      <LogLineList
        lines={range(0, 100)}
        markedKey="k40"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    act(() => viewport.dispatchEvent(new Event("scroll")));
    expect(viewport.scrollTop).toBe(
      12 + 40 * 24 + 12 - VIRTUAL_VIEWPORT_HEIGHT / 2,
    );
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
    expect(container.querySelector("[data-marked]")).toHaveTextContent(
      "line 40",
    );
    await settle();
  });

  it("reads the older page first in descending order too, where it lands below the mark", async () => {
    const onLoadOlder = vi.fn();
    const { container, rerender } = render(
      <LogLineList
        lines={range(40, 100)}
        order="desc"
        markedKey="k40"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    const viewport = viewportOf(container);
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
    rerender(
      <LogLineList
        lines={range(0, 100)}
        order="desc"
        markedKey="k40"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    act(() => viewport.dispatchEvent(new Event("scroll")));
    // Newest first: line 40 is row 59 of 100.
    expect(viewport.scrollTop).toBe(
      12 + 59 * 24 + 12 - VIRTUAL_VIEWPORT_HEIGHT / 2,
    );
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
    await settle();
  });

  it("centres a mark near the oldest line when there is nothing older to read", () => {
    const onLoadOlder = vi.fn();
    const { container } = render(
      <LogLineList
        lines={range(0, 100)}
        markedKey="k2"
        onLoadOlder={onLoadOlder}
      />,
    );
    // As near the middle as the list allows: its top.
    expect(viewportOf(container).scrollTop).toBe(0);
    expect(container.querySelector("[data-marked]")).toHaveTextContent(
      "line 2",
    );
    expect(onLoadOlder).not.toHaveBeenCalled();
  });

  it("reads each older page through the explorer's current loader", () => {
    // An explorer rebuilds loadOlder with its pager's state, and the one
    // before returns early: the list must call the one this render handed it,
    // or View in context stops after the first older page (production, pin
    // 11b1e8f85f06).
    const before = vi.fn();
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = render(
      <LogLineList
        lines={range(50, 100)}
        markedKey="k10"
        onLoadOlder={before}
      />,
    );
    rerender(
      <LogLineList
        lines={range(50, 100)}
        markedKey="k10"
        hasMore
        onLoadOlder={first}
      />,
    );
    expect(first).toHaveBeenCalledTimes(1);
    rerender(
      <LogLineList
        lines={range(50, 100)}
        markedKey="k10"
        hasMore
        loadingOlder
        onLoadOlder={vi.fn()}
      />,
    );
    rerender(
      <LogLineList
        lines={range(30, 100)}
        markedKey="k10"
        hasMore
        onLoadOlder={second}
      />,
    );
    expect(second).toHaveBeenCalledTimes(1);
    expect(before).not.toHaveBeenCalled();
  });
});

describe("New logs in Live Tail (w3/m528 t004)", () => {
  setupVirtualGeometry();

  it("shows Render's button at the newest edge, which switches to Live and dismisses", async () => {
    const onSwitch = vi.fn();
    const onDismiss = vi.fn();
    const { rerender } = render(
      <LogLineList
        lines={range(0, 3)}
        newLogsInLiveTail={{ onSwitch, onDismiss }}
      />,
    );
    const button = screen.getByRole("button", {
      name: "New logs in Live Tail",
    });
    expect(button).toHaveAttribute("title", "Switch to Live Tail");
    expect(button.parentElement!.className).toContain("bottom-4");
    fireEvent.click(button);
    expect(onSwitch).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
    // The paused list's lines are its own: the button changes none of them.
    expect(screen.getByText("line 2")).toBeInTheDocument();
    rerender(
      <LogLineList
        lines={range(0, 3)}
        order="desc"
        newLogsInLiveTail={{ onSwitch, onDismiss }}
      />,
    );
    expect(
      screen.getByRole("button", { name: "New logs in Live Tail" })
        .parentElement!.className,
    ).toContain("top-4");
    rerender(<LogLineList lines={range(0, 3)} />);
    expect(
      screen.queryByRole("button", { name: "New logs in Live Tail" }),
    ).toBeNull();
  });
});
