export const shortNumber = (value: number | string): string => {
  const number = typeof value === "string" ? Number.parseFloat(value) : value;
  if (!Number.isFinite(number)) return String(value);
  const magnitude = Math.abs(number);
  const suffixes = [
    [1e12, "T"],
    [1e9, "B"],
    [1e6, "M"],
    [1e3, "K"],
  ] as const;
  for (const [threshold, suffix] of suffixes) {
    if (magnitude >= threshold) {
      const compact = number / threshold;
      return `${Number.isInteger(compact) ? compact : compact.toFixed(1)}${suffix}`;
    }
  }
  return Number.isInteger(number) ? String(number) : number.toFixed(1);
};

export const formatPercent = (value: number): string =>
  Number.isFinite(value) ? `${value.toFixed(1)}%` : "—";

export const formatBytes = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes === 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const index = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  const value = bytes / 1024 ** index;
  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
};
