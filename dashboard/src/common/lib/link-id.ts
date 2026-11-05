/** Rewrite a saved dashboard link's historical durable ID (`<legacyPrefix>-`
 * plus a 20-character xid) to the public spelling the API now returns, keeping
 * the suffix. Only that exact shape is rewritten; names and unknown formats
 * remain untouched, so they never acquire ID semantics. Prefixes are trusted
 * lowercase literals, never user input. */
export function canonicalLinkId(
  value: string,
  legacyPrefix: string,
  publicPrefix: string,
): string {
  return new RegExp(`^${legacyPrefix}-[0-9a-v]{20}$`).test(value)
    ? `${publicPrefix}-${value.slice(legacyPrefix.length + 1)}`
    : value;
}
