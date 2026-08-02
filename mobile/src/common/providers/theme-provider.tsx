import { useEffect, useState, type ReactNode } from "react";
import { Appearance } from "react-native";
import {
  getSystemColorScheme,
  themes,
  ThemeProvider as BaseThemeProvider,
} from "@/common/theme";

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [scheme, setScheme] = useState(getSystemColorScheme());
  useEffect(() => {
    const subscription = Appearance.addChangeListener(({ colorScheme }) => {
      setScheme(colorScheme === "dark" ? "dark" : "light");
    });
    return () => subscription.remove();
  }, []);
  return (
    <BaseThemeProvider theme={themes[scheme]}>{children}</BaseThemeProvider>
  );
}
