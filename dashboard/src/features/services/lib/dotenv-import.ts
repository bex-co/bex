const VALID_ENV_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/;

export const MAX_DOTENV_FILE_BYTES = 1024 * 1024;

export interface DotenvEntry {
  key: string;
  value: string;
  line: number;
}

/** Apply parsed assignments in order, updating an existing key or appending it. */
export function upsertDotenvEntries<Row>(
  source: readonly Row[],
  entries: readonly DotenvEntry[],
  keyOf: (row: Row) => string | null,
  update: (row: Row, entry: DotenvEntry) => Row,
  create: (entry: DotenvEntry) => Row,
): Row[] {
  const rows = [...source];
  for (const entry of entries) {
    const index = rows.findIndex((row) => keyOf(row)?.trim() === entry.key);
    if (index >= 0) rows[index] = update(rows[index], entry);
    else rows.push(create(entry));
  }
  return rows;
}

export class DotenvParseError extends Error {
  constructor(
    public readonly line: number,
    public readonly reason: "assignment" | "key" | "quote" | "trailing",
  ) {
    super(`Invalid dotenv syntax on line ${line}`);
    this.name = "DotenvParseError";
  }
}

/**
 * Parse dotenv text as data only. There is no shell expansion, interpolation,
 * command substitution, or execution. Duplicate keys are deterministic: the
 * last assignment wins and carries its source line in the returned entry.
 *
 * ## Escape contract (the inverse of `formatEnvExport`)
 *
 * Inside **double quotes** this parser accepts the full JSON string escape set —
 * `\"`, `\\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t` and `\uXXXX` (surrogate pairs
 * included) — which is exactly what `env-export.ts` emits via `JSON.stringify`.
 * That makes `parseDotenv(formatEnvExport(x))` equal `x` for any string, the
 * property `__tests__/env-round-trip.test.ts` pins. Any other escape keeps the
 * previous behavior and yields the escaped character itself (`\q` → `q`).
 *
 * This is a deliberate superset of common dotenv parsers: a literal `\uXXXX`
 * a user typed into their own `.env` **is** expanded here, inside double
 * quotes only. Single-quoted and unquoted values are taken verbatim, which is
 * the escape hatch for anyone who means the backslash literally.
 */
export function parseDotenv(text: string): DotenvEntry[] {
  const entries = new Map<string, DotenvEntry>();
  const lines = text
    .replaceAll("\r\n", "\n")
    .replaceAll("\r", "\n")
    .split("\n");

  lines.forEach((source, index) => {
    const line = index + 1;
    let input = source.trim();
    if (!input || input.startsWith("#")) return;
    if (input.startsWith("export ") || input.startsWith("export\t")) {
      input = input.slice(6).trimStart();
    }
    const equals = input.indexOf("=");
    if (equals < 1) throw new DotenvParseError(line, "assignment");
    const key = input.slice(0, equals).trim();
    if (!VALID_ENV_KEY.test(key)) throw new DotenvParseError(line, "key");
    const value = parseValue(input.slice(equals + 1), line);
    entries.set(key, { key, value, line });
  });

  return [...entries.values()];
}

function parseValue(source: string, line: number): string {
  const input = source.trim();
  if (!input) return "";
  const quote = input[0];
  if (quote !== '"' && quote !== "'") return stripInlineComment(input);

  let value = "";
  let close = -1;
  let index = 1;
  while (index < input.length) {
    const character = input[index];
    if (quote === '"' && character === "\\") {
      const escape = decodeEscape(input, index);
      if (!escape) throw new DotenvParseError(line, "quote");
      value += escape.text;
      index = escape.next;
      continue;
    }
    if (character === quote) {
      close = index;
      break;
    }
    value += character;
    index += 1;
  }
  if (close < 0) throw new DotenvParseError(line, "quote");
  const trailing = input.slice(close + 1).trim();
  if (trailing && !trailing.startsWith("#")) {
    throw new DotenvParseError(line, "trailing");
  }
  return value;
}

/** The JSON single-character escapes, plus `"` `\` `/` which stand for themselves. */
const SHORT_ESCAPES: Readonly<Record<string, string>> = {
  b: "\b",
  f: "\f",
  n: "\n",
  r: "\r",
  t: "\t",
};

const HEX_QUAD = /^[0-9a-fA-F]{4}$/;

/**
 * Decode the escape sequence starting at the backslash `input[start]`.
 * Returns the decoded text and the index just past the sequence, or `null` when
 * the backslash is the last character (an unterminated escape).
 *
 * `\uXXXX` decodes one UTF-16 code unit, so a surrogate pair written as two
 * consecutive escapes reassembles into its astral character, and a lone
 * surrogate survives as itself — which is what `JSON.stringify` emits for one.
 * A malformed `\u` (fewer than four hex digits) is left as the literal `u`,
 * preserving the pre-existing "unknown escape yields its character" behavior.
 */
function decodeEscape(
  input: string,
  start: number,
): { text: string; next: number } | null {
  const marker = input[start + 1];
  if (marker === undefined) return null;
  const short = SHORT_ESCAPES[marker];
  if (short !== undefined) return { text: short, next: start + 2 };
  if (marker === "u") {
    const hex = input.slice(start + 2, start + 6);
    if (HEX_QUAD.test(hex)) {
      return {
        text: String.fromCharCode(Number.parseInt(hex, 16)),
        next: start + 6,
      };
    }
  }
  return { text: marker, next: start + 2 };
}

function stripInlineComment(value: string): string {
  const comment = value.search(/\s+#/);
  return (comment < 0 ? value : value.slice(0, comment)).trimEnd();
}
