/** The last point's value of the first series — null when there is none. */
export function latestValue(
  series: { points: { value: number }[] }[],
): number | null {
  const points = series[0]?.points ?? [];
  return points.length > 0 ? points[points.length - 1].value : null;
}
