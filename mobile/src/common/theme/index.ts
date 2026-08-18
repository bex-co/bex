import { Appearance, Platform } from "react-native";
import { createTheming } from "@callstack/react-theme-provider";
import type { ColorTheme, ThemeProps } from "@/types/theme-props";
import { resolveMonoFontFamily } from "./typography";

export const fonts = {
  mono: resolveMonoFontFamily(Platform.OS),
  monoMedium: resolveMonoFontFamily(Platform.OS, { weight: "medium" }),
} as const;

export {
  fontSizes,
  fontWeights,
  headerActionStyle,
  maxFontSizeMultipliers,
  monoMinFontSize,
} from "./typography";
export { withAlpha } from "./color-utils";
export {
  space,
  gutter,
  rowMinHeight,
  rowPaddingVertical,
  sectionHeaderPaddingVertical,
} from "./spacing";

export const getSystemColorScheme = (): "light" | "dark" =>
  Appearance.getColorScheme() === "dark" ? "dark" : "light";

const shared = {
  primary: "#388a36",
  primaryLight: "#57a957",
  primaryDark: "#246523",
  secondary: "#596579",
  success: "#27833f",
  warning: "#b26a12",
  information: "#397bc0",
  error: "#c63f36",
};

const lightTheme: ColorTheme = {
  ...shared,
  isDark: false,
  background: "#f8faf8",
  foreground: "#202420",
  card: "#ffffff",
  border: "#d8ddd8",
  mutedForeground: "#677067",
  primaryMuted: "#e1f0e1",
  overlay: "rgba(0,0,0,0.5)",
  white: "#ffffff",
  black: "#202420",
  black90: "#303630",
  black80: "#677067",
  black60: "#929b92",
  black40: "#c3c9c3",
  black20: "#e3e7e3",
  black10: "#f0f3f0",
  text01: "#303630",
  nav01: "#202420",
  nav02: "#246523",
  tabIconDefault: "#929b92",
  tabIconSelected: shared.primary,
  activeTintColor: shared.primary,
  inactiveTintColor: "#677067",
  activeBackgroundColor: "#ffffff",
  inactiveBackgroundColor: "#ffffff",
  navBg: "#ffffff",
  navText: "#202420",
};

const darkTheme: ColorTheme = {
  ...shared,
  primary: "#74c875",
  primaryLight: "#93da94",
  primaryDark: "#57a957",
  success: "#62c87b",
  warning: "#e0a14a",
  information: "#71ace5",
  error: "#ec7168",
  isDark: true,
  background: "#111411",
  foreground: "#f1f4f1",
  card: "#191d19",
  border: "#343a34",
  mutedForeground: "#a3aca3",
  primaryMuted: "#223922",
  overlay: "rgba(0,0,0,0.65)",
  white: "#191d19",
  black: "#f1f4f1",
  black90: "#e3e8e3",
  black80: "#a3aca3",
  black60: "#808980",
  black40: "#596159",
  black20: "#343a34",
  black10: "#242924",
  text01: "#e3e8e3",
  nav01: "#0b0e0b",
  nav02: "#0b0e0b",
  tabIconDefault: "#808980",
  tabIconSelected: "#74c875",
  activeTintColor: "#74c875",
  inactiveTintColor: "#a3aca3",
  activeBackgroundColor: "#191d19",
  inactiveBackgroundColor: "#191d19",
  navBg: "#191d19",
  navText: "#f1f4f1",
};

export const themes: Record<"light" | "dark", ThemeProps> = {
  light: { name: "light", colorTheme: lightTheme },
  dark: { name: "dark", colorTheme: darkTheme },
};

const theming = createTheming(themes[getSystemColorScheme()]);
export const ThemeProvider = theming.ThemeProvider;
export const withTheme = theming.withTheme;
export const useTheme = theming.useTheme;
