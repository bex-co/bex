import { useMemo } from "react";
import { View, useWindowDimensions } from "react-native";
import Svg, { G, Line, Rect, Text as SvgText } from "react-native-svg";
import { scaleBand, scaleLinear } from "d3-scale";
import { ErrorBoundary } from "react-error-boundary";
import { useTheme } from "@/common/theme";
import { shortNumber } from "@/common/number-utils";
import { chartDomain, zipSeries } from "./utils";

export type ChartDatum = { label: string; value: number };
export type ValueFormatter = (value: number) => string;

type Props = {
  data: readonly ChartDatum[];
  height?: number;
  width?: number;
  formatValue?: ValueFormatter;
  accessibilityLabel: string;
};

function BarChart({
  data,
  height = 220,
  width,
  formatValue = shortNumber,
  accessibilityLabel,
}: Props) {
  const theme = useTheme().colorTheme;
  const window = useWindowDimensions();
  const chartWidth = width ?? Math.max(240, window.width - 32);
  const values = data.map((datum) => datum.value);
  const [minimum, maximum] = chartDomain(values, true);
  const left = 52;
  const top = 16;
  const bottom = 32;
  const x = useMemo(
    () =>
      scaleBand<string>()
        .domain(data.map((datum) => datum.label))
        .range([left, chartWidth - 8])
        .padding(0.24),
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
  const zero = y(0);
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
              x2={chartWidth - 8}
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
        {data.map((datum) => {
          const valueY = y(datum.value);
          const topY = datum.value >= 0 ? valueY : zero;
          return (
            <Rect
              key={datum.label}
              x={x(datum.label) ?? left}
              y={topY}
              width={x.bandwidth()}
              height={Math.max(2, Math.abs(zero - valueY))}
              rx={3}
              fill={datum.value >= 0 ? theme.primary : theme.error}
            />
          );
        })}
        {data.map((datum) => (
          <SvgText
            key={`label-${datum.label}`}
            x={(x(datum.label) ?? left) + x.bandwidth() / 2}
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

export function BarChartD3(props: Props) {
  const safe = zipSeries(
    props.data.map((datum) => datum.label),
    props.data.map((datum) => datum.value),
  );
  return (
    <ErrorBoundary fallback={null}>
      <BarChart {...props} data={safe} />
    </ErrorBoundary>
  );
}
