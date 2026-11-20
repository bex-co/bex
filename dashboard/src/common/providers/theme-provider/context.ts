import { createContext } from "react";
import type { ThemeProviderContextType } from "./type.ts";

/** The context outside a provider: the defaults, and setters that do nothing. */
export const initialThemeContext: ThemeProviderContextType = {
  theme: "system",
  setTheme: () => null,
  highContrast: false,
  setHighContrast: () => null,
  logTheme: "MATCH_APP",
  setLogTheme: () => null,
  ready: false,
  bindAccount: () => null,
};

export const ThemeProviderContext =
  createContext<ThemeProviderContextType>(initialThemeContext);
