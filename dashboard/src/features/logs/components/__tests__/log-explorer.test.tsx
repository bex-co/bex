import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Toaster } from "sonner";
import { LogOptionsMenu, MaximizeButton } from "../log-explorer";
import { LOG_MAXIMIZED_CLASS } from "../log-toolbar-layout";
import { useClearedLines, useLogExplorer } from "../../hooks/use-log-explorer";
import { LogLineList } from "../log-line-list";
import type { LogLine } from "../../types";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// Options ends with Theme Settings, a router Link (w3/m528): render it as
// its anchor, with no router around the component.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: (await import("@/test/mocks/router-link")).PlainLink,
}));

// Render's log explorer controls (w3/m509, render.com/docs/logging): the
// keyboard shortcuts, Maximize, and the Options menu.

const lines: LogLine[] = ["first", "second"].map((message, i) => ({
  key: message,
  timestamp: `2026-11-19T10:00:0${i}Z`,
  instance: "",
  message,
  spans: null,
  resource: "",
  type: "app",
  level: "",
  method: "",
  statusCode: "",
  host: "",
  path: "",
}));

const writeText = vi.fn<(text: string) => Promise<void>>();
beforeEach(() => {
  writeText.mockReset().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
});
afterEach(() => {
  delete (navigator as { clipboard?: unknown }).clipboard;
});

/** One explorer: a search box, its controls and its list. */
function Explorer({
  clear,
  withDialog = false,
}: {
  clear?: () => void;
  withDialog?: boolean;
}) {
  const { explorer, attachList, attachSearch } = useLogExplorer({ clear });
  const [text, setText] = useState("");
  return (
    <div
      data-testid="frame"
      className={explorer.maximized ? LOG_MAXIMIZED_CLASS : undefined}
    >
      <input
        ref={attachSearch}
        aria-label="Search logs"
        value={text}
        onChange={(event) => setText(event.target.value)}
      />
      <MaximizeButton explorer={explorer} />
      <LogOptionsMenu explorer={explorer} canCopy phoneMaximizeItem />
      <p data-testid="state">
        {explorer.order}/{String(explorer.expandAll)}
      </p>
      <LogLineList
        lines={lines}
        order={explorer.order}
        expandAll={explorer.expandAll}
        handleRef={attachList}
        showTimestamps={false}
      />
      {withDialog ? (
        <div role="dialog" data-state="open">
          <button type="button">in a dialog</button>
        </div>
      ) : null}
      <Toaster />
    </div>
  );
}

const maximized = () =>
  screen.getByTestId("frame").className.includes("fixed inset-0");

describe("log explorer shortcuts (w3/m509)", () => {
  setupVirtualGeometry();

  it("focuses search on /, without typing the slash", async () => {
    const user = userEvent.setup();
    render(<Explorer />);
    await user.keyboard("/");
    const search = screen.getByRole("textbox", { name: "Search logs" });
    expect(search).toHaveFocus();
    expect(search).toHaveValue("");
  });

  it("toggles full screen on M and leaves it on M or Esc", async () => {
    const user = userEvent.setup();
    render(<Explorer />);
    await user.keyboard("m");
    expect(maximized()).toBe(true);
    expect(
      screen.getByRole("button", { name: "Exit full screen" }),
    ).toBeInTheDocument();
    await user.keyboard("M");
    expect(maximized()).toBe(false);
    await user.keyboard("m");
    await user.keyboard("{Escape}");
    expect(maximized()).toBe(false);
  });

  it("never fires while typing in a field, and Esc still leaves full screen", async () => {
    const user = userEvent.setup();
    const clear = vi.fn();
    render(<Explorer clear={clear} />);
    const search = screen.getByRole("textbox", { name: "Search logs" });
    await user.click(search);
    await user.keyboard("m/L{Home}{End}");
    expect(search).toHaveValue("m/L");
    expect(maximized()).toBe(false);
    expect(clear).not.toHaveBeenCalled();
    fireEvent.keyDown(search, { key: "C", ctrlKey: true, shiftKey: true });
    expect(writeText).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Maximize" }));
    await user.click(search);
    await user.keyboard("{Escape}");
    expect(maximized()).toBe(false);
  });

  it("leaves keys to an open dialog", async () => {
    const user = userEvent.setup();
    render(<Explorer withDialog />);
    await user.keyboard("m");
    expect(maximized()).toBe(false);
  });

  it("copies every line shown on ⌘⇧C and Ctrl+Shift+C", async () => {
    render(<Explorer />);
    await act(async () => {
      fireEvent.keyDown(window, { key: "C", metaKey: true, shiftKey: true });
    });
    expect(writeText).toHaveBeenCalledWith("first\nsecond");
    await act(async () => {
      fireEvent.keyDown(window, { key: "c", ctrlKey: true, shiftKey: true });
    });
    expect(writeText).toHaveBeenCalledTimes(2);
    expect(await screen.findAllByText("Logs copied")).not.toHaveLength(0);
  });

  it("does not copy on a plain ⌘C, which copies the selection", () => {
    render(<Explorer />);
    fireEvent.keyDown(window, { key: "c", metaKey: true });
    expect(writeText).not.toHaveBeenCalled();
  });

  it("clears with Shift+L only while a live tail gives it a clear", () => {
    const clear = vi.fn();
    const { rerender } = render(<Explorer clear={clear} />);
    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(clear).toHaveBeenCalledTimes(1);
    // A plain l is not the shortcut.
    fireEvent.keyDown(window, { key: "l" });
    expect(clear).toHaveBeenCalledTimes(1);
    rerender(<Explorer />);
    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(clear).toHaveBeenCalledTimes(1);
  });

  it("jumps with Home and End", () => {
    const { container } = render(<Explorer />);
    const viewport = container.querySelector<HTMLElement>(
      "[data-log-viewport]",
    )!;
    viewport.scrollTop = 30;
    fireEvent.keyDown(window, { key: "Home" });
    expect(viewport.scrollTop).toBe(0);
  });
});

describe("log explorer Options (w3/m509)", () => {
  setupVirtualGeometry();

  it("copies, orders and expands from the menu, and offers Maximize on a phone", async () => {
    const user = userEvent.setup();
    render(<Explorer />);
    const open = () =>
      user.click(screen.getByRole("button", { name: "Log options" }));

    await open();
    expect(
      screen.getByRole("menuitem", { name: /Copy logs/ }),
    ).toHaveTextContent("Ctrl+Shift+C");
    await user.click(screen.getByRole("menuitem", { name: /Copy logs/ }));
    // user-event puts its own clipboard in place of the page's.
    expect(await navigator.clipboard.readText()).toBe("first\nsecond");

    await open();
    await user.click(screen.getByRole("menuitemradio", { name: "Descending" }));
    expect(screen.getByTestId("state")).toHaveTextContent("desc/false");

    await open();
    await user.click(
      screen.getByRole("menuitemcheckbox", { name: "Expand everything" }),
    );
    expect(screen.getByTestId("state")).toHaveTextContent("desc/true");

    await open();
    const item = screen.getByRole("menuitem", { name: "Maximize" });
    // A phone's only way to it: hidden where the button shows.
    expect(item.className).toContain("sm:hidden");
    await user.click(item);
    expect(maximized()).toBe(true);
  });

  it("disables Copy logs while no line is shown", async () => {
    function Empty() {
      const { explorer } = useLogExplorer();
      return <LogOptionsMenu explorer={explorer} canCopy={false} />;
    }
    const user = userEvent.setup();
    render(<Empty />);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(screen.getByRole("menuitem", { name: /Copy logs/ })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  // Render's Options ends with Theme Settings (w3/m528): `href:
  // "/u/settings#appearance"`, bex's account Settings ▸ Appearance.
  it("ends with Theme Settings, a link to account Settings' Appearance", async () => {
    const user = userEvent.setup();
    render(<Explorer />);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    const items = screen.getAllByRole("menuitem");
    const last = items.at(-1)!;
    expect(last).toHaveAccessibleName("Theme Settings");
    expect(last).toHaveAttribute("href", "/settings#appearance");
  });
});

describe("useClearedLines (w3/m509)", () => {
  const at = (key: string, second: number) => ({
    key,
    timestamp: `2026-11-19T10:00:${String(second).padStart(2, "0")}Z`,
  });

  it("hides the lines shown when cleared, and shows lines after them", () => {
    const shown = [at("a", 1), at("b", 2)];
    const { result, rerender } = renderHook(
      ({ list, live, reader }) => useClearedLines(list, live, reader),
      { initialProps: { list: shown, live: true, reader: "r" } },
    );
    act(() => result.current.clear!());
    expect(result.current.lines).toEqual([]);
    expect(result.current.cleared).toBe(true);

    rerender({ list: [...shown, at("c", 3)], live: true, reader: "r" });
    expect(result.current.lines.map((line) => line.key)).toEqual(["c"]);
  });

  it("brings the lines back on restore, a paused tail or a new reader", () => {
    const shown = [at("a", 1), at("b", 2)];
    const { result, rerender } = renderHook(
      ({ list, live, reader }) => useClearedLines(list, live, reader),
      { initialProps: { list: shown, live: true, reader: "r" } },
    );
    act(() => result.current.clear!());
    act(() => result.current.restore());
    expect(result.current.lines).toBe(shown);

    act(() => result.current.clear!());
    rerender({ list: shown, live: false, reader: "r" });
    expect(result.current.lines).toBe(shown);
    // Resuming the tail does not clear them again.
    rerender({ list: shown, live: true, reader: "r" });
    expect(result.current.lines).toBe(shown);

    act(() => result.current.clear!());
    rerender({ list: shown, live: true, reader: "other filters" });
    expect(result.current.lines).toBe(shown);
    expect(result.current.cleared).toBe(false);
  });

  it("offers no clear while the tail is paused", () => {
    const { result } = renderHook(() =>
      useClearedLines([at("a", 1)], false, "r"),
    );
    expect(result.current.clear).toBeUndefined();
  });

  it("keeps lines without a time that arrive after the cut", () => {
    const shown = [at("a", 1)];
    const { result, rerender } = renderHook(
      ({ list }) => useClearedLines(list, true, "r"),
      { initialProps: { list: shown } },
    );
    act(() => result.current.clear!());
    rerender({ list: [...shown, { key: "x", timestamp: "" }, at("b", 2)] });
    expect(result.current.lines.map((line) => line.key)).toEqual(["x", "b"]);
  });

  it("keeps the same array while nothing is cleared, so the list keeps its place", () => {
    const shown = [at("a", 1)];
    const { result, rerender } = renderHook(
      ({ list }) => useClearedLines(list, true, "r"),
      { initialProps: { list: shown } },
    );
    const first = result.current.lines;
    rerender({ list: shown });
    expect(result.current.lines).toBe(first);
  });
});
