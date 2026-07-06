import { useContext } from "react";
import { ThemeProviderContext } from "./context.ts";
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
