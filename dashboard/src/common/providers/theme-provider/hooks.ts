import { useContext } from "react";
import { ThemeProviderContext } from "./context.ts";
import { logThemeClass } from "./settings.ts";
import { getSystemTheme } from "./utils.ts";

export const useTheme = () => {
  const context = useContext(ThemeProviderContext);

  if (context === undefined)
    throw new Error("useTheme must be used within a ThemeProvider");

  return context;
};

export const useIsDarkTheme = () => {
  const { theme } = useTheme();
  if (theme === "system") {
    return getSystemTheme() === "dark";
  }
  return theme === "dark";
};

/** The classes a log explorer's root carries for Log Explorer Theme, or
 *  undefined while it matches the dashboard (w3/m531). */
export const useLogThemeClass = () => {
  const { logTheme, highContrast } = useTheme();
  return logThemeClass(logTheme, highContrast);
};
