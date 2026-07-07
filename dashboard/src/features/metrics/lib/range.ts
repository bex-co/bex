export interface RangePreset {
  id: "30m" | "1h" | "12h";
  spanSeconds: number;
  resolutionSeconds: number;
}

// Resolution scales with span so a request-metric series stays readable (not
// thousands of points) while still resolving fine-grained enough to show shape.
// Display labels live in metrics-filters.tsx's RANGE_LABEL_KEYS (i18n).
export const RANGE_PRESETS: RangePreset[] = [
  { id: "30m", spanSeconds: 30 * 60, resolutionSeconds: 15 },
  { id: "1h", spanSeconds: 60 * 60, resolutionSeconds: 30 },
  { id: "12h", spanSeconds: 12 * 60 * 60, resolutionSeconds: 300 },
];

export const DEFAULT_RANGE_PRESET = RANGE_PRESETS[1]; // "1h", matches bex-api's own default span

/** Resolves a preset into the startTime/endTime/resolutionSeconds query args. */
export function resolveRange(preset: RangePreset, now: Date) {
  const end = now.toISOString();
  const start = new Date(
    now.getTime() - preset.spanSeconds * 1000,
  ).toISOString();
  return {
    startTime: start,
    endTime: end,
    resolutionSeconds: preset.resolutionSeconds,
  };
}
