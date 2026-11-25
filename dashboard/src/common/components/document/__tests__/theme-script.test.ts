import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { memoryStorage } from "@/test/mocks/storage";
import { PAGE_COLOR } from "@/test/theme-tokens";
import { themeScript } from "../theme-script";

// The pre-paint script (w3/m531): before React runs, <html> gets the theme
// the browser's copy of the account's settings holds, its high-contrast twin
// included, so a reload never flashes the other theme. Its page colour goes on
// <meta name="theme-color"> at the same moment (w3/098).

const root = () => document.documentElement.classList;
const themeColors = () =>
  [
    ...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]'),
  ].map((meta) => meta.content);
const removeThemeColors = () =>
  document
    .querySelectorAll('meta[name="theme-color"]')
    .forEach((meta) => meta.remove());
let prefersDark = false;

function run() {
  new Function(themeScript)();
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
  removeThemeColors();
});
afterEach(() => {
  vi.unstubAllGlobals();
  root().remove("light", "dark", "high-contrast");
  removeThemeColors();
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

describe("the pre-paint theme colour (w3/098)", () => {
  it.each(Object.entries(PAGE_COLOR))(
    "gives %s's page colour, %s, to one <meta name=theme-color>",
    (setting, color) => {
      settings(setting);
      run();
      expect(themeColors()).toEqual([color]);
    },
  );

  it("gives System and its twin the colour of the system preference", () => {
    prefersDark = true;
    settings("SYSTEM_HIGH_CONTRAST");
    run();
    expect(themeColors()).toEqual([PAGE_COLOR.DARK]);
  });

  it("follows ?theme=, and colours a document with no stored theme", () => {
    window.history.replaceState(null, "", "/?theme=dark");
    run();
    expect(themeColors()).toEqual([PAGE_COLOR.DARK]);
    window.history.replaceState(null, "", "/");
    run();
    expect(themeColors()).toEqual([PAGE_COLOR.LIGHT]);
  });

  it("recolours the tag a document already has rather than adding one", () => {
    const meta = document.createElement("meta");
    meta.name = "theme-color";
    meta.content = "#123456";
    document.head.appendChild(meta);
    settings("DARK");
    run();
    expect(themeColors()).toEqual([PAGE_COLOR.DARK]);
    expect(meta.content).toBe(PAGE_COLOR.DARK);
  });
});
