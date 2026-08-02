import { useCallback, useMemo, useRef, useState } from "react";
import {
  PanResponder,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
  type GestureResponderEvent,
} from "react-native";
import Svg, {
  Circle,
  Defs,
  LinearGradient,
  Path,
  Stop,
} from "react-native-svg";
import { scaleLinear } from "d3-scale";
import { area as d3Area, curveMonotoneX, line as d3Line } from "d3-shape";
import * as Haptics from "expo-haptics";
import { useTheme } from "@/common/theme";
import { shortNumber } from "@/common/number-utils";
import type { ChartDatum, ValueFormatter } from "./bar-chart-d3";
import { chartDomain, nearestIndex, zipSeries } from "./utils";

type Props = {
  data: readonly ChartDatum[];
  label?: string;
  height?: number;
  width?: number;
  formatValue?: ValueFormatter;
  placeholder?: string;
  accessibilityLabel: string;
  onScrubStart?: () => void;
  onScrubEnd?: () => void;
};

export function InteractiveLineChart({
  data: input,
  label,
  height = 190,
  width,
  formatValue = shortNumber,
  placeholder = "Not enough data",
  accessibilityLabel,
  onScrubStart,
  onScrubEnd,
}: Props) {
  const data = useMemo(
    () =>
      zipSeries(
        input.map((datum) => datum.label),
        input.map((datum) => datum.value),
      ),
    [input],
  );
  const theme = useTheme().colorTheme;
  const window = useWindowDimensions();
  const chartWidth = width ?? Math.max(240, window.width - 32);
  const [active, setActive] = useState<number | null>(null);
  const last = useRef<number | null>(null);
  const hasSeries = data.length > 1;
  const [minimum, maximum] = chartDomain(data.map((datum) => datum.value));
  const x = (index: number) =>
    12 + (index / Math.max(1, data.length - 1)) * (chartWidth - 24);
  const y = scaleLinear()
    .domain([minimum, maximum])
    .range([height - 18, 14]);
  const linePath =
    d3Line<ChartDatum>()
      .x((_, index) => x(index))
      .y((datum) => y(datum.value))
      .curve(curveMonotoneX)(data) ?? "";
  const areaPath =
    d3Area<ChartDatum>()
      .x((_, index) => x(index))
      .y0(height - 18)
      .y1((datum) => y(datum.value))
      .curve(curveMonotoneX)(data) ?? "";
  const update = useCallback(
    (event: GestureResponderEvent) => {
      const index = nearestIndex(
        event.nativeEvent.locationX,
        chartWidth,
        data.length,
        12,
      );
      if (index !== last.current) {
        last.current = index;
        setActive(index);
        Haptics.selectionAsync().catch(() => undefined);
      }
    },
    [chartWidth, data.length],
  );
  const responder = useMemo(
    () =>
      PanResponder.create({
        onStartShouldSetPanResponder: () => hasSeries,
        onMoveShouldSetPanResponder: () => hasSeries,
        onPanResponderTerminationRequest: () => false,
        onPanResponderGrant: (event) => {
          onScrubStart?.();
          update(event);
        },
        onPanResponderMove: update,
        onPanResponderRelease: () => {
          last.current = null;
          setActive(null);
          onScrubEnd?.();
        },
        onPanResponderTerminate: () => {
          last.current = null;
          setActive(null);
          onScrubEnd?.();
        },
      }),
    [hasSeries, onScrubEnd, onScrubStart, update],
  );
  const shown = data[active ?? data.length - 1];
  return (
    <View
      accessible
      accessibilityRole="image"
      accessibilityLabel={accessibilityLabel}
    >
      {label ? (
        <Text style={[styles.label, { color: theme.mutedForeground }]}>
          {label}
        </Text>
      ) : null}
      <Text style={[styles.value, { color: theme.foreground }]}>
        {shown ? formatValue(shown.value) : "—"}
      </Text>
      <Text style={[styles.caption, { color: theme.mutedForeground }]}>
        {shown?.label ?? placeholder}
      </Text>
      <View {...responder.panHandlers}>
        <Svg width={chartWidth} height={height}>
          <Defs>
            <LinearGradient id="chart-fill" x1="0" y1="0" x2="0" y2="1">
              <Stop offset="0" stopColor={theme.primary} stopOpacity={0.28} />
              <Stop offset="1" stopColor={theme.primary} stopOpacity={0} />
            </LinearGradient>
          </Defs>
          {hasSeries ? (
            <>
              <Path d={areaPath} fill="url(#chart-fill)" />
              <Path
                d={linePath}
                fill="none"
                stroke={theme.primary}
                strokeWidth={3}
              />
              {active !== null && shown ? (
                <Circle
                  cx={x(active)}
                  cy={y(shown.value)}
                  r={6}
                  fill={theme.card}
                  stroke={theme.primary}
                  strokeWidth={3}
                />
              ) : null}
            </>
          ) : null}
        </Svg>
        {!hasSeries ? (
          <View pointerEvents="none" style={styles.placeholder}>
            <Text style={{ color: theme.mutedForeground }}>{placeholder}</Text>
          </View>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  label: { fontSize: 14 },
  value: { fontSize: 28, fontWeight: "600", marginTop: 2 },
  caption: { fontSize: 13, marginBottom: 4 },
  placeholder: {
    ...StyleSheet.absoluteFill,
    alignItems: "center",
    justifyContent: "center",
  },
});
