import type { LogThemeSetting, ThemeSettings } from "./settings.ts";

export type Theme = "dark" | "light" | "system";

/**
 * The signed-in account the theme belongs to (w3/m531): its stored settings,
 * and how to write a change to it. `write` resolves to the values the server
 * kept, or throws when the change was not saved.
 */
export interface AccountThemeBinding {
  settings: ThemeSettings;
  write: (patch: Partial<ThemeSettings>) => Promise<Partial<ThemeSettings>>;
}

export type ThemeProviderContextType = {
  /** Dashboard Theme: System, Light or Dark, whatever High Contrast Mode. */
  theme: Theme;
  /** Picks a Dashboard Theme and keeps High Contrast Mode as it is. */
  setTheme: (theme: Theme) => void;
  /** High Contrast Mode: the theme is its `_HIGH_CONTRAST` twin. */
  highContrast: boolean;
  setHighContrast: (on: boolean) => void;
  /** Log Explorer Theme. */
  logTheme: LogThemeSetting;
  setLogTheme: (logTheme: LogThemeSetting) => void;
  /** Whether the stored settings have been read (after hydration). */
  ready: boolean;
  /** The signed-in shell binds the account (null on sign-out). */
  bindAccount: (binding: AccountThemeBinding | null) => void;
};
