import { useCallback, useEffect, useMemo, useState } from "react";
import { ThemeProviderContext } from "./context.ts";
import type { Theme } from "./type.ts";
import { getSystemTheme, THEME_STORAGE_KEY } from "./utils.ts";

type ThemeProviderProps = {
  children: React.ReactNode;
  defaultTheme?: Theme;
  storageKey?: string;
};

const isValidTheme = (value: string | null): value is Theme => {
  return value !== null && ["dark", "light", "system"].includes(value);
};

const getThemeFromQuery = (): Theme | null => {
  if (typeof window === "undefined") return null; // SSR guard
  const params = new URLSearchParams(window.location.search);
  const themeParam = params.get("theme");
  if (isValidTheme(themeParam)) {
    return themeParam;
  }
  return null;
};

const getThemeFromStorage = (storageKey: string): Theme | null => {
  if (typeof window === "undefined") return null; // SSR guard
  const storedTheme = localStorage.getItem(storageKey);
  if (isValidTheme(storedTheme)) {
    return storedTheme;
  }
  return null;
};

export function ThemeProvider({
  children,
  defaultTheme = "system",
  storageKey = THEME_STORAGE_KEY,
  ...props
}: ThemeProviderProps) {
  // Track hydration state to prevent hydration mismatch
  const [isHydrated, setIsHydrated] = useState(false);

  // Get initial theme - prefer SSR-injected theme over defaultTheme
  const getInitialTheme = (): Theme => {
    // In SSR, always use defaultTheme
    if (typeof window === "undefined") {
      return defaultTheme;
    }

    // In browser, check if SSR injected theme
    if (window.__THEME__) {
      return window.__THEME__; // SSR theme is "light" or "dark", safe to use as Theme
    }

    // Fallback to defaultTheme
    return defaultTheme;
  };

  // Always start with defaultTheme to match server-side rendering
  // This prevents hydration mismatch
  const [theme, setTheme] = useState<Theme>(getInitialTheme);

  // After first render (hydration complete), mark as hydrated
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional for hydration tracking
    setIsHydrated(true);
  }, []);

  // After hydration, load theme from URL query param or localStorage
  // Priority: URL query param > localStorage > defaultTheme
  useEffect(() => {
    if (!isHydrated) return; // Wait for hydration to complete

    const queryTheme = getThemeFromQuery();
    if (queryTheme) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional theme loading after hydration
      setTheme(queryTheme);
      return;
    }
    const storedTheme = getThemeFromStorage(storageKey);
    if (storedTheme) {
      setTheme(storedTheme);
    }
    // If neither exists, keep defaultTheme (already set)
  }, [storageKey, isHydrated]);

  // Apply theme to DOM by adding/removing class on <html> element
  // ONLY apply after hydration to prevent mismatch
  useEffect(() => {
    if (!isHydrated) return; // Wait for hydration to complete

    const root = window.document.documentElement;

    // Detect currently applied theme from DOM class
    const currentClass = root.classList.contains("dark")
      ? "dark"
      : root.classList.contains("light")
        ? "light"
        : null;

    // Resolve "system" theme to actual "light" or "dark" based on user's OS preference
    const targetTheme = theme === "system" ? getSystemTheme() : theme;

    // Only update DOM if theme actually changed (prevents unnecessary reflows)
    if (currentClass !== targetTheme) {
      root.classList.remove("light", "dark");
      root.classList.add(targetTheme);
    }
  }, [theme, isHydrated]);

  const updateTheme = useCallback(
    (theme: Theme) => {
      localStorage.setItem(storageKey, theme);

      // Sync theme to cookie for SSR theme detection
      // This ensures server can detect theme on next request
      // Cookie expires in 1 year (same as typical localStorage behavior)
      try {
        document.cookie = `vite-ui-theme=${theme}; path=/; max-age=31536000; SameSite=Lax`;
      } catch {
        // Ignore cookie errors (incognito mode, etc.)
      }

      setTheme(theme);
    },
    [storageKey],
  );

  const value = useMemo(
    () => ({ theme, setTheme: updateTheme }),
    [theme, updateTheme],
  );

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

/* eslint-disable-next-line react-refresh/only-export-components */
export { useTheme, useIsDarkTheme } from "./hooks.ts";
