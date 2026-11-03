/** Binary byte units, scaled to the largest unit below 1024. */
export function formatBytes(value: number, digits = 0): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = value;
  let i = 0;
  while (Math.abs(v) >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  // Whole bytes stay whole at the base precisions (digits ≤ 1); an axis
  // asking for more (formatAxisTickLabels) gets decimals even in B (w4/381).
  return `${v.toFixed(i === 0 && digits <= 1 ? 0 : digits)} ${units[i]}`;
}
