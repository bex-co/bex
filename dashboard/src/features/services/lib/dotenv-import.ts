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

  for (let index = 0; index < lines.length; index += 1) {
    const line = index + 1;
    let input = lines[index].trim();
    if (!input || input.startsWith("#")) continue;
    if (input.startsWith("export ") || input.startsWith("export\t")) {
      input = input.slice(6).trimStart();
    }
    const equals = input.indexOf("=");
    if (equals < 1) throw new DotenvParseError(line, "assignment");
    const key = input.slice(0, equals).trim();
    if (!VALID_ENV_KEY.test(key)) throw new DotenvParseError(line, "key");
    // A quoted value may span physical lines until its closing quote, with
    // the line breaks kept, as dotenv parses a PEM key or certificate (w4/159).
    // An unquoted value still ends at its line.
    let raw = input.slice(equals + 1);
    const quote = raw.trimStart()[0];
    let parsed = parseValue(raw, line);
    while (parsed === UNTERMINATED && index + 1 < lines.length) {
      index += 1;
      raw += "\n" + lines[index];
      // Only a line holding the quote character can close the value, so
      // re-scan then rather than once per line of a long certificate.
      if (lines[index].includes(quote)) parsed = parseValue(raw, line);
    }
    if (parsed === UNTERMINATED) throw new DotenvParseError(line, "quote");
    entries.set(key, { key, value: parsed, line });
  }

  return [...entries.values()];
}

// parseValue's answer for a quoted value whose closing quote has not been
// reached yet: the caller appends the next physical line and tries again.
const UNTERMINATED = Symbol("unterminated");

function parseValue(
  source: string,
  line: number,
): string | typeof UNTERMINATED {
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
      if (!escape) return UNTERMINATED;
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
  if (close < 0) return UNTERMINATED;
  const trailing = input.slice(close + 1).trim();
  if (trailing && !trailing.startsWith("#")) {
    // Name the physical line the closing quote is on, which a multi-line
    // value puts below the assignment's own line.
    const closingLine = line + (input.slice(0, close).split("\n").length - 1);
    throw new DotenvParseError(closingLine, "trailing");
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
