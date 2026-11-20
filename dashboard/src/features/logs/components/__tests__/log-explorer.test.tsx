import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Toaster } from "sonner";
import { LogOptionsMenu, MaximizeButton } from "../log-explorer";
import { LOG_MAXIMIZED_CLASS } from "../log-toolbar-layout";
import { useClearedLines, useLogExplorer } from "../../hooks/use-log-explorer";
import { logDisplaySetting } from "../../lib/log-display";
import { LogLineList } from "../log-line-list";
import type { LogLine } from "../../types";
import { setupVirtualGeometry } from "@/test/virtual-geometry";
import { memoryStorage } from "@/test/mocks/storage";
import {
  BROWSER_ENVIRONMENT,
  hydrateAcrossEnvironments,
} from "@/test/hydration";

// Options ends with Theme Settings, a router Link (w3/m528): render it as
// its anchor, with no router around the component.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: (await import("@/test/mocks/router-link")).PlainLink,
}));

// A phone's Options menu lists Display's choices itself (w3/088).
const viewport = vi.hoisted(() => ({ isMobile: false }));
vi.mock("@/common/hooks/use-mobile", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/common/hooks/use-mobile")>()),
  useIsMobile: () => viewport.isMobile,
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
  // Options ▸ Display is kept in localStorage (w3/088): each test starts
  // with nothing stored.
  vi.stubGlobal("localStorage", memoryStorage());
});
afterEach(() => {
  delete (navigator as { clipboard?: unknown }).clipboard;
  vi.unstubAllGlobals();
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
      <p data-testid="state">{`${explorer.order}/${explorer.display}`}</p>
      <LogLineList
        lines={lines}
        order={explorer.order}
        display={explorer.display}
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

/**
 * Opens Options ▸ Display and returns its submenu, by keyboard: jsdom's
 * zero-size rects defeat Radix's pointer-intent grace area, so a pointer
 * click on a submenu item never selects (as move-to-project-menu's test
 * says). ArrowRight opens it, as a reader's keyboard does.
 */
async function openDisplay(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Log options" }));
  screen.getByRole("menuitem", { name: /^Display:/ }).focus();
  await user.keyboard("{ArrowRight}");
  return screen.findByRole("menu", { name: /^Display:/ });
}

/** Chooses `name` in Options ▸ Display. */
async function chooseDisplay(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) {
  const submenu = await openDisplay(user);
  within(submenu).getByRole("menuitemradio", { name }).focus();
  await user.keyboard("{Enter}");
}

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

  it("copies, orders and sets Display from the menu, and offers Maximize on a phone", async () => {
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
    expect(screen.getByTestId("state")).toHaveTextContent("desc/expand-all");

    await chooseDisplay(user, "Collapse everything");
    expect(screen.getByTestId("state")).toHaveTextContent("desc/collapse-all");

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

// Render's Options ▸ Display (w3/088, `Logs-*.js`): a submenu reading
// `Display: <choice>` over Display settings' Expand everything (the default),
// Expand errors and Collapse everything, kept in localStorage
// (`render-log-truncation-mode`) for every explorer.
describe("log explorer Display (w3/088)", () => {
  setupVirtualGeometry();
  const DISPLAY_KEY = "bex.logs.display";

  afterEach(() => {
    viewport.isMobile = false;
  });

  it("is Render's submenu, its choice named on the item, Expand everything by default", async () => {
    const user = userEvent.setup();
    render(<Explorer />);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(
      screen.getByRole("menuitem", { name: "Display: Expand everything" }),
    ).toHaveAttribute("aria-haspopup", "menu");
    await user.keyboard("{Escape}");

    const submenu = await openDisplay(user);
    expect(within(submenu).getByText("Display settings")).toBeInTheDocument();
    const choices = within(submenu).getAllByRole("menuitemradio");
    expect(choices.map((choice) => choice.textContent)).toEqual([
      "Expand everything",
      "Expand errors",
      "Collapse everything",
    ]);
    expect(choices[0]).toHaveAttribute("aria-checked", "true");
    choices[1].focus();
    await user.keyboard("{Enter}");
    expect(screen.getByTestId("state")).toHaveTextContent("asc/expand-errors");
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(
      screen.getByRole("menuitem", { name: "Display: Expand errors" }),
    ).toBeInTheDocument();
  });

  it("keeps the choice across visits, one for every explorer", async () => {
    const user = userEvent.setup();
    const first = render(<Explorer />);
    await chooseDisplay(user, "Collapse everything");
    expect(localStorage.getItem(DISPLAY_KEY)).toBe("collapse-all");
    first.unmount();

    // Another explorer, opened afterwards, opens in it.
    render(<Explorer />);
    expect(screen.getByTestId("state")).toHaveTextContent("asc/collapse-all");
  });

  it("follows a choice made in another tab", () => {
    render(<Explorer />);
    act(() => {
      localStorage.setItem(DISPLAY_KEY, "expand-errors");
      window.dispatchEvent(new StorageEvent("storage", { key: DISPLAY_KEY }));
    });
    expect(screen.getByTestId("state")).toHaveTextContent("asc/expand-errors");
  });

  it("reads a stored value it does not know as Expand everything", () => {
    localStorage.setItem(DISPLAY_KEY, "wrap-everything");
    render(<Explorer />);
    expect(screen.getByTestId("state")).toHaveTextContent("asc/expand-all");
  });

  it("keeps a choice storage refuses for the page", async () => {
    const refusing = memoryStorage();
    refusing.setItem = () => {
      throw new DOMException("refused", "QuotaExceededError");
    };
    vi.stubGlobal("localStorage", refusing);
    const user = userEvent.setup();
    render(<Explorer />);
    await chooseDisplay(user, "Collapse everything");
    expect(screen.getByTestId("state")).toHaveTextContent("asc/collapse-all");
    // A write storage takes ends the page's own copy, for the tests after.
    vi.stubGlobal("localStorage", memoryStorage());
    act(() => logDisplaySetting.write("expand-all"));
    expect(screen.getByTestId("state")).toHaveTextContent("asc/expand-all");
  });

  it("renders Expand everything on the server and while hydrating, then the stored choice", async () => {
    let hydrated = "";
    const probe = await hydrateAcrossEnvironments(() => <Explorer />, {
      client: {
        ...BROWSER_ENVIRONMENT,
        storage: { [DISPLAY_KEY]: "collapse-all" },
      },
      inspect: (container) => {
        hydrated = within(container).getByTestId("state").textContent ?? "";
      },
    });
    expect(probe.html).toContain("asc/expand-all");
    expect(probe.recovered).toEqual([]);
    expect(hydrated).toBe("asc/collapse-all");
  });

  it("lists the choices in the menu itself on a phone", async () => {
    viewport.isMobile = true;
    const user = userEvent.setup();
    render(<Explorer />);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(screen.queryByRole("menuitem", { name: /^Display:/ })).toBeNull();
    const group = screen.getByRole("group", { name: "Display settings" });
    await user.click(
      within(group).getByRole("menuitemradio", { name: "Expand errors" }),
    );
    expect(screen.getByTestId("state")).toHaveTextContent("asc/expand-errors");
  });
});

// Render's Options ▸ Order by (w3/089, `Logs-*.js`): one direction for every
// explorer, kept in localStorage (`render-log-direction-setting`).
describe("log explorer Order by (w3/089)", () => {
  setupVirtualGeometry();
  const ORDER_KEY = "bex.logs.order";
  const shownOrder = () =>
    screen
      .getAllByText(/^(first|second)$/)
      .map((element) => element.textContent);

  it("opens a new explorer in the order the last one chose", async () => {
    const user = userEvent.setup();
    const first = render(<Explorer />);
    expect(shownOrder()).toEqual(["first", "second"]);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    await user.click(screen.getByRole("menuitemradio", { name: "Descending" }));
    expect(localStorage.getItem(ORDER_KEY)).toBe("desc");
    first.unmount();

    // Another explorer, opened afterwards, opens newest first.
    render(<Explorer />);
    expect(screen.getByTestId("state")).toHaveTextContent("desc/expand-all");
    expect(shownOrder()).toEqual(["second", "first"]);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(
      screen.getByRole("menuitemradio", { name: "Descending" }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it("follows an order chosen in another tab", () => {
    render(<Explorer />);
    act(() => {
      localStorage.setItem(ORDER_KEY, "desc");
      window.dispatchEvent(new StorageEvent("storage", { key: ORDER_KEY }));
    });
    expect(screen.getByTestId("state")).toHaveTextContent("desc/expand-all");
  });

  it("reads a stored value it does not know as Ascending", () => {
    localStorage.setItem(ORDER_KEY, "backward");
    render(<Explorer />);
    expect(screen.getByTestId("state")).toHaveTextContent("asc/expand-all");
  });

  it("renders Ascending on the server and while hydrating, then the stored order", async () => {
    let hydrated = "";
    const probe = await hydrateAcrossEnvironments(() => <Explorer />, {
      client: { ...BROWSER_ENVIRONMENT, storage: { [ORDER_KEY]: "desc" } },
      inspect: (container) => {
        hydrated = within(container).getByTestId("state").textContent ?? "";
      },
    });
    expect(probe.html).toContain("asc/expand-all");
    expect(probe.recovered).toEqual([]);
    expect(hydrated).toBe("desc/expand-all");
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
