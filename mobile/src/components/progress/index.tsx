import { Animated, View as RNView } from "react-native";
import { useEffect, useRef } from "react";
import { useTheme } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";

type ProgressProps = {
  percent: number;
  height?: number;
  duration?: number;
};

export const Progress = ({
  percent,
  height = 3,
  duration = 500,
}: ProgressProps) => {
  const percentValue = useRef(
    new Animated.Value(Math.min(Math.max(percent, 0), 100)),
  ).current;
  const theme = useTheme().colorTheme;
  const styles = useThemeStyle(() => ({
    container: {
      backgroundColor: theme.white,
      height: height,
      width: "100%",
    },
  }));
  const animatedStyle = {
    width: percentValue.interpolate({
      inputRange: [0, 100],
      outputRange: ["0%", "100%"],
      extrapolate: "clamp",
    }),
    backgroundColor: theme.primary,
    height: height,
  };

  useEffect(() => {
    Animated.timing(percentValue, {
      toValue: Math.min(Math.max(percent, 0), 100),
      duration,
      useNativeDriver: false,
    }).start();
  }, [duration, percent, percentValue]);

  return (
    <RNView style={styles.container}>
      <Animated.View style={animatedStyle} />
    </RNView>
  );
};

export default Progress;
