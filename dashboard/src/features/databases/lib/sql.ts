const READ_ONLY_KEYWORDS = new Set([
  "select",
  "show",
  "explain",
  "values",
  "table",
]);

/**
 * Best-effort client classification for the confirmation dialog. Unknown
 * statements are treated as writable; the backend's read-only transaction is
 * still the authority whenever this returns true.
 */
export function isReadOnlySQL(sql: string): boolean {
  let remaining = sql.trimStart();
  while (remaining) {
    if (remaining.startsWith("--")) {
      const newline = remaining.indexOf("\n");
      if (newline < 0) return true;
      remaining = remaining.slice(newline + 1).trimStart();
      continue;
    }
    if (remaining.startsWith("/*")) {
      const end = remaining.indexOf("*/", 2);
      if (end < 0) return true;
      remaining = remaining.slice(end + 2).trimStart();
      continue;
    }
    break;
  }
  const keyword = remaining.match(/^([a-z]+)/i)?.[1]?.toLowerCase();
  return keyword ? READ_ONLY_KEYWORDS.has(keyword) : true;
}
