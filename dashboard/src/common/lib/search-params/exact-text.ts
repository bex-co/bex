import { defaultParseSearch } from "@tanstack/react-router";

// Free-text search keys. Their value is what the user typed, never a number.
const TEXT_KEYS = ["text", "q"];

/**
 * The router's search parser, except that an unquoted numeric value of a
 * free-text key keeps the digits the link carried.
 *
 * The default parser JSON-parses every value, so `?text=9007199254740993`
 * arrives as the number 9007199254740992 and a validator that stringifies it
 * searches for a different identifier (w4/289). Only a single, finite-number
 * value is restored from the raw string; quoted strings, repeated keys and
 * every other key parse exactly as before.
 */
export function parseSearchWithExactText(searchStr: string) {
  const parsed: Record<string, unknown> = defaultParseSearch(searchStr);
  let raw: URLSearchParams | undefined;
  for (const key of TEXT_KEYS) {
    const value = parsed[key];
    if (typeof value !== "number" || !Number.isFinite(value)) continue;
    raw ??= new URLSearchParams(searchStr);
    const written = raw.getAll(key);
    if (written.length === 1) parsed[key] = written[0];
  }
  return parsed;
}
