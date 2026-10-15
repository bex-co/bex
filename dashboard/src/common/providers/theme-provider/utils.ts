export const THEME_STORAGE_KEY = "vite-ui-theme";

export const SYSTEM_DARK_QUERY = "(prefers-color-scheme: dark)";

export const getSystemTheme = () => {
  // During SSR, default to light theme
  if (typeof window === "undefined") {
    return "light";
  }
  return window.matchMedia(SYSTEM_DARK_QUERY).matches ? "dark" : "light";
};
