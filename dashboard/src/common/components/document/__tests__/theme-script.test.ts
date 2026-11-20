import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { memoryStorage } from "@/test/mocks/storage";
import script from "../theme-script.js.txt?raw";

// The pre-paint script (w3/m531): before React runs, <html> gets the theme
// the browser's copy of the account's settings holds, its high-contrast twin
// included, so a reload never flashes the other theme.

const root = () => document.documentElement.classList;
let prefersDark = false;

function run() {
  new Function(script)();
}

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
  prefersDark = false;
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: query === "(prefers-color-scheme: dark)" && prefersDark,
    }),
  });
  root().remove("light", "dark", "high-contrast");
});
afterEach(() => {
  vi.unstubAllGlobals();
  root().remove("light", "dark", "high-contrast");
  window.history.replaceState(null, "", "/");
});

const settings = (themeSetting: string) =>
  localStorage.setItem(
    "bex-user-settings",
    JSON.stringify({ themeSetting, logThemeSetting: "MATCH_APP" }),
  );

describe("the pre-paint theme script (w3/m531)", () => {
  it.each([
    ["DARK", "dark", false],
    ["LIGHT", "light", false],
    ["DARK_HIGH_CONTRAST", "dark", true],
    ["LIGHT_HIGH_CONTRAST", "light", true],
  ])("paints %s as %s, high contrast %s", (setting, theme, highContrast) => {
    settings(setting);
    run();
    expect(root().contains(theme)).toBe(true);
    expect(root().contains(theme === "dark" ? "light" : "dark")).toBe(false);
    expect(root().contains("high-contrast")).toBe(highContrast);
  });

  it("resolves System and its twin through the system preference", () => {
    prefersDark = true;
    settings("SYSTEM_HIGH_CONTRAST");
    run();
    expect(root().contains("dark")).toBe(true);
    expect(root().contains("high-contrast")).toBe(true);
  });

  it("reads the theme a browser kept before the account did", () => {
    localStorage.setItem("vite-ui-theme", "dark");
    run();
    expect(root().contains("dark")).toBe(true);
    expect(root().contains("high-contrast")).toBe(false);
  });

  it("lets ?theme= win, and keeps High Contrast Mode", () => {
    settings("DARK_HIGH_CONTRAST");
    window.history.replaceState(null, "", "/?theme=light");
    run();
    expect(root().contains("light")).toBe(true);
    expect(root().contains("high-contrast")).toBe(true);
  });

  it("falls back to the system preference on a copy it cannot read", () => {
    localStorage.setItem("bex-user-settings", "{not json");
    run();
    expect(root().contains("light")).toBe(true);
  });
});
