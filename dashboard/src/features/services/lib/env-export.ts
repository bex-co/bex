import { downloadTextFile } from "@/common/lib/download-file";

/**
 * Serializes a complete service environment as a deterministic dotenv file.
 * Keys sort by code point and every value is JSON-quoted, so whitespace,
 * newlines, quotes, and empty strings remain unambiguous. Callers must supply
 * freshly revealed values for every listed key; this formatter has no masked
 * placeholder or partial-export mode.
 *
 * ## Escape contract (the inverse of `parseDotenv`)
 *
 * `JSON.stringify` is the serializer, so a value leaves as a double-quoted JSON
 * string: `\"`, `\\`, `\b`, `\f`, `\n`, `\r`, `\t` and `\uXXXX` for every other
 * control character and for lone surrogates. `dotenv-import.ts` decodes exactly
 * that set, which makes Export and Import inverses —
 * `parseDotenv(formatEnvExport(x))` equals `x` for any string, pinned by
 * `__tests__/env-round-trip.test.ts`. Change one side and you must change the
 * other. (U+2028/U+2029 are emitted raw, which is safe here: they are not
 * `\n`, so the importer's line split leaves them inside their value.)
 */
export function formatEnvExport(
  entries: ReadonlyArray<{ key: string; value: string }>,
): string {
  return [...entries]
    .sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0))
    .map(({ key, value }) => `${key}=${JSON.stringify(value)}`)
    .join("\n")
    .concat(entries.length > 0 ? "\n" : "");
}

export function downloadEnvFile(filename: string, contents: string): void {
  downloadTextFile(filename, contents);
}
