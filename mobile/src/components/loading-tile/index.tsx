import { useEffect, useRef } from "react";
import { Animated, StyleSheet, ViewStyle } from "react-native";
import { useTheme } from "@/common/theme";

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

  const opacity = useRef(new Animated.Value(1)).current;

  useEffect(() => {
    const animation = Animated.loop(
      Animated.sequence([
        Animated.timing(opacity, {
          toValue: 0.5,
          duration: 300,
          useNativeDriver: true,
        }),
        Animated.timing(opacity, {
          toValue: 1,
          duration: 300,
          useNativeDriver: true,
        }),
      ]),
    );
    animation.start();
    return () => animation.stop();
  }, [opacity]);

  return (
    <Animated.View
      style={[
        styles.loadingTile,
        dynamicStyles,
        theme === "dark" ? styles.dark : styles.light,
        style,
        { opacity },
      ]}
    />
  );
};
