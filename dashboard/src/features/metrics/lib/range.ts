export interface RangePreset {
  id: "30m" | "1h" | "3h" | "6h" | "12h" | "1d";
  spanSeconds: number;
  resolutionSeconds: number;
}

// Resolution scales with span so a series stays readable (~120 points, not
// thousands) while still resolving fine-grained enough to show shape.
// Display labels live in metrics-filters.tsx's RANGE_LABEL_KEYS (i18n).
// Ranges beyond Prometheus's retention (deploy/gitops/base/prometheus.yaml)
// simply show the retained window — same as Render past its own horizon.
export const RANGE_PRESETS: RangePreset[] = [
  { id: "30m", spanSeconds: 30 * 60, resolutionSeconds: 15 },
  { id: "1h", spanSeconds: 60 * 60, resolutionSeconds: 30 },
  { id: "3h", spanSeconds: 3 * 60 * 60, resolutionSeconds: 90 },
  { id: "6h", spanSeconds: 6 * 60 * 60, resolutionSeconds: 180 },
  { id: "12h", spanSeconds: 12 * 60 * 60, resolutionSeconds: 300 },
  { id: "1d", spanSeconds: 24 * 60 * 60, resolutionSeconds: 720 },
];

export const DEFAULT_RANGE_PRESET = RANGE_PRESETS[1]; // "1h", matches bex-api's own default span

export type RangePresetID = RangePreset["id"];

/** Returns the supported relative range named by a URL value, or null. */
export function parseRangePreset(value: unknown): RangePreset | null {
  if (typeof value !== "string") return null;
  return RANGE_PRESETS.find((preset) => preset.id === value) ?? null;
}

// A relative window is just a span + a bucket size — the preset `id` is a
// picker/URL concern, so consumers with a fixed window (e.g. the Scaling
// page's 48h Recent Metrics, w7/m43) can pass a bare window without minting a
// fake preset id.
export type RangeWindow = Pick<RangePreset, "spanSeconds" | "resolutionSeconds">;

/** Resolves a window into the startTime/endTime/resolutionSeconds query args. */
export function resolveRange(preset: RangeWindow, now: Date) {
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
