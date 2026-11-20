import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LogLineList, type LogLineListHandle } from "../log-line-list";
import type { LogLine } from "../../types";
import {
  scrollViewport,
  setupVirtualGeometry,
  VIRTUAL_VIEWPORT_HEIGHT,
} from "@/test/virtual-geometry";

// What a log explorer shows of each line and drives in its list (w3/m509):
// the level gutter, the timestamp hover, date separators, the order and
// Display options, and the handle Home/End and Copy logs use.

function line(over: Partial<LogLine> = {}): LogLine {
  return {
    key: over.key ?? "k",
    timestamp: "2026-11-19T10:36:01.709Z",
    instance: "",
    message: over.key ?? "hello",
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

const buffer = (n: number) =>
  Array.from({ length: n }, (_, i) =>
    line({ key: `k${i}`, message: `line ${i}` }),
  );

const viewportOf = (container: HTMLElement) =>
  container.querySelector<HTMLElement>("[data-log-viewport]")!;
const rowOf = (text: string) =>
  screen.getByText(text).closest<HTMLElement>("[data-index]")!;

describe("LogLineList level gutter (w3/m509)", () => {
  setupVirtualGeometry();

  it("marks a 404 request line with the warning icon and tints its row", () => {
    render(
      <LogLineList
        lines={[
          line({
            key: "r",
            type: "request",
            method: "GET",
            statusCode: "404",
            message: "not found",
          }),
        ]}
      />,
    );
    const row = rowOf("not found");
    const mark = screen.getByRole("img", { name: "Level: warning" });
    expect(row).toContainElement(mark);
    expect(mark).toHaveAttribute("title", "warning");
    expect(row.className).toContain("bg-amber-500/10");
  });

  it("names a line's own level and tints every error-class level", () => {
    render(
      <LogLineList
        lines={[line({ key: "c", level: "critical", message: "disk gone" })]}
      />,
    );
    expect(
      screen.getByRole("img", { name: "Level: critical" }),
    ).toBeInTheDocument();
    expect(rowOf("disk gone").className).toContain("bg-destructive/10");
  });

  it("shows an info line's mark only on hover, without tinting it", () => {
    render(
      <LogLineList
        lines={[line({ key: "i", level: "info", message: "started" })]}
      />,
    );
    const mark = screen.getByRole("img", { name: "Level: info" });
    expect(mark.className).toContain("opacity-0");
    expect(mark.className).toContain("group-hover/row:opacity-100");
    expect(rowOf("started").className).not.toContain("bg-amber-500/10");
    expect(rowOf("started").className).not.toContain("bg-destructive/10");
  });

  // Render's LogLevel marks (w3/086): debug and notice hide until hover as
  // info does, and notice is the info mark in debug's grey.
  it("shows debug and notice marks only on hover, in the muted colour", () => {
    render(
      <LogLineList
        lines={[
          line({ key: "d", level: "debug", message: "probe" }),
          line({ key: "n", level: "notice", message: "heads up" }),
        ]}
      />,
    );
    for (const level of ["debug", "notice"]) {
      const mark = screen.getByRole("img", { name: `Level: ${level}` });
      expect(mark.className).toContain("opacity-0");
      expect(mark.className).toContain("group-hover/row:opacity-100");
      expect(mark.className).toContain("text-muted-foreground");
    }
    expect(rowOf("heads up").className).not.toContain("bg-destructive/10");
  });

  it("keeps the gutter's column for a line with no level", () => {
    const { container } = render(
      <LogLineList lines={[line({ key: "n", message: "plain" })]} />,
    );
    expect(screen.queryByRole("img")).toBeNull();
    expect(
      viewportOf(container).querySelector("[data-index] > span[aria-hidden]"),
    ).not.toBeNull();
  });
});

describe("LogLineList timestamp hover (w3/m509)", () => {
  setupVirtualGeometry();

  it("opens the local, UTC and Unix timestamp on hover", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <LogLineList lines={[line({ key: "a", message: "first" })]} />,
    );
    const time = container.querySelector("time")!;
    // Out of the Tab order: the row cursor opens it for the keyboard.
    expect(time.tabIndex).toBe(-1);
    // No tooltip machinery in a row until it is hovered.
    expect(time).not.toHaveAttribute("data-state");
    await user.hover(time);
    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent(
      "UTCNovember 19, 2026 at 10:36:01 AM UTC",
    );
    expect(tooltip).toHaveTextContent("Timestamp1795084561");
  });

  it("opens it for the keyboard's row cursor", async () => {
    const { container } = render(
      <LogLineList
        lines={[line({ key: "a", message: "first" })]}
        onInstanceFilter={vi.fn()}
      />,
    );
    const viewport = viewportOf(container);
    act(() => viewport.focus());
    fireEvent.keyDown(viewport, { key: "ArrowDown" });
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Timestamp1795084561",
    );
  });
});

describe("LogLineList date separators (w3/m509)", () => {
  setupVirtualGeometry();

  const acrossMidnight = [
    line({ key: "late", timestamp: "2026-11-18T23:59:59Z" }),
    line({ key: "early", timestamp: "2026-11-19T00:00:01Z" }),
  ];
  const days = (container: HTMLElement) =>
    [...container.querySelectorAll("[data-log-day]")].map(
      (row) => row.textContent,
    );

  it("marks each day of a range across midnight", () => {
    const { container } = render(<LogLineList lines={acrossMidnight} />);
    expect(days(container)).toEqual(["Nov 18", "Nov 19"]);
  });

  it("marks them newest first in descending order", () => {
    const { container } = render(
      <LogLineList lines={acrossMidnight} order="desc" />,
    );
    expect(days(container)).toEqual(["Nov 19", "Nov 18"]);
    const text = viewportOf(container).textContent ?? "";
    expect(text.indexOf("Nov 19")).toBeLessThan(text.indexOf("early"));
    expect(text.indexOf("early")).toBeLessThan(text.indexOf("Nov 18"));
  });

  it("adds none to one day's lines", () => {
    const { container } = render(<LogLineList lines={buffer(3)} />);
    expect(days(container)).toEqual([]);
  });

  it("moves the keyboard's row cursor over a separator, never onto it", () => {
    const onInstanceFilter = vi.fn();
    const { container } = render(
      <LogLineList
        lines={[
          line({ ...acrossMidnight[0], instance: "web-1" }),
          line({ ...acrossMidnight[1], instance: "web-2" }),
        ]}
        onInstanceFilter={onInstanceFilter}
      />,
    );
    const viewport = viewportOf(container);
    act(() => viewport.focus());
    fireEvent.keyDown(viewport, { key: "ArrowDown" });
    fireEvent.keyDown(viewport, { key: "ArrowDown" });
    expect(rowOf("early")).toHaveAttribute("data-active");
    fireEvent.keyDown(viewport, { key: "Enter" });
    expect(onInstanceFilter).toHaveBeenCalledWith("web-2");
  });
});

describe("LogLineList order (w3/m509)", () => {
  setupVirtualGeometry();

  it("puts the newest line first in descending order", () => {
    const { container } = render(
      <LogLineList lines={buffer(3)} order="desc" />,
    );
    const rows = [...container.querySelectorAll("[data-index]")];
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining("line 2"),
      expect.stringContaining("line 1"),
      expect.stringContaining("line 0"),
    ]);
  });

  it("pins to the top descending: reading down offers the way back up", async () => {
    const { container } = render(
      <LogLineList lines={buffer(1000)} order="desc" />,
    );
    const viewport = viewportOf(container);
    expect(screen.queryByRole("button", { name: "Return to Top" })).toBeNull();
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 2000,
        scrollHeight: 24_000,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    // Render's Return to Top, at the top right descending (w3/m528).
    expect(
      screen.queryByRole("button", { name: "Return to Bottom" }),
    ).toBeNull();
    const jump = screen.getByRole("button", { name: "Return to Top" });
    expect(jump.className).toContain("top-4");
    expect(jump.className).toContain("right-6");
    expect(jump).toHaveTextContent("");
    fireEvent.click(jump);
    expect(viewport.scrollTop).toBe(0);
    expect(screen.queryByRole("button", { name: "Return to Top" })).toBeNull();
    await act(() => new Promise((resolve) => setTimeout(resolve, 175)));
  });

  it("loads older lines near the bottom when descending", () => {
    const onLoadOlder = vi.fn();
    const { container } = render(
      <LogLineList
        lines={buffer(1000)}
        order="desc"
        hasMore
        onLoadOlder={onLoadOlder}
      />,
    );
    const viewport = viewportOf(container);
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 0,
        scrollHeight: 24_000,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    expect(onLoadOlder).not.toHaveBeenCalled();
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 24_000 - VIRTUAL_VIEWPORT_HEIGHT - 10,
        scrollHeight: 24_000,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
  });
});

// Render's Options ▸ Display (w3/088, its `Logs-*.js` truncation mode): a
// line shown whole wraps; a line on one line is cut off at the panel's edge
// (`overflow-hidden text-ellipsis whitespace-nowrap`). Render has no per-row
// Show more, and no clamp under Expand everything.
describe("LogLineList Display (w3/088)", () => {
  setupVirtualGeometry();

  const ONE_LINE = ["overflow-hidden", "text-ellipsis", "whitespace-nowrap"];
  // The message cell, by its text exactly as written (line breaks kept).
  const cellOf = (text: string) =>
    screen.getByText(text, { normalizer: (value) => value });
  const isOneLine = (text: string) =>
    ONE_LINE.every((name) => cellOf(text).classList.contains(name));

  const long = `${"a".repeat(5000)}\nSECOND-LINE ${"b".repeat(3000)}TAIL`;
  const levels = [
    line({ key: "error", level: "error", message: "an error" }),
    line({ key: "critical", level: "critical", message: "a critical" }),
    line({ key: "alert", level: "alert", message: "an alert" }),
    line({ key: "emergency", level: "emergency", message: "an emergency" }),
    // A request line with no level label takes its status class's (w4/584).
    line({
      key: "503",
      type: "request",
      statusCode: "503",
      message: "a 503",
    }),
    line({ key: "warning", level: "warning", message: "a warning" }),
    line({ key: "info", level: "info", message: "an info" }),
    line({ key: "plain", message: "no level" }),
  ];
  const errors = [
    "an error",
    "a critical",
    "an alert",
    "an emergency",
    "a 503",
  ];
  const others = ["a warning", "an info", "no level"];

  it("shows every line whole by default, however long, with no Show more", () => {
    const { container } = render(
      <LogLineList lines={[line({ key: "l", message: long })]} />,
    );
    expect(container.textContent).toContain("SECOND-LINE");
    expect(container.textContent).toContain("TAIL");
    expect(isOneLine(long)).toBe(false);
    expect(cellOf(long).className).toContain("-indent-[2ch]");
    expect(screen.queryByRole("button", { name: /show/i })).toBeNull();
  });

  it("puts every line on one line under Collapse everything, errors included", () => {
    render(<LogLineList lines={levels} display="collapse-all" />);
    for (const text of [...errors, ...others]) {
      expect(isOneLine(text), text).toBe(true);
    }
  });

  it("shows only Render's error levels whole under Expand errors", () => {
    render(<LogLineList lines={levels} display="expand-errors" />);
    for (const text of errors) expect(isOneLine(text), text).toBe(false);
    for (const text of others) expect(isOneLine(text), text).toBe(true);
  });

  it("keeps a one-line row's whole message in the DOM, for selection and copy", () => {
    const handle = createRef<LogLineListHandle>();
    const { container } = render(
      <LogLineList
        lines={[line({ key: "l", message: long })]}
        display="collapse-all"
        showTimestamps={false}
        handleRef={handle}
      />,
    );
    expect(isOneLine(long)).toBe(true);
    expect(container.textContent).toContain("TAIL");
    expect(handle.current!.text()).toBe(long);
  });
});

describe("LogLineList handle (w3/m509)", () => {
  setupVirtualGeometry();

  it("copies every row shown, not only the rows in the DOM", () => {
    const handle = createRef<LogLineListHandle>();
    const { container } = render(
      <LogLineList lines={buffer(1000)} handleRef={handle} />,
    );
    expect(container.querySelectorAll("[data-index]").length).toBeLessThan(120);
    const text = handle.current!.text().split("\n");
    expect(text).toHaveLength(1000);
    expect(text[0]).toMatch(/^\d\d:\d\d:\d\d [AP]M line 0$/);
    expect(text[999]).toMatch(/line 999$/);
  });

  it("copies in the order shown, without the time when the list hides it", () => {
    const handle = createRef<LogLineListHandle>();
    render(
      <LogLineList
        lines={buffer(2)}
        order="desc"
        showTimestamps={false}
        handleRef={handle}
      />,
    );
    expect(handle.current!.text()).toBe("line 1\nline 0");
  });

  it("jumps to the top and back to the newest line at the bottom", async () => {
    const handle = createRef<LogLineListHandle>();
    const { container } = render(
      <LogLineList lines={buffer(1000)} handleRef={handle} />,
    );
    const viewport = viewportOf(container);
    act(() => {
      viewport.scrollTop = 4000;
      handle.current!.jump("top");
    });
    expect(viewport.scrollTop).toBe(0);
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 0,
        scrollHeight: 24_000,
        clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
      }),
    );
    expect(
      screen.getByRole("button", { name: "Return to Bottom" }),
    ).toBeInTheDocument();
    act(() => handle.current!.jump("bottom"));
    expect(
      screen.queryByRole("button", { name: "Return to Bottom" }),
    ).toBeNull();
    await act(() => new Promise((resolve) => setTimeout(resolve, 175)));
  });
});
