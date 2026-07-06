/**
 * Format a number, optionally with thousands separators.
 *
 * @param value       - The numeric value to format
 * @param renderCommas - When true, adds thousands separators (e.g. 1,234,567.89).
 *                       When false, omits them (e.g. 1234567.89).
 * @returns Formatted number string
 */
export const formatNumber = (value: number, renderCommas: boolean): string => {
  if (renderCommas) {
    return value.toLocaleString();
  }
  if (Number.isInteger(value)) {
    return String(value);
  }
  // Preserve up to 2 decimal places, strip trailing zeros
  return value.toFixed(2).replace(/\.?0+$/, "");
};
