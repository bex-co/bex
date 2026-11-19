/**
 * Stands in for one placeholder's value while translating, so splitting the
 * result on it recovers the translated words around the value whatever the
 * language's word order — and the caller can style the value between them.
 * Never collides with real phrase text.
 */
export const PLACEHOLDER_SENTINEL = "\u0000";

/** A translation made with `PLACEHOLDER_SENTINEL` as one value, split into
 *  the text before and after it. */
export function aroundPlaceholder(translated: string): [string, string] {
  const [before = "", after = ""] = translated.split(PLACEHOLDER_SENTINEL);
  return [before, after];
}
