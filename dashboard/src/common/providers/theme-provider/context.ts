import { createContext } from "react";
import type { ThemeProviderContextType } from "./type.ts";

const notSaved = () => Promise.resolve(false);

/** The context outside a provider: the defaults, and setters that do nothing
 *  and save nothing. */
export const initialThemeContext: ThemeProviderContextType = {
  theme: "system",
  setTheme: notSaved,
  previewTheme: () => null,
  highContrast: false,
  setHighContrast: notSaved,
  logTheme: "MATCH_APP",
  setLogTheme: notSaved,
  ready: false,
  bindAccount: () => null,
};

export const ThemeProviderContext =
  createContext<ThemeProviderContextType>(initialThemeContext);
