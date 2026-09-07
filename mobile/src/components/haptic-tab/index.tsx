// As of SDK 56, expo-router vendors react-navigation; app code must import
// from expo-router/react-navigation instead of @react-navigation/*.
import { PlatformPressable } from "expo-router/react-navigation";
import * as Haptics from "expo-haptics";
import { useEffect, useRef } from "react";
import { Animated, Platform, StyleSheet } from "react-native";
import { useTheme } from "@/common/theme";
import { useReducedMotion } from "@/common/hooks/use-reduced-motion";
// BottomTabBarButtonProps is not re-exported from expo-router/react-navigation;
// type-only import from the vendored module — erased at compile time.
import type { BottomTabBarButtonProps } from "expo-router/build/react-navigation/bottom-tabs";

const styles = StyleSheet.create({
  button: {
    borderRadius: 26,
    borderCurve: "continuous",
    justifyContent: "center",
    overflow: "hidden",
  },
  content: { flex: 1, alignItems: "center", justifyContent: "center" },
});

export function HapticTab({
  children,
  style,
  ...props
}: BottomTabBarButtonProps) {
  const theme = useTheme().colorTheme;
  const reducedMotion = useReducedMotion();
  const selected =
    (props["aria-selected"] ?? props.accessibilityState?.selected) === true;
  const selection = useRef(new Animated.Value(selected ? 1 : 0)).current;
  const press = useRef(new Animated.Value(1)).current;

  useEffect(() => {
    const toValue = selected ? 1 : 0;
    if (reducedMotion) {
      selection.stopAnimation();
      selection.setValue(toValue);
      return;
    }
    const animation = Animated.spring(selection, {
      toValue,
      damping: 18,
      stiffness: 220,
      mass: 0.8,
      useNativeDriver: Platform.OS !== "web",
    });
    animation.start();
    return () => animation.stop();
  }, [selected, reducedMotion, selection]);

  const animatePress = (pressed: boolean) => {
    Animated.spring(press, {
      toValue: pressed && !reducedMotion ? 0.96 : 1,
      damping: 20,
      stiffness: 400,
      useNativeDriver: Platform.OS !== "web",
    }).start();
  };

  return (
    <PlatformPressable
      {...props}
      style={[style, styles.button]}
      onPressIn={(ev) => {
        if (Platform.OS !== "web") {
          void Haptics.selectionAsync().catch(() => undefined);
        }
        animatePress(true);
        props.onPressIn?.(ev);
      }}
      onPressOut={(ev) => {
        animatePress(false);
        props.onPressOut?.(ev);
      }}
    >
      <Animated.View
        pointerEvents="none"
        style={[
          StyleSheet.absoluteFill,
          {
            backgroundColor: theme.isDark ? theme.black10 : theme.black20,
            opacity: selection,
            transform: [
              {
                scale: selection.interpolate({
                  inputRange: [0, 1],
                  outputRange: [0.92, 1],
                }),
              },
            ],
          },
        ]}
      />
      <Animated.View
        style={[styles.content, { transform: [{ scale: press }] }]}
      >
        {children}
      </Animated.View>
    </PlatformPressable>
  );
}
