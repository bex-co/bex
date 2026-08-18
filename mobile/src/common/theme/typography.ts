import type { TextStyle } from "react-native";
import type { ColorTheme } from "@/types/theme-props";

export const JETBRAINS_MONO_REGULAR = "JetBrainsMono-Regular";
export const JETBRAINS_MONO_MEDIUM = "JetBrainsMono-Medium";

export const resolveMonoFontFamily = (
  os: string,
  opts: { embedded?: boolean; weight?: "regular" | "medium" } = {},
): string => {
  const { embedded = true, weight = "regular" } = opts;
  if (embedded) {
    return weight === "medium" ? JETBRAINS_MONO_MEDIUM : JETBRAINS_MONO_REGULAR;
  }
  return os === "ios" ? "Menlo" : "monospace";
};

export const fontSizes = {
  xs: 12,
  sm: 13,
  md: 14,
  lg: 16,
  xl: 18,
  xxl: 24,
  display: 28,
  heroSm: 36,
  hero: 56,
} as const;

export const monoMinFontSize = 13;
export const maxFontSizeMultipliers = {
  control: 1.5,
  content: 2,
  heading: 2,
} as const;
export const fontWeights = {
  regular: "400",
  medium: "500",
} as const satisfies Record<string, TextStyle["fontWeight"]>;

export const headerActionStyle = (theme: ColorTheme): TextStyle => ({
  fontSize: fontSizes.lg,
  fontWeight: fontWeights.medium,
  color: theme.primary,
});
