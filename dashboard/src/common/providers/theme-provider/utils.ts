export const THEME_STORAGE_KEY = "vite-ui-theme";

export const getSystemTheme = () => {
  // During SSR, default to light theme
  if (typeof window === "undefined") {
    return "light";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
};
