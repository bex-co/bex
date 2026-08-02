import { useMemo } from "react";
import { StyleSheet } from "react-native";
import { useTheme } from "@/common/theme";
import type { ColorTheme } from "@/types/theme-props";

export function useThemeStyle<T extends StyleSheet.NamedStyles<T>>(
  factory: (theme: ColorTheme) => T,
): T {
  const theme = useTheme().colorTheme;
  return useMemo(() => StyleSheet.create(factory(theme)), [factory, theme]);
}
