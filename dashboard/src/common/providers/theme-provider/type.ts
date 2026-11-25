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
  /** Dashboard Theme: System, Light or Dark, whatever High Contrast Mode;
   *  the chosen one, never a preview. */
  theme: Theme;
  /** Picks a Dashboard Theme and keeps High Contrast Mode as it is. Each
   *  setter resolves to whether the account kept the change. */
  setTheme: (theme: Theme) => Promise<boolean>;
  /** Shows a Dashboard Theme on the page without choosing it, as
   *  Appearance's row does while it is edited (w3/097, Render's
   *  `previewThemeSetting`); null puts the chosen theme back. */
  previewTheme: (theme: Theme | null) => void;
  /** High Contrast Mode: the theme is its `_HIGH_CONTRAST` twin. */
  highContrast: boolean;
  setHighContrast: (on: boolean) => Promise<boolean>;
  /** Log Explorer Theme. */
  logTheme: LogThemeSetting;
  setLogTheme: (logTheme: LogThemeSetting) => Promise<boolean>;
  /** Whether the stored settings have been read (after hydration). */
  ready: boolean;
  /** The signed-in shell binds the account (null on sign-out). */
  bindAccount: (binding: AccountThemeBinding | null) => void;
};
