export interface RangePreset {
  id: "30m" | "1h" | "4h" | "12h" | "24h" | "2d" | "7d" | "14d";
  spanSeconds: number;
  resolutionSeconds: number;
}

// Render's preset ladder, captured live 2026-07-17 (w5/m42): 30 min / hour /
// 4 hours / 12 hours / 24 hours / 2 days / 7 days / 14 days. Render's
// plan-gated "Last 30 days" and "Custom" are deliberately not offered.
// Resolution scales with span so a series stays readable (~120 points, not
// thousands) while still resolving fine-grained enough to show shape.
// Display labels live in range-select.tsx's RANGE_LABEL_KEYS (i18n).
// Ranges beyond Prometheus's retention (deploy/gitops/base/prometheus.yaml)
// simply show the retained window — same as Render past its own horizon.
export const RANGE_PRESETS: RangePreset[] = [
  { id: "30m", spanSeconds: 30 * 60, resolutionSeconds: 15 },
  { id: "1h", spanSeconds: 60 * 60, resolutionSeconds: 30 },
  { id: "4h", spanSeconds: 4 * 60 * 60, resolutionSeconds: 120 },
  { id: "12h", spanSeconds: 12 * 60 * 60, resolutionSeconds: 300 },
  { id: "24h", spanSeconds: 24 * 60 * 60, resolutionSeconds: 720 },
  { id: "2d", spanSeconds: 2 * 24 * 60 * 60, resolutionSeconds: 1440 },
  { id: "7d", spanSeconds: 7 * 24 * 60 * 60, resolutionSeconds: 5040 },
  { id: "14d", spanSeconds: 14 * 24 * 60 * 60, resolutionSeconds: 10080 },
];

export const DEFAULT_RANGE_PRESET = RANGE_PRESETS[3]; // "12h" — Render's own default

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
export type RangeWindow = Pick<
  RangePreset,
  "spanSeconds" | "resolutionSeconds"
>;

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
