import { PixelRatio, Platform, StatusBar } from "react-native";

export const contentPadding = 16;
export const onePx = 1 / PixelRatio.get();
export const statusBarHeight =
  Platform.OS === "android" ? (StatusBar.currentHeight ?? 0) : 0;
