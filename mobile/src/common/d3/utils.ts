import type { ChartDatum } from "./bar-chart-d3";

export function generateTicks(
  minimum: number,
  maximum: number,
  count: number,
): number[] {
  if (count <= 0) return [];
  if (count === 1) return [minimum];
  const step = (maximum - minimum) / (count - 1);
  return Array.from({ length: count }, (_, index) => minimum + step * index);
}

export function chartDomain(
  values: readonly number[],
  includeZero = false,
): [number, number] {
  const finite = values.filter(Number.isFinite);
  let minimum = finite.length ? Math.min(...finite) : 0;
  let maximum = finite.length ? Math.max(...finite) : 1;
  if (includeZero) {
    minimum = Math.min(0, minimum);
    maximum = Math.max(0, maximum);
  }
  if (minimum === maximum) {
    const padding = Math.abs(minimum) * 0.1 || 1;
    minimum -= padding;
    maximum += padding;
  }
  return [minimum, maximum];
}

export function zipSeries(
  labels: readonly string[],
  values: readonly number[],
): ChartDatum[] {
  return labels
    .slice(0, values.length)
    .map((label, index) => ({ label, value: values[index] }))
    .filter((datum) => Number.isFinite(datum.value));
}

export function nearestIndex(
  x: number,
  width: number,
  count: number,
  padding = 0,
): number {
  if (count <= 1 || width <= padding * 2) return 0;
  const clamped = Math.max(padding, Math.min(width - padding, x));
  return Math.max(
    0,
    Math.min(
      count - 1,
      Math.round(((clamped - padding) / (width - padding * 2)) * (count - 1)),
    ),
  );
}
