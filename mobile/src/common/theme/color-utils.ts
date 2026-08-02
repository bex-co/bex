/**
 * Color helpers for composing theme tokens.
 *
 * This module is pure (no runtime react-native import) so unit tests can load
 * it under Node, same as `./typography.ts`.
 */

/**
 * Apply an alpha channel to a hex color, for tinted fills built from a token.
 *
 * Every `ColorTheme` value is a 6-digit hex, which React Native accepts with an
 * `#rrggbbaa` suffix. Anything else (already-aliased colors, `rgba(...)`
 * strings) is passed through untouched rather than corrupted.
 *
 * @param hex - A `#rrggbb` color, typically a theme token.
 * @param alpha - Opacity in the 0–1 range; clamped.
 * @returns `#rrggbbaa`, or `hex` unchanged when it is not 6-digit hex.
 */
export const withAlpha = (hex: string, alpha: number): string => {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return hex;
  const clamped = Math.max(0, Math.min(1, alpha));
  const suffix = Math.round(clamped * 255)
    .toString(16)
    .padStart(2, "0");
  return `${hex}${suffix}`;
};
