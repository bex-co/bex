import { setCookie } from "@/common/hooks/use-cookie-storage-state/cookie";
import type { Theme } from "./type.ts";
import { THEME_STORAGE_KEY } from "./utils.ts";

/** Dashboard Theme as the controls offer it: System, Light, Dark. */
export const THEMES = ["system", "light", "dark"] as const satisfies Theme[];

export const isTheme = (value: unknown): value is Theme =>
  (THEMES as readonly unknown[]).includes(value);

/**
 * Render's ThemeSetting and LogThemeSetting (w3/m531,
 * docs/render-artifacts/account-appearance.md), the values bex-api keeps on
 * the account. High Contrast Mode is not a setting of its own: it is the
 * `_HIGH_CONTRAST` twin of System, Light or Dark.
 */
export const THEME_SETTINGS = [
  "SYSTEM",
  "LIGHT",
  "DARK",
  "SYSTEM_HIGH_CONTRAST",
  "LIGHT_HIGH_CONTRAST",
  "DARK_HIGH_CONTRAST",
] as const;
export type ThemeSetting = (typeof THEME_SETTINGS)[number];

export const LOG_THEME_SETTINGS = [
  "MATCH_APP",
  "ALWAYS_LIGHT",
  "ALWAYS_DARK",
] as const;
export type LogThemeSetting = (typeof LOG_THEME_SETTINGS)[number];

export interface ThemeSettings {
  themeSetting: ThemeSetting;
  logThemeSetting: LogThemeSetting;
}

export const DEFAULT_THEME_SETTINGS: ThemeSettings = {
  themeSetting: "SYSTEM",
  logThemeSetting: "MATCH_APP",
};

export const isThemeSetting = (value: unknown): value is ThemeSetting =>
  (THEME_SETTINGS as readonly unknown[]).includes(value);

export const isLogThemeSetting = (value: unknown): value is LogThemeSetting =>
  (LOG_THEME_SETTINGS as readonly unknown[]).includes(value);

const HIGH_CONTRAST_SUFFIX = "_HIGH_CONTRAST";

/** Whether a setting is a high-contrast twin (Render's
 *  `highContrastThemeEnabled`). */
export const isHighContrast = (setting: ThemeSetting) =>
  setting.endsWith(HIGH_CONTRAST_SUFFIX);

/** The System, Light or Dark a setting is, twin or not. */
export const baseTheme = (setting: ThemeSetting): Theme =>
  setting.replace(HIGH_CONTRAST_SUFFIX, "").toLowerCase() as Theme;

/** The setting for a base theme with High Contrast Mode on or off: Render's
 *  `currentHighContrastThemeSetting` and `currentNonHighContrastThemeSetting`. */
export const toThemeSetting = (
  theme: Theme,
  highContrast: boolean,
): ThemeSetting =>
  `${theme.toUpperCase()}${highContrast ? HIGH_CONTRAST_SUFFIX : ""}` as ThemeSetting;

/** The class that marks a high-contrast subtree, beside `light` or `dark`. */
export const HIGH_CONTRAST_CLASS = "high-contrast";

/**
 * The classes a log explorer's root carries (Render's
 * `effectiveLogViewerTheme`): none for Match Dashboard, so the explorer
 * follows the page; otherwise the forced theme, with the high-contrast class
 * while High Contrast Mode is on.
 */
export function logThemeClass(
  logThemeSetting: LogThemeSetting,
  highContrast: boolean,
): string | undefined {
  if (logThemeSetting === "MATCH_APP") return undefined;
  const theme = logThemeSetting === "ALWAYS_DARK" ? "dark" : "light";
  return highContrast ? `${theme} ${HIGH_CONTRAST_CLASS}` : theme;
}

/**
 * The browser's copy of the account's settings, JSON `{themeSetting,
 * logThemeSetting}` as Render keeps `render-user-settings`. The pre-paint
 * script (`theme-script.js.txt`) reads it to apply the theme before the first
 * paint, and other tabs follow it through `storage` events.
 */
export const SETTINGS_STORAGE_KEY = "bex-user-settings";

/** A stored copy, or null when the value is not one. */
export function parseStoredSettings(raw: string | null): ThemeSettings | null {
  if (!raw) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (typeof value !== "object" || value === null) return null;
    const { themeSetting, logThemeSetting } = value as Record<string, unknown>;
    if (!isThemeSetting(themeSetting)) return null;
    return {
      themeSetting,
      logThemeSetting: isLogThemeSetting(logThemeSetting)
        ? logThemeSetting
        : "MATCH_APP",
    };
  } catch {
    return null;
  }
}

/**
 * The browser's copy: the settings JSON, else the theme this browser kept
 * before w3/m531 (`vite-ui-theme`), else the defaults.
 */
export function readBrowserSettings(): ThemeSettings {
  try {
    const stored = parseStoredSettings(
      localStorage.getItem(SETTINGS_STORAGE_KEY),
    );
    if (stored) return stored;
    const legacy = localStorage.getItem(THEME_STORAGE_KEY);
    if (isTheme(legacy)) {
      return {
        ...DEFAULT_THEME_SETTINGS,
        themeSetting: toThemeSetting(legacy, false),
      };
    }
  } catch {
    // No storage (a private window): the defaults.
  }
  return DEFAULT_THEME_SETTINGS;
}

/** Keeps the browser's copy in step with what is applied. */
export function writeBrowserSettings(settings: ThemeSettings) {
  const theme = baseTheme(settings.themeSetting);
  try {
    localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings));
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // No storage: the theme still applies for this page.
  }
  setCookie(THEME_STORAGE_KEY, theme, {
    expires: 365,
    sameSite: "lax",
    path: "/",
  });
}
