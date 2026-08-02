export interface ThemeProps {
  name: "light" | "dark";
  colorTheme: ColorTheme;
}

export interface ColorTheme {
  isDark: boolean;
  background: string;
  foreground: string;
  card: string;
  border: string;
  mutedForeground: string;
  primaryMuted: string;
  overlay: string;
  primary: string;
  primaryLight: string;
  primaryDark: string;
  secondary: string;
  white: string;
  black: string;
  black90: string;
  black80: string;
  black60: string;
  black40: string;
  black20: string;
  black10: string;
  text01: string;
  error: string;
  success: string;
  warning: string;
  information: string;
  nav01: string;
  nav02: string;
  tabIconDefault: string;
  tabIconSelected: string;
  activeTintColor: string;
  inactiveTintColor: string;
  activeBackgroundColor: string;
  inactiveBackgroundColor: string;
  navBg: string;
  navText: string;
}
