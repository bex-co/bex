import { useEffect } from "react";
import { StyleSheet, ViewStyle } from "react-native";
import { useTheme } from "@/common/theme";
import Animated, {
  useSharedValue,
  useAnimatedStyle,
  withRepeat,
  withTiming,
  withSequence,
} from "react-native-reanimated";

const styles = StyleSheet.create({
  light: {
    backgroundColor: "rgba(0,0,0,0.1)",
  },
  dark: {
    backgroundColor: "rgba(255, 255, 255, 0.1)",
  },
  loadingTile: {
    overflow: "hidden",
    borderRadius: 3,
  },
});

type LoadingTileProps = {
  mx?: number;
  height?: number;
  width?: number;
  style?: ViewStyle;
};

export const LoadingTile = (props: LoadingTileProps) => {
  const { style, mx, height, width } = props;
  // Resolved theme name from the provider — themeVar itself can hold
  // "system", which must not be compared against "dark" directly.
  const theme = useTheme().name;

  const dynamicStyles: ViewStyle = {
    ...(mx && { marginHorizontal: mx }),
    ...(height && { height }),
    ...(width && { width }),
  };

  const opacity = useSharedValue(1);

  useEffect(() => {
    opacity.value = withRepeat(
      withSequence(
        withTiming(0.5, { duration: 300 }),
        withTiming(1, { duration: 300 }),
      ),
      -1,
      true,
    );
  }, [opacity]);

  const animatedStyle = useAnimatedStyle(() => {
    return {
      opacity: opacity.value,
    };
  });

  return (
    <Animated.View
      style={[
        styles.loadingTile,
        dynamicStyles,
        theme === "dark" ? styles.dark : styles.light,
        style,
        animatedStyle,
      ]}
    />
  );
};
