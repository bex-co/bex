/**
 * Spacing scale — the single source of truth for layout gaps, paddings and
 * margins.
 *
 * Spacing does not change between light and dark, so it lives here as plain
 * constants rather than on the theme object (the old per-theme `sizing` array
 * was never actually read). Every layout number in the app should resolve to
 * one of these tokens; the hard-coded pixel values that used to live per-file
 * drifted apart and made related screens feel inconsistent.
 *
 * 4pt-based, with 2 kept as a hairline half-step. Use the semantic aliases
 * (`gutter`, `rowPaddingVertical`, …) when one exists; reach for `space.*`
 * for one-off gaps.
 */
export const space = {
  xxs: 2,
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  xxl: 32,
} as const;

/**
 * Canonical horizontal inset for screen content, list rows and headers. Every
 * screen's left/right gutter should be this, so content lines up edge-to-edge
 * as you move between tabs.
 */
export const gutter = space.lg; // 16

/**
 * Minimum height for a primary list/table row. Pins single-line rows to a
 * comfortable tap target so density reads the same across screens; multi-line
 * rows grow past it. `minHeight` (not a fixed `height`) so rows still scale
 * with Dynamic Type instead of clipping large text.
 */
export const rowMinHeight = 44;

/** Vertical padding for a primary list/table row. */
export const rowPaddingVertical = space.md; // 12

/** Vertical padding for a section / column header sitting between rows. */
export const sectionHeaderPaddingVertical = space.sm; // 8
