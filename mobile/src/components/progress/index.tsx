import { View as RNView } from "react-native";
import { useEffect } from "react";
import Animated, {
  useSharedValue,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";
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
  const percentValue = useSharedValue(percent);
  const theme = useTheme().colorTheme;
  const styles = useThemeStyle(() => ({
    container: {
      backgroundColor: theme.white,
      height: height,
      width: "100%",
    },
  }));
  const animatedStyle = useAnimatedStyle(() => ({
    width: `${Math.min(percentValue.value, 100)}%`,
    backgroundColor: theme.primary,
    height: height,
  }));

  useEffect(() => {
    percentValue.value = withTiming(percent, { duration });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [percent]);

  return (
    <RNView style={styles.container}>
      <Animated.View style={animatedStyle} />
    </RNView>
  );
};

export default Progress;
