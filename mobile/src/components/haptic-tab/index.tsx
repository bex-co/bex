// As of SDK 56, expo-router vendors react-navigation; app code must import
// from expo-router/react-navigation instead of @react-navigation/*.
import { PlatformPressable } from "expo-router/react-navigation";
import * as Haptics from "expo-haptics";
// BottomTabBarButtonProps is not re-exported from expo-router/react-navigation;
// type-only import from the vendored module — erased at compile time.
import type { BottomTabBarButtonProps } from "expo-router/build/react-navigation/bottom-tabs";

export function HapticTab(props: BottomTabBarButtonProps) {
  return (
    <PlatformPressable
      {...props}
      onPressIn={(ev) => {
        if (process.env.EXPO_OS === "ios") {
          // Add a soft haptic feedback when pressing down on the tabs.
          Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
        }
        props.onPressIn?.(ev);
      }}
    />
  );
}
