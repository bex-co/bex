import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { useContext } from "react";
import { ThemeProvider } from "../index";
import { ThemeProviderContext } from "../context";
import { SYSTEM_DARK_QUERY, THEME_STORAGE_KEY } from "../utils";

// A controllable prefers-color-scheme query: `prefer` flips it and delivers a
// real change event to whoever is listening, as the browser does (w4/315).
function mediaQuery() {
  const listeners = new Set<(e: MediaQueryListEvent) => void>();
  let dark = false;
  const mql = {
    get matches() {
      return dark;
    },
    media: SYSTEM_DARK_QUERY,
    onchange: null,
    addEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) =>
      listeners.add(fn),
    removeEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) =>
      listeners.delete(fn),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  };
  return {
    install() {
      Object.defineProperty(window, "matchMedia", {
        configurable: true,
        writable: true,
        value: () => mql,
      });
    },
    prefer(next: "dark" | "light") {
      dark = next === "dark";
      act(() => {
        for (const fn of listeners)
          fn({ matches: dark } as MediaQueryListEvent);
      });
    },
    listeners,
  };
}

const applied = () => {
  const list = document.documentElement.classList;
  return list.contains("dark")
    ? "dark"
    : list.contains("light")
      ? "light"
      : null;
};

let setThemeRef: ((t: "light" | "dark" | "system") => void) | null = null;
function Capture() {
  setThemeRef = useContext(ThemeProviderContext).setTheme;
  return null;
}

describe("System theme follows preference changes (w4/315)", () => {
  const originalMatchMedia = window.matchMedia;
  let media: ReturnType<typeof mediaQuery>;
  beforeEach(() => {
    media = mediaQuery();
    media.install();
    vi.mocked(localStorage.setItem).mockClear();
    document.documentElement.classList.remove("light", "dark");
  });
  afterEach(() => {
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: originalMatchMedia,
    });
    document.documentElement.classList.remove("light", "dark");
  });

  it("switches light → dark → light while the choice stays system", () => {
    render(
      <ThemeProvider>
        <Capture />
      </ThemeProvider>,
    );
    expect(applied()).toBe("light");
    media.prefer("dark");
    expect(applied()).toBe("dark");
    media.prefer("light");
    expect(applied()).toBe("light");
    // A preference change never persists a resolved color as the choice.
    expect(localStorage.setItem).not.toHaveBeenCalled();
  });

  it("ignores preference changes under an explicit choice, and after leaving system", () => {
    render(
      <ThemeProvider>
        <Capture />
      </ThemeProvider>,
    );
    act(() => setThemeRef?.("light"));
    media.prefer("dark");
    expect(applied()).toBe("light");
    expect(media.listeners.size).toBe(0);

    act(() => setThemeRef?.("dark"));
    media.prefer("light");
    expect(applied()).toBe("dark");

    // Back to System applies the current preference, then follows it again.
    act(() => setThemeRef?.("system"));
    expect(applied()).toBe("light");
    media.prefer("dark");
    expect(applied()).toBe("dark");
    expect(vi.mocked(localStorage.setItem).mock.calls.at(-1)).toEqual([
      THEME_STORAGE_KEY,
      "system",
    ]);
  });

  it("stops listening when unmounted", () => {
    const { unmount } = render(
      <ThemeProvider>
        <Capture />
      </ThemeProvider>,
    );
    expect(media.listeners.size).toBe(1);
    unmount();
    expect(media.listeners.size).toBe(0);
    media.prefer("dark");
    expect(applied()).toBe("light");
  });
});
