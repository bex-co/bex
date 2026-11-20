import { createElement, type ReactNode } from "react";
import {
  initialThemeContext,
  ThemeProviderContext,
} from "@/common/providers/theme-provider/context";
import type { LogThemeSetting } from "@/common/providers/theme-provider/settings";

/**
 * Log Explorer Theme for a test (w3/m531): the theme context with a chosen
 * Log Explorer Theme and High Contrast Mode, around an explorer.
 */
export function LogThemeContext({
  logTheme,
  highContrast,
  children,
}: {
  logTheme: LogThemeSetting;
  highContrast: boolean;
  children: ReactNode;
}) {
  return createElement(
    ThemeProviderContext.Provider,
    {
      value: { ...initialThemeContext, logTheme, highContrast, ready: true },
    },
    children,
  );
}

/** Each Log Explorer Theme with High Contrast Mode off and on, and the
 *  theme classes it puts on an explorer's root. */
export const LOG_THEME_CASES: [LogThemeSetting, boolean, string[]][] = [
  ["MATCH_APP", false, []],
  ["MATCH_APP", true, []],
  ["ALWAYS_DARK", false, ["dark"]],
  ["ALWAYS_LIGHT", false, ["light"]],
  ["ALWAYS_DARK", true, ["dark", "high-contrast"]],
  ["ALWAYS_LIGHT", true, ["light", "high-contrast"]],
];

/** The theme classes an explorer's root carries. */
export function explorerThemeClasses(container: HTMLElement): string[] {
  const root = container.querySelector<HTMLElement>("[data-log-explorer]");
  if (!root) throw new Error("no log explorer root rendered");
  return ["light", "dark", "high-contrast"].filter((name) =>
    root.classList.contains(name),
  );
}
