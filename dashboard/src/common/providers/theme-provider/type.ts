export type Theme = "dark" | "light" | "system";

export type ThemeProviderContextType = {
  theme: Theme;
  setTheme: (theme: Theme) => void;
};
