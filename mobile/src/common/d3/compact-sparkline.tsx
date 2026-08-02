import { useMemo } from "react";
import { StyleSheet, View, useWindowDimensions } from "react-native";
import Svg, { Circle, Path } from "react-native-svg";
import { scaleLinear } from "d3-scale";
import { curveLinear, line as d3Line } from "d3-shape";
import { useTheme } from "@/common/theme";
import { chartDomain } from "./utils";

type SparklinePoint = { timestamp: string; value: number };

export function CompactSparkline({
  points,
  accessibilityLabel,
  width,
  height = 54,
}: {
  points: readonly SparklinePoint[];
  accessibilityLabel: string;
  width?: number;
  height?: number;
}) {
  const theme = useTheme().colorTheme;
  const window = useWindowDimensions();
  const chartWidth = width ?? Math.max(120, Math.min(240, window.width * 0.42));
  const safe = useMemo(
    () =>
      points
        .filter(
          (point) =>
            Number.isFinite(Date.parse(point.timestamp)) &&
            Number.isFinite(point.value),
        )
        .sort(
          (left, right) =>
            Date.parse(left.timestamp) - Date.parse(right.timestamp),
        ),
    [points],
  );
  const [minimum, maximum] = chartDomain(safe.map((point) => point.value));
  const firstTime = safe[0] ? Date.parse(safe[0].timestamp) : 0;
  const lastTime = safe.at(-1) ? Date.parse(safe.at(-1)!.timestamp) : 1;
  const x = scaleLinear()
    .domain(
      firstTime === lastTime
        ? [firstTime - 1, lastTime + 1]
        : [firstTime, lastTime],
    )
    .range([4, chartWidth - 4]);
  const y = scaleLinear()
    .domain([minimum, maximum])
    .range([height - 5, 5]);
  const path =
    safe.length > 1
      ? d3Line<SparklinePoint>()
          .x((point) => x(Date.parse(point.timestamp)))
          .y((point) => y(point.value))
          .curve(curveLinear)(safe)
      : null;
  const single = safe.length === 1 ? safe[0] : null;

  return (
    <View
      accessible
      accessibilityRole="image"
      accessibilityLabel={accessibilityLabel}
      style={styles.container}
    >
      <Svg width={chartWidth} height={height}>
        {path ? (
          <Path d={path} fill="none" stroke={theme.primary} strokeWidth={2.5} />
        ) : null}
        {single ? (
          <Circle
            cx={chartWidth / 2}
            cy={y(single.value)}
            r={4}
            fill={theme.card}
            stroke={theme.primary}
            strokeWidth={2.5}
          />
        ) : null}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({ container: { flexShrink: 0 } });
