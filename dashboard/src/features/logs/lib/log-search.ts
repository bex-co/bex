import {
  DEFAULT_RANGE_PRESET,
  parseRangePreset,
  type RangePreset,
  type RangePresetID,
} from "@/features/metrics/lib/range";

export interface LogSearch {
  range?: RangePresetID;
}

export function parseLogSearch(search: Record<string, unknown>): LogSearch {
  const range = parseRangePreset(search.range);
  return range ? { range: range.id } : {};
}

/** Restores the URL range, falling back to the documented one-hour default. */
export function logRangeFromSearch(search: LogSearch): RangePreset {
  return parseRangePreset(search.range) ?? DEFAULT_RANGE_PRESET;
}
