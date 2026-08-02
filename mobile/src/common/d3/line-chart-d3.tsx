import { useMemo } from "react";
import { View, useWindowDimensions } from "react-native";
import Svg, { Circle, G, Line, Path, Text as SvgText } from "react-native-svg";
import { scaleLinear, scalePoint } from "d3-scale";
import { curveMonotoneX, line as d3Line } from "d3-shape";
import { ErrorBoundary } from "react-error-boundary";
import { useTheme } from "@/common/theme";
import { shortNumber } from "@/common/number-utils";
import { chartDomain, zipSeries } from "./utils";
import type { ChartDatum, ValueFormatter } from "./bar-chart-d3";

type Props = {
  data: readonly ChartDatum[];
  height?: number;
  width?: number;
  formatValue?: ValueFormatter;
  accessibilityLabel: string;
};

function LineChart({
  data,
  height = 220,
  width,
  formatValue = shortNumber,
  accessibilityLabel,
}: Props) {
  const theme = useTheme().colorTheme;
  const window = useWindowDimensions();
  const chartWidth = width ?? Math.max(240, window.width - 32);
  const left = 52;
  const top = 16;
  const bottom = 32;
  const [minimum, maximum] = chartDomain(data.map((datum) => datum.value));
  const x = useMemo(
    () =>
      scalePoint<string>()
        .domain(data.map((datum) => datum.label))
        .range([left, chartWidth - 12])
        .padding(0.4),
    [chartWidth, data],
  );
  const y = useMemo(
    () =>
      scaleLinear()
        .domain([minimum, maximum])
        .range([height - bottom, top])
        .nice(),
    [bottom, height, maximum, minimum],
  );
  const path = d3Line<ChartDatum>()
    .x((datum) => x(datum.label) ?? left)
    .y((datum) => y(datum.value))
    .curve(curveMonotoneX)(data);
  return (
    <View
      accessible
      accessibilityRole="image"
      accessibilityLabel={accessibilityLabel}
    >
      <Svg width={chartWidth} height={height}>
        {y.ticks(4).map((tick) => (
          <G key={tick}>
            <Line
              x1={left}
              x2={chartWidth - 12}
              y1={y(tick)}
              y2={y(tick)}
              stroke={theme.border}
              strokeDasharray="4 3"
            />
            <SvgText
              x={left - 6}
              y={y(tick) + 4}
              fill={theme.mutedForeground}
              fontSize={11}
              textAnchor="end"
            >
              {formatValue(tick)}
            </SvgText>
          </G>
        ))}
        {path ? (
          <Path d={path} fill="none" stroke={theme.primary} strokeWidth={3} />
        ) : null}
        {data.map((datum) => (
          <Circle
            key={datum.label}
            cx={x(datum.label) ?? left}
            cy={y(datum.value)}
            r={4}
            fill={theme.card}
            stroke={theme.primary}
            strokeWidth={2}
          />
        ))}
        {data.map((datum) => (
          <SvgText
            key={`label-${datum.label}`}
            x={x(datum.label) ?? left}
            y={height - 9}
            fill={theme.mutedForeground}
            fontSize={11}
            textAnchor="middle"
          >
            {datum.label}
          </SvgText>
        ))}
      </Svg>
    </View>
  );
}

export function LineChartD3(props: Props) {
  const safe = zipSeries(
    props.data.map((datum) => datum.label),
    props.data.map((datum) => datum.value),
  );
  return (
    <ErrorBoundary fallback={null}>
      <LineChart {...props} data={safe} />
    </ErrorBoundary>
  );
}
