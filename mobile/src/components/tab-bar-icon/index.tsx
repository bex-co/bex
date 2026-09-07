import { useEffect, useRef } from "react";
import { Animated, Platform, type ColorValue } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useReducedMotion } from "@/common/hooks/use-reduced-motion";
import { tabIcons, type TabRouteName } from "./tab-icons";

export function TabBarIcon({
  route,
  color,
  focused,
}: {
  route: TabRouteName;
  color: ColorValue;
  focused: boolean;
}) {
  const scale = useRef(new Animated.Value(focused ? 1.1 : 1)).current;
  const reducedMotion = useReducedMotion();

  useEffect(() => {
    const toValue = focused ? 1.1 : 1;
    if (reducedMotion) {
      scale.stopAnimation();
      scale.setValue(toValue);
      return;
    }
    const animation = Animated.spring(scale, {
      toValue,
      damping: 18,
      stiffness: 220,
      mass: 0.8,
      useNativeDriver: Platform.OS !== "web",
    });
    animation.start();
    return () => animation.stop();
  }, [focused, reducedMotion, scale]);

  return (
    <Animated.View style={{ transform: [{ scale }] }}>
      <Ionicons
        name={tabIcons[route][focused ? "active" : "inactive"]}
        size={28}
        color={color}
      />
    </Animated.View>
  );
}
