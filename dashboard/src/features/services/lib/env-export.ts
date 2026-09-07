import { downloadTextFile } from "@/common/lib/download-file";

/**
 * Serializes a complete service environment as a deterministic dotenv file.
 * Keys sort by code point and every value is JSON-quoted, so whitespace,
 * newlines, quotes, and empty strings remain unambiguous. Callers must supply
 * freshly revealed values for every listed key; this formatter has no masked
 * placeholder or partial-export mode.
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
