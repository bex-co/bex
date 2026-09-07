import { useIsFocused } from "expo-router";
import { useRef, type ReactElement } from "react";
import { View } from "react-native";
import { useTheme } from "@/common/theme";

/** Defer each native tab's queries until first focus, then retain its UI state. */
export function LazyTabScreen({ children }: { children: ReactElement }) {
  const focused = useIsFocused();
  const visited = useRef(focused);
  const theme = useTheme().colorTheme;
  if (focused) visited.current = true;

  return visited.current ? (
    children
  ) : (
    <View style={{ flex: 1, backgroundColor: theme.background }} />
  );
}
