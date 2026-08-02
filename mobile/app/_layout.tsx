import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { Providers } from "@/common/providers/providers";
import { useTheme } from "@/common/theme";

function RootStack() {
  const theme = useTheme().colorTheme;
  return (
    <>
      <StatusBar style={theme.isDark ? "light" : "dark"} />
      <Stack screenOptions={{ headerShown: false }} />
    </>
  );
}

export default function RootLayout() {
  return (
    <Providers>
      <RootStack />
    </Providers>
  );
}
