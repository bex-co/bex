import { useContext } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { memoryStorage } from "@/test/mocks/storage";
import { PAGE_COLOR } from "@/test/theme-tokens";
import { ThemeProvider } from "../index";
import { ThemeProviderContext } from "../context";
import { SETTINGS_STORAGE_KEY, type ThemeSetting } from "../settings";
import { THEME_COLORS } from "../theme-color";
import type { ThemeProviderContextType } from "../type";

// <meta name="theme-color"> carries the applied theme's page colour (w3/098),
// as Render's provider sets it from the current theme's `page-primary`: the
// colour a phone's browser bar and an installed app's title bar take.

let theme: ThemeProviderContextType;
function Capture() {
  theme = useContext(ThemeProviderContext);
  return null;
}

const themeColors = () =>
  [
    ...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]'),
  ].map((meta) => meta.content);
const removeThemeColors = () =>
  document
    .querySelectorAll('meta[name="theme-color"]')
    .forEach((meta) => meta.remove());

function renderWith(themeSetting: ThemeSetting) {
  localStorage.setItem(
    SETTINGS_STORAGE_KEY,
    JSON.stringify({ themeSetting, logThemeSetting: "MATCH_APP" }),
  );
  return render(
    <ThemeProvider>
      <Capture />
    </ThemeProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
  document.documentElement.classList.remove("light", "dark", "high-contrast");
  removeThemeColors();
});
afterEach(() => {
  vi.unstubAllGlobals();
  document.documentElement.classList.remove("light", "dark", "high-contrast");
  removeThemeColors();
});

describe("the theme colour (w3/098)", () => {
  it("is each theme's --background, High Contrast Mode leaving it as it is", () => {
    expect(PAGE_COLOR).toEqual({
      LIGHT: THEME_COLORS.light,
      DARK: THEME_COLORS.dark,
      LIGHT_HIGH_CONTRAST: THEME_COLORS.light,
      DARK_HIGH_CONTRAST: THEME_COLORS.dark,
    });
  });

  it.each(Object.entries(PAGE_COLOR) as [ThemeSetting, string][])(
    "gives %s's page colour, %s, to one <meta name=theme-color>",
    (setting, color) => {
      renderWith(setting);
      expect(themeColors()).toEqual([color]);
    },
  );

  it("follows a change of theme and of High Contrast Mode", () => {
    renderWith("LIGHT");
    act(() => void theme.setTheme("dark"));
    expect(themeColors()).toEqual([PAGE_COLOR.DARK]);
    act(() => void theme.setHighContrast(true));
    expect(themeColors()).toEqual([PAGE_COLOR.DARK_HIGH_CONTRAST]);
    act(() => void theme.setTheme("light"));
    expect(themeColors()).toEqual([PAGE_COLOR.LIGHT_HIGH_CONTRAST]);
  });

  it("shows a previewed theme's colour, and puts the chosen one's back", () => {
    renderWith("LIGHT");
    act(() => theme.previewTheme("dark"));
    expect(themeColors()).toEqual([PAGE_COLOR.DARK]);
    act(() => theme.previewTheme(null));
    expect(themeColors()).toEqual([PAGE_COLOR.LIGHT]);
  });

  it("recolours the tag the pre-paint script added rather than adding one", () => {
    const meta = document.createElement("meta");
    meta.name = "theme-color";
    meta.content = PAGE_COLOR.LIGHT;
    document.head.appendChild(meta);
    renderWith("DARK");
    expect(themeColors()).toEqual([PAGE_COLOR.DARK]);
    expect(meta.content).toBe(PAGE_COLOR.DARK);
  });
});
